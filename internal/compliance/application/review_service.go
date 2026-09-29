package application

import (
	d "InkLore/internal/compliance/domain"
	"InkLore/internal/platform/scope"
	"context"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

func (s *Service) CreateReview(ctx context.Context, p scope.Principal, reportID string, req d.ReviewCreate) (d.Review, error) {
	if !scope.ValidID(reportID) {
		return d.Review{}, bad("report_id")
	}
	if req.ExpectedRevision < 1 {
		return d.Review{}, bad("expected_revision")
	}
	if utf8.RuneCountInString(req.Note) > 1000 {
		return d.Review{}, bad("note")
	}

	var created d.Review
	err := s.opts.UoW.Within(ctx, func(txctx context.Context) error {
		report, err := s.opts.Repositories.Reports.GetReportForUpdate(txctx, p.TenantID, reportID)
		if err != nil {
			return err
		}
		if report.OwnerUserID != p.ActorID {
			return missing()
		}
		if report.State != d.ReportStateSucceeded {
			return invalidState()
		}
		if report.Revision != req.ExpectedRevision {
			return versionConflict(report.Revision)
		}

		// If report has any risk (open or ignored), acknowledged_risk must be true
		if report.RiskCount > 0 && !req.AcknowledgedRisk {
			return reviewRequired()
		}

		now := time.Now().UTC()
		report.Revision++
		report.UpdatedAt = now
		if err := s.opts.Repositories.Reports.UpdateReport(txctx, report); err != nil {
			return err
		}

		review := d.Review{
			ID:               uuid.NewString(),
			TenantID:         p.TenantID,
			CreatedAt:        now,
			UpdatedAt:        now,
			ReportID:         reportID,
			ReportRevision:   report.Revision,
			ChapterID:        report.ChapterID,
			ChapterRevision:  report.ChapterRevision,
			ContentHash:      report.ContentHash,
			RuleSetID:        report.RuleSetID,
			ActorUserID:      p.ActorID,
			AcknowledgedRisk: req.AcknowledgedRisk,
			Note:             req.Note,
		}

		if err := s.opts.Repositories.Reviews.CreateReview(txctx, review); err != nil {
			return err
		}

		created = review
		return nil
	})

	return created, err
}

func (s *Service) ListReviews(ctx context.Context, p scope.Principal, reportID string, query d.ListQuery) (d.Page[d.Review], error) {
	if !scope.ValidID(reportID) {
		return d.Page[d.Review]{}, bad("report_id")
	}

	report, err := s.opts.Repositories.Reports.GetReport(ctx, p.TenantID, reportID)
	if err != nil {
		return d.Page[d.Review]{}, err
	}
	if report.OwnerUserID != p.ActorID {
		return d.Page[d.Review]{}, missing()
	}

	if query.Limit <= 0 || query.Limit > 100 {
		query.Limit = 20
	}

	items, hasMore, err := s.opts.Repositories.Reviews.ListReviewsByReport(ctx, p.TenantID, reportID, query)
	if err != nil {
		return d.Page[d.Review]{}, err
	}

	return d.Page[d.Review]{
		Items:   items,
		HasMore: hasMore,
	}, nil
}

// ValidateReviewReceipts validates export compliance receipts for content domain export gating.
// It checks scope, report success, report revision matching, snapshot exact match, and rule set freshness.
func (s *Service) ValidateReviewReceipts(ctx context.Context, tenantID, actorID string, snapshots []ExportSnapshot, reviewIDs []string) (ValidatedReviews, error) {
	if len(reviewIDs) == 0 {
		return ValidatedReviews{}, nil
	}

	// 1. Get current published rule set for tenant
	publishedSet, err := s.opts.Repositories.RuleSets.GetCurrentPublishedRuleSet(ctx, tenantID)
	if err != nil {
		return ValidatedReviews{}, reviewStale()
	}

	// 2. Fetch reviews
	reviews, err := s.opts.Repositories.Reviews.GetReviewsByIDs(ctx, tenantID, reviewIDs)
	if err != nil || len(reviews) != len(reviewIDs) {
		return ValidatedReviews{}, reviewStale()
	}

	snapshotMap := make(map[string]ExportSnapshot, len(snapshots))
	for _, sn := range snapshots {
		snapshotMap[sn.ChapterID] = sn
	}

	var receipts []ValidatedReviewReceipt
	for _, r := range reviews {
		if r.ActorUserID != actorID {
			return ValidatedReviews{}, reviewStale()
		}

		// Check rule set is still current published
		if r.RuleSetID != publishedSet.ID {
			return ValidatedReviews{}, reviewStale()
		}

		// Check report
		report, err := s.opts.Repositories.Reports.GetReport(ctx, tenantID, r.ReportID)
		if err != nil || report.State != d.ReportStateSucceeded {
			return ValidatedReviews{}, reviewStale()
		}
		// If report was modified later, revision mismatch makes receipt stale
		if report.Revision != r.ReportRevision {
			return ValidatedReviews{}, reviewStale()
		}

		// Check snapshot
		sn, ok := snapshotMap[r.ChapterID]
		if !ok || sn.ChapterRevision != r.ChapterRevision || sn.ContentHash != r.ContentHash {
			return ValidatedReviews{}, reviewStale()
		}

		receipts = append(receipts, ValidatedReviewReceipt{
			ChapterID:       r.ChapterID,
			ChapterRevision: r.ChapterRevision,
			ContentHash:     r.ContentHash,
			ReviewID:        r.ID,
		})
	}

	return ValidatedReviews{Receipts: receipts}, nil
}
