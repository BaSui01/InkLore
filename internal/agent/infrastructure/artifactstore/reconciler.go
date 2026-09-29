package artifactstore

import (
	"context"
	"errors"
	"strings"
	"time"

	"InkLore/internal/platform/logger"
)

type ReconcileOptions struct {
	OlderThan time.Duration
	Limit     int
	Execute   bool
}

type ReconcileItem struct {
	RefID   int64
	Status  string
	Reason  string
	Action  string
	Changed bool
}

type ReconcileReport struct {
	Items      []ReconcileItem
	Scanned    int
	Candidates int
	Repaired   int
	Failed     int
	DryRun     bool
}

type LocatorCleaner interface {
	DeleteArtifact(context.Context, ArtifactLocator) error
}

type LocatorCleanupFunc func(context.Context, ArtifactLocator) error

func (f LocatorCleanupFunc) DeleteArtifact(ctx context.Context, locator ArtifactLocator) error {
	if f == nil {
		return errors.New("artifact locator cleaner is not configured")
	}
	return f(ctx, locator)
}

// artifactRefReconciliationStore 是对账器需要的最小 ref 端口。
// 对账器只读取候选并推进清理状态，不应依赖 ref 仓储的其他写入能力。
type artifactRefReconciliationStore interface {
	ListReconciliationCandidates(context.Context, time.Time, int) ([]ArtifactRef, error)
	MarkStatus(context.Context, int64, string, string) error
}

type Reconciler struct {
	refs           artifactRefReconciliationStore
	store          Store
	locatorCleaner LocatorCleaner
	now            func() time.Time
}

func NewReconciler(refs artifactRefReconciliationStore, store Store, cleaners ...LocatorCleaner) *Reconciler {
	reconciler := &Reconciler{refs: refs, store: store, now: time.Now}
	if len(cleaners) > 0 {
		reconciler.locatorCleaner = cleaners[0]
	}
	return reconciler
}

func (r *Reconciler) Run(ctx context.Context, opts ReconcileOptions) (ReconcileReport, error) {
	report := ReconcileReport{DryRun: !opts.Execute, Items: make([]ReconcileItem, 0)}
	if r == nil || r.refs == nil {
		return report, errors.New("artifact reconciler is not configured")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if opts.OlderThan <= 0 {
		opts.OlderThan = 30 * time.Minute
	}
	candidates, err := r.refs.ListReconciliationCandidates(ctx, r.now().Add(-opts.OlderThan), opts.Limit)
	if err != nil {
		return report, err
	}
	report.Scanned = len(candidates)
	report.Candidates = len(candidates)
	for i := range candidates {
		item := r.reconcileOne(ctx, &candidates[i], opts.Execute)
		report.Items = append(report.Items, item)
		if item.Changed {
			report.Repaired++
		}
		if item.Action == "failed" {
			report.Failed++
		}
		logger.InfoContext(ctx, "artifact reconciliation candidate", "artifact_ref_id", item.RefID, "status", item.Status, "reason", item.Reason, "action", item.Action, "dry_run", !opts.Execute)
	}
	return report, nil
}

// RunCandidates reconciles an owner-filtered, already proven candidate set.
// Owners perform reference checks; artifactstore still owns lifecycle mutation.
// Execute remains opt-in and false is the safe default.
func (r *Reconciler) RunCandidates(ctx context.Context, candidates []ArtifactRef, opts ReconcileOptions) (ReconcileReport, error) {
	report := ReconcileReport{DryRun: !opts.Execute, Items: make([]ReconcileItem, 0, len(candidates)), Scanned: len(candidates), Candidates: len(candidates)}
	if r == nil || r.refs == nil {
		return report, errors.New("artifact reconciler is not configured")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	for i := range candidates {
		if candidates[i].Status != StatusFailed && candidates[i].Status != StatusCleanupPending {
			report.Items = append(report.Items, ReconcileItem{RefID: candidates[i].ID, Status: candidates[i].Status, Reason: "status_not_cleanupable", Action: "skipped"})
			continue
		}
		item := r.reconcileOne(ctx, &candidates[i], opts.Execute)
		report.Items = append(report.Items, item)
		if item.Changed {
			report.Repaired++
		}
		if item.Action == "failed" {
			report.Failed++
		}
	}
	return report, nil
}
func (r *Reconciler) reconcileOne(ctx context.Context, ref *ArtifactRef, execute bool) ReconcileItem {
	item := ReconcileItem{RefID: ref.ID, Status: ref.Status, Reason: reconciliationReason(ref), Action: "would_cleanup"}
	if !execute {
		return item
	}
	if ref.Status == StatusReady && ref.StorageBackend == BackendPostgres && strings.TrimSpace(ref.StorageObjectID) == "" {
		if err := r.refs.MarkStatus(ctx, ref.ID, StatusFailed, "invalid_locator"); err != nil {
			item.Action = "failed"
			return item
		}
		item.Action = "marked_failed"
		item.Changed = true
		return item
	}
	if ref.StorageBackend == BackendPostgres && strings.TrimSpace(ref.StorageObjectID) != "" {
		if r.store == nil {
			item.Action = "failed"
			return item
		}
		if err := r.store.Delete(ctx, ref.StorageObjectID); err != nil && !errors.Is(err, ErrNotFound) {
			if ctx.Err() != nil {
				item.Action = "failed"
				return item
			}
			_ = r.refs.MarkStatus(context.WithoutCancel(ctx), ref.ID, StatusCleanupPending, "reconcile_delete_failed")
			item.Action = "failed"
			return item
		}
	}
	if ref.Status == StatusCleanupPending && ref.StorageBackend != BackendPostgres && strings.TrimSpace(ref.StorageKey) != "" {
		if r.locatorCleaner == nil {
			item.Action = "failed"
			return item
		}
		if err := r.locatorCleaner.DeleteArtifact(ctx, ArtifactLocator{
			StorageKey: ref.StorageKey, StorageBackend: ref.StorageBackend,
			ContentHash: ref.ContentHash, SizeBytes: ref.SizeBytes,
		}); err != nil && !errors.Is(err, ErrNotFound) {
			if ctx.Err() != nil {
				item.Action = "failed"
				return item
			}
			_ = r.refs.MarkStatus(context.WithoutCancel(ctx), ref.ID, StatusCleanupPending, "reconcile_delete_failed")
			item.Action = "failed"
			return item
		}
	}
	status := StatusFailed
	category := item.Reason
	if ref.Status == StatusCleanupPending || (ref.Status == StatusFailed && strings.TrimSpace(ref.StorageObjectID) != "") {
		status = StatusDeleted
		category = ""
	}
	if err := r.refs.MarkStatus(ctx, ref.ID, status, category); err != nil {
		item.Action = "failed"
		return item
	}
	item.Action = "cleaned"
	item.Changed = true
	return item
}

func reconciliationReason(ref *ArtifactRef) string {
	if ref == nil {
		return "invalid_ref"
	}
	if ref.Status == StatusReady && ref.StorageBackend == BackendPostgres && strings.TrimSpace(ref.StorageObjectID) == "" {
		return "invalid_locator"
	}
	switch ref.Status {
	case StatusPending:
		return "pending_timeout"
	case StatusCleanupPending:
		return "cleanup_pending"
	case StatusFailed:
		return "failed_with_locator"
	default:
		return "unknown"
	}
}
