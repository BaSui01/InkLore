package application

import (
	d "InkLore/internal/compliance/domain"
	"InkLore/internal/platform/scope"
	"context"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

func (s *Service) IgnoreIssue(ctx context.Context, p scope.Principal, reportID, issueID string, req d.ReasonCommand) (d.Issue, error) {
	if !scope.ValidID(reportID) || !scope.ValidID(issueID) {
		return d.Issue{}, bad("id")
	}
	if req.ExpectedRevision < 1 {
		return d.Issue{}, bad("expected_revision")
	}
	reasonLen := utf8.RuneCountInString(req.Reason)
	if reasonLen < 1 || reasonLen > 500 {
		return d.Issue{}, bad("reason")
	}

	var updated d.Issue
	err := s.opts.UoW.Within(ctx, func(txctx context.Context) error {
		report, err := s.opts.Repositories.Reports.GetReportForUpdate(txctx, p.TenantID, reportID)
		if err != nil {
			return err
		}
		if report.OwnerUserID != p.ActorID {
			return missing()
		}

		issue, err := s.opts.Repositories.Issues.GetIssueForUpdate(txctx, p.TenantID, reportID, issueID)
		if err != nil {
			return err
		}
		if issue.Revision != req.ExpectedRevision {
			return versionConflict(issue.Revision)
		}
		if issue.State == d.IssueStateIgnored {
			return invalidState()
		}

		now := time.Now().UTC()
		issue.State = d.IssueStateIgnored
		issue.Revision++
		issue.UpdatedAt = now

		if err := s.opts.Repositories.Issues.UpdateIssue(txctx, issue, req.ExpectedRevision); err != nil {
			return err
		}

		if report.OpenCount > 0 {
			report.OpenCount--
		}
		report.Revision++
		report.UpdatedAt = now
		if err := s.opts.Repositories.Reports.UpdateReport(txctx, report); err != nil {
			return err
		}

		decision := d.IssueDecision{
			ID:            uuid.NewString(),
			TenantID:      p.TenantID,
			CreatedAt:     now,
			UpdatedAt:     now,
			ReportID:      reportID,
			IssueID:       issueID,
			ActorUserID:   p.ActorID,
			Action:        d.DecisionActionIgnore,
			Reason:        req.Reason,
			IssueRevision: issue.Revision,
		}
		if err := s.opts.Repositories.Decisions.CreateDecision(txctx, decision); err != nil {
			return err
		}

		updated = issue
		return nil
	})

	return updated, err
}

func (s *Service) ReopenIssue(ctx context.Context, p scope.Principal, reportID, issueID string, req d.ReasonCommand) (d.Issue, error) {
	if !scope.ValidID(reportID) || !scope.ValidID(issueID) {
		return d.Issue{}, bad("id")
	}
	if req.ExpectedRevision < 1 {
		return d.Issue{}, bad("expected_revision")
	}
	reasonLen := utf8.RuneCountInString(req.Reason)
	if reasonLen < 1 || reasonLen > 500 {
		return d.Issue{}, bad("reason")
	}

	var updated d.Issue
	err := s.opts.UoW.Within(ctx, func(txctx context.Context) error {
		report, err := s.opts.Repositories.Reports.GetReportForUpdate(txctx, p.TenantID, reportID)
		if err != nil {
			return err
		}
		if report.OwnerUserID != p.ActorID {
			return missing()
		}

		issue, err := s.opts.Repositories.Issues.GetIssueForUpdate(txctx, p.TenantID, reportID, issueID)
		if err != nil {
			return err
		}
		if issue.Revision != req.ExpectedRevision {
			return versionConflict(issue.Revision)
		}
		if issue.State != d.IssueStateIgnored {
			return invalidState()
		}

		now := time.Now().UTC()
		issue.State = d.IssueStateOpen
		issue.Revision++
		issue.UpdatedAt = now

		if err := s.opts.Repositories.Issues.UpdateIssue(txctx, issue, req.ExpectedRevision); err != nil {
			return err
		}

		report.OpenCount++
		report.Revision++
		report.UpdatedAt = now
		if err := s.opts.Repositories.Reports.UpdateReport(txctx, report); err != nil {
			return err
		}

		decision := d.IssueDecision{
			ID:            uuid.NewString(),
			TenantID:      p.TenantID,
			CreatedAt:     now,
			UpdatedAt:     now,
			ReportID:      reportID,
			IssueID:       issueID,
			ActorUserID:   p.ActorID,
			Action:        d.DecisionActionReopen,
			Reason:        req.Reason,
			IssueRevision: issue.Revision,
		}
		if err := s.opts.Repositories.Decisions.CreateDecision(txctx, decision); err != nil {
			return err
		}

		updated = issue
		return nil
	})

	return updated, err
}

func (s *Service) ListIssueDecisions(ctx context.Context, p scope.Principal, reportID, issueID string, query d.ListQuery) (d.Page[d.IssueDecision], error) {
	if !scope.ValidID(reportID) || !scope.ValidID(issueID) {
		return d.Page[d.IssueDecision]{}, bad("id")
	}

	report, err := s.opts.Repositories.Reports.GetReport(ctx, p.TenantID, reportID)
	if err != nil {
		return d.Page[d.IssueDecision]{}, err
	}
	if report.OwnerUserID != p.ActorID {
		return d.Page[d.IssueDecision]{}, missing()
	}

	_, err = s.opts.Repositories.Issues.GetIssue(ctx, p.TenantID, reportID, issueID)
	if err != nil {
		return d.Page[d.IssueDecision]{}, err
	}

	if query.Limit <= 0 || query.Limit > 100 {
		query.Limit = 20
	}

	items, hasMore, err := s.opts.Repositories.Decisions.ListDecisionsByIssue(ctx, p.TenantID, reportID, issueID, query)
	if err != nil {
		return d.Page[d.IssueDecision]{}, err
	}

	return d.Page[d.IssueDecision]{
		Items:   items,
		HasMore: hasMore,
	}, nil
}
