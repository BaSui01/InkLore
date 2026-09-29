package artifactstore

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// artifactRefSagaStore 是 Saga 所需的最小 ref 生命周期端口。
// Saga 只负责编排 payload 与 ref 的补偿关系，不应绑定具体 GORM 仓储实现。
type artifactRefSagaStore interface {
	Begin(context.Context, ArtifactRefCreate) (*ArtifactRef, bool, error)
	MarkReady(context.Context, int64, ArtifactLocator) error
	MarkStatus(context.Context, int64, string, string) error
	Get(context.Context, int64) (*ArtifactRef, error)
}

type Saga struct {
	refs  artifactRefSagaStore
	store Store
}

type WriteRequest struct {
	Ref      ArtifactRefCreate
	Document Document
}

type WriteResult struct {
	Ref    ArtifactRef
	Stored StoredDocument
	Reused bool
}

func NewSaga(refs artifactRefSagaStore, store Store) *Saga {
	return &Saga{refs: refs, store: store}
}

func (s *Saga) Write(ctx context.Context, req WriteRequest) (*WriteResult, error) {
	if s == nil || s.refs == nil || s.store == nil {
		return nil, fmt.Errorf("artifact saga is not configured")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ref, reused, err := s.refs.Begin(ctx, req.Ref)
	if err != nil {
		return nil, err
	}
	if reused && (ref.Status == StatusReady || ref.Status == StatusActiveLegacy) && ref.StorageObjectID != "" {
		return &WriteResult{Ref: *ref, Stored: storedDocumentFromRef(ref), Reused: true}, nil
	}
	req.Document.ID = fmt.Sprintf("artifact-ref-%d", ref.ID)
	req.Document.ArtifactRefID = ref.ID
	stored, err := s.store.Put(ctx, req.Document)
	if err != nil {
		markErr := s.refs.MarkStatus(context.WithoutCancel(ctx), ref.ID, StatusFailed, "payload_write_failed")
		return nil, errors.Join(fmt.Errorf("写入 artifact payload 失败: %w", err), markErr)
	}
	locator := ArtifactLocator{
		StorageObjectID: stored.ID,
		StorageBackend:  stored.Backend,
		ContentHash:     stored.ContentHash,
		SizeBytes:       stored.PayloadSizeBytes,
	}
	if err := s.refs.MarkReady(ctx, ref.ID, locator); err != nil {
		return nil, s.compensateFailedCommit(ref.ID, stored.ID, err)
	}
	ref.StorageObjectID = stored.ID
	ref.StorageBackend = stored.Backend
	ref.ContentHash = stored.ContentHash
	ref.SizeBytes = stored.PayloadSizeBytes
	ref.Status = StatusReady
	return &WriteResult{Ref: *ref, Stored: stored, Reused: reused}, nil
}

func (s *Saga) BeginExternal(ctx context.Context, ref ArtifactRefCreate) (*ArtifactRef, bool, error) {
	if s == nil || s.refs == nil {
		return nil, false, fmt.Errorf("artifact saga is not configured")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return s.refs.Begin(ctx, ref)
}

func (s *Saga) CompleteExternal(ctx context.Context, refID int64, locator ArtifactLocator) error {
	if s == nil || s.refs == nil {
		return fmt.Errorf("artifact saga is not configured")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return s.refs.MarkReady(ctx, refID, locator)
}

func (s *Saga) RegisterExternalReady(ctx context.Context, ref ArtifactRefCreate, locator ArtifactLocator) (*ArtifactRef, bool, error) {
	pending, reused, err := s.BeginExternal(ctx, ref)
	if err != nil {
		return nil, false, err
	}
	if reused && (pending.Status == StatusReady || pending.Status == StatusActiveLegacy) {
		return pending, true, nil
	}
	if err := s.CompleteExternal(ctx, pending.ID, locator); err != nil {
		return nil, reused, err
	}
	pending.Status = StatusReady
	pending.StorageObjectID = locator.StorageObjectID
	pending.StorageKey = locator.StorageKey
	pending.StorageBackend = locator.StorageBackend
	pending.ContentHash = locator.ContentHash
	pending.SizeBytes = locator.SizeBytes
	return pending, reused, nil
}

func (s *Saga) Compensate(ctx context.Context, refID int64, category string, cleanup func(context.Context) error) error {
	if s == nil || s.refs == nil || refID <= 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	var cleanupErr error
	if cleanup != nil {
		cleanupErr = cleanup(cleanupCtx)
	}
	if cleanupErr == nil && s.store != nil {
		if ref, err := s.refs.Get(cleanupCtx, refID); err != nil {
			cleanupErr = err
		} else if ref != nil && ref.StorageObjectID != "" {
			cleanupErr = s.store.Delete(cleanupCtx, ref.StorageObjectID)
			if errors.Is(cleanupErr, ErrNotFound) {
				cleanupErr = nil
			}
		}
	}
	status := StatusFailed
	if cleanupErr != nil {
		status = StatusCleanupPending
	}
	statusErr := s.refs.MarkStatus(cleanupCtx, refID, status, category)
	return errors.Join(cleanupErr, statusErr)
}

func (s *Saga) compensateFailedCommit(refID int64, StorageObjectID string, cause error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cleanupErr := s.store.Delete(ctx, StorageObjectID)
	if errors.Is(cleanupErr, ErrNotFound) {
		cleanupErr = nil
	}
	status := StatusFailed
	if cleanupErr != nil {
		status = StatusCleanupPending
	}
	markErr := s.refs.MarkStatus(ctx, refID, status, "ref_commit_failed")
	return errors.Join(fmt.Errorf("提交 artifact ref 失败: %w", cause), cleanupErr, markErr)
}

func storedDocumentFromRef(ref *ArtifactRef) StoredDocument {
	if ref == nil {
		return StoredDocument{}
	}
	createdAt := ref.CreatedAt
	if ref.ReadyAt != nil {
		createdAt = *ref.ReadyAt
	}
	return StoredDocument{ID: ref.StorageObjectID, Backend: ref.StorageBackend, ContentHash: ref.ContentHash, PayloadSizeBytes: ref.SizeBytes, CreatedAt: createdAt}
}
