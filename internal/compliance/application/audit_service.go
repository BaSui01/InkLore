package application

import (
	d "InkLore/internal/compliance/domain"
	"InkLore/internal/platform/scope"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

func (s *Service) Scan(ctx context.Context, p scope.Principal, req d.ScanCreate) (d.Report, error) {
	if !scope.ValidID(req.NovelID) || !scope.ValidID(req.ChapterID) {
		return d.Report{}, bad("id")
	}
	if req.ChapterRevision < 1 {
		return d.Report{}, bad("chapter_revision")
	}
	if utf8.RuneCountInString(req.Text) > d.MaxTextRunes {
		return d.Report{}, analysisTooLarge()
	}

	hash := sha256.Sum256([]byte(req.Text))
	computedHash := hex.EncodeToString(hash[:])
	if computedHash != req.ContentHash {
		return d.Report{}, hashMismatch()
	}

	sc, err := s.opts.Scope.Assert(ctx, p, scope.Resource{Kind: "chapter", ID: req.ChapterID}, scope.Read)
	if err != nil {
		return d.Report{}, missing()
	}
	if sc.NovelID == nil || *sc.NovelID != req.NovelID {
		return d.Report{}, missing()
	}

	_, err = s.opts.Scope.AssertChapterSnapshot(ctx, p, req.ChapterID, req.ChapterRevision, req.ContentHash)
	if err != nil {
		return d.Report{}, snapshotMismatch()
	}

	publishedSet, err := s.opts.Repositories.RuleSets.GetCurrentPublishedRuleSet(ctx, p.TenantID)
	if err != nil {
		return d.Report{}, ruleSnapshotUnavailable()
	}
	if publishedSet.Checksum == nil {
		return d.Report{}, ruleSnapshotUnavailable()
	}

	matcher, err := s.getOrCompileSnapshot(ctx, p.TenantID, publishedSet.ID, *publishedSet.Checksum)
	if err != nil {
		return d.Report{}, ruleSnapshotUnavailable()
	}

	reportID := uuid.NewString()
	matches, totalRunes, err := matcher.Match(req.Text)
	if err != nil {
		if errors.Is(err, d.ErrTooManyIssues) {
			return d.Report{}, tooManyIssues()
		}
		if errors.Is(err, d.ErrTextTooLarge) {
			return d.Report{}, analysisTooLarge()
		}
		return d.Report{}, err
	}

	issues, _, err := matcher.BuildIssues(req.Text, p.TenantID, reportID)
	if err != nil {
		if errors.Is(err, d.ErrTooManyIssues) {
			return d.Report{}, tooManyIssues()
		}
		return d.Report{}, err
	}

	score := d.CalculateAIFlavorScore(matches, totalRunes)
	riskCount := len(issues)
	openCount := riskCount
	blockCount := 0
	for _, issue := range issues {
		if issue.Severity == d.SeverityBlock {
			blockCount++
		}
	}

	now := time.Now().UTC()
	expireAt := now.Add(d.DefaultRetentionDays * 24 * time.Hour)

	report := d.Report{
		ID:              reportID,
		TenantID:        p.TenantID,
		CreatedAt:       now,
		UpdatedAt:       now,
		Revision:        1,
		OwnerUserID:     p.ActorID,
		NovelID:         req.NovelID,
		ChapterID:       req.ChapterID,
		ChapterRevision: req.ChapterRevision,
		ContentHash:     req.ContentHash,
		SourceText:      req.Text,
		RuleSetID:       publishedSet.ID,
		Mode:            d.ReportModeRules,
		State:           d.ReportStateSucceeded,
		Budget: d.Budget{
			TotalTokens:     d.DefaultTotalTokens,
			MaxOutputTokens: d.DefaultMaxOutputTokens,
			MaxCalls:        d.DefaultMaxCalls,
		},
		Usage: d.Usage{
			UsedTokens:     0,
			ReservedTokens: 0,
			CallsUsed:      0,
		},
		Summary:        "",
		AIFlavorScore:  &score,
		FormulaVersion: d.FormulaVersion,
		RiskCount:      riskCount,
		OpenCount:      openCount,
		BlockCount:     blockCount,
		ExpireAt:       expireAt,
	}

	err = s.opts.UoW.Within(ctx, func(txctx context.Context) error {
		if err := s.opts.Repositories.Reports.CreateReport(txctx, report); err != nil {
			return err
		}
		if len(issues) > 0 {
			if err := s.opts.Repositories.Issues.BatchCreateIssues(txctx, issues); err != nil {
				return err
			}
		}
		return nil
	})

	return report, err
}

func (s *Service) CreateReport(ctx context.Context, p scope.Principal, req d.ReportCreate) (d.Report, error) {
	if !scope.ValidID(req.NovelID) || !scope.ValidID(req.ChapterID) {
		return d.Report{}, bad("id")
	}
	if req.ChapterRevision < 1 {
		return d.Report{}, bad("chapter_revision")
	}
	textRunes := utf8.RuneCountInString(req.Text)
	if textRunes > d.MaxTextRunes {
		return d.Report{}, analysisTooLarge()
	}

	if req.Budget.TotalTokens < 1000 || req.Budget.TotalTokens > 120000 {
		return d.Report{}, bad("total_tokens")
	}
	if req.Budget.MaxOutputTokens < 256 || req.Budget.MaxOutputTokens > 8000 || req.Budget.MaxOutputTokens > req.Budget.TotalTokens {
		return d.Report{}, bad("max_output_tokens")
	}
	if req.Budget.MaxCalls < 1 || req.Budget.MaxCalls > 6 {
		return d.Report{}, bad("max_calls")
	}

	if textRunes > d.MaxContextTokens {
		return d.Report{}, analysisTooLarge()
	}

	hash := sha256.Sum256([]byte(req.Text))
	computedHash := hex.EncodeToString(hash[:])
	if computedHash != req.ContentHash {
		return d.Report{}, hashMismatch()
	}

	sc, err := s.opts.Scope.Assert(ctx, p, scope.Resource{Kind: "chapter", ID: req.ChapterID}, scope.Read)
	if err != nil {
		return d.Report{}, missing()
	}
	if sc.NovelID == nil || *sc.NovelID != req.NovelID {
		return d.Report{}, missing()
	}

	_, err = s.opts.Scope.AssertChapterSnapshot(ctx, p, req.ChapterID, req.ChapterRevision, req.ContentHash)
	if err != nil {
		return d.Report{}, snapshotMismatch()
	}

	publishedSet, err := s.opts.Repositories.RuleSets.GetCurrentPublishedRuleSet(ctx, p.TenantID)
	if err != nil {
		return d.Report{}, ruleSnapshotUnavailable()
	}

	reportID := uuid.NewString()
	now := time.Now().UTC()
	expireAt := now.Add(d.DefaultRetentionDays * 24 * time.Hour)

	report := d.Report{
		ID:              reportID,
		TenantID:        p.TenantID,
		CreatedAt:       now,
		UpdatedAt:       now,
		Revision:        1,
		OwnerUserID:     p.ActorID,
		NovelID:         req.NovelID,
		ChapterID:       req.ChapterID,
		ChapterRevision: req.ChapterRevision,
		ContentHash:     req.ContentHash,
		SourceText:      req.Text,
		RuleSetID:       publishedSet.ID,
		Mode:            d.ReportModeDeep,
		State:           d.ReportStatePending,
		Budget:          req.Budget,
		Usage: d.Usage{
			UsedTokens:     0,
			ReservedTokens: 0,
			CallsUsed:      0,
		},
		Summary:        "",
		FormulaVersion: d.FormulaVersion,
		RiskCount:      0,
		OpenCount:      0,
		BlockCount:     0,
		ExpireAt:       expireAt,
	}

	err = s.opts.UoW.Within(ctx, func(txctx context.Context) error {
		return s.opts.Repositories.Reports.CreateReport(txctx, report)
	})

	return report, err
}

func (s *Service) GetReport(ctx context.Context, p scope.Principal, id string) (d.Report, error) {
	if !scope.ValidID(id) {
		return d.Report{}, bad("report_id")
	}
	report, err := s.opts.Repositories.Reports.GetReport(ctx, p.TenantID, id)
	if err != nil {
		return d.Report{}, err
	}
	if report.OwnerUserID != p.ActorID {
		return d.Report{}, missing()
	}
	return report, nil
}

func (s *Service) ListReports(ctx context.Context, p scope.Principal, query d.ListQuery) (d.Page[d.Report], error) {
	if query.ChapterID != "" && !scope.ValidID(query.ChapterID) {
		return d.Page[d.Report]{}, bad("chapter_id")
	}
	if query.State != "" {
		switch query.State {
		case d.ReportStatePending, d.ReportStateRunning, d.ReportStateSucceeded, d.ReportStateFailed, d.ReportStateCancelled:
		default:
			return d.Page[d.Report]{}, bad("state")
		}
	}
	if query.Limit <= 0 || query.Limit > 100 {
		query.Limit = 20
	}

	items, hasMore, err := s.opts.Repositories.Reports.ListReports(ctx, p.TenantID, p.ActorID, query)
	if err != nil {
		return d.Page[d.Report]{}, err
	}

	return d.Page[d.Report]{
		Items:   items,
		HasMore: hasMore,
	}, nil
}

func (s *Service) ListIssues(ctx context.Context, p scope.Principal, reportID string, query d.ListQuery) (d.Page[d.Issue], error) {
	if !scope.ValidID(reportID) {
		return d.Page[d.Issue]{}, bad("report_id")
	}
	report, err := s.opts.Repositories.Reports.GetReport(ctx, p.TenantID, reportID)
	if err != nil {
		return d.Page[d.Issue]{}, err
	}
	if report.OwnerUserID != p.ActorID {
		return d.Page[d.Issue]{}, missing()
	}

	if query.State != "" && query.State != d.IssueStateOpen && query.State != d.IssueStateIgnored {
		return d.Page[d.Issue]{}, bad("state")
	}
	if query.Limit <= 0 || query.Limit > 100 {
		query.Limit = 20
	}

	items, hasMore, err := s.opts.Repositories.Issues.ListIssuesByReport(ctx, p.TenantID, reportID, query)
	if err != nil {
		return d.Page[d.Issue]{}, err
	}

	return d.Page[d.Issue]{
		Items:   items,
		HasMore: hasMore,
	}, nil
}

func (s *Service) ExecuteAudit(ctx context.Context, in ExecutionInput) (d.Report, error) {
	var report d.Report
	err := s.opts.UoW.Within(ctx, func(txctx context.Context) error {
		claimed, err := s.opts.Repositories.Reports.ClaimReportExecution(txctx, in.TenantID, in.ResourceID, in.RunID, in.ExecutionKey, in.Fence)
		if err != nil {
			return err
		}
		report = claimed
		return nil
	})
	if err != nil {
		return d.Report{}, err
	}

	ruleSet, err := s.opts.Repositories.RuleSets.GetRuleSet(ctx, in.TenantID, report.RuleSetID)
	if err != nil {
		return d.Report{}, err
	}
	checksum := ""
	if ruleSet.Checksum != nil {
		checksum = *ruleSet.Checksum
	}

	matcher, err := s.getOrCompileSnapshot(ctx, in.TenantID, report.RuleSetID, checksum)
	if err != nil {
		return d.Report{}, ruleSnapshotUnavailable()
	}

	matches, totalRunes, err := matcher.Match(report.SourceText)
	if err != nil {
		_ = s.FailExecution(ctx, in.TenantID, in.ResourceID, in.ExecutionKey, in.Fence, "compliance.analysis_failed")
		return d.Report{}, err
	}

	issues, _, err := matcher.BuildIssues(report.SourceText, in.TenantID, in.ResourceID)
	if err != nil {
		_ = s.FailExecution(ctx, in.TenantID, in.ResourceID, in.ExecutionKey, in.Fence, "compliance.too_many_issues")
		return d.Report{}, err
	}

	summary := ""
	usedTokens := 0
	callsUsed := 0

	if s.opts.Generation != nil {
		llmRes, err := s.opts.Generation.GenerateAudit(ctx, in.TenantID, report.ID, report.NovelID, report.ChapterID, report.SourceText, report.Budget)
		if err == nil {
			summary = llmRes.Summary
			usedTokens = llmRes.UsedTokens
			callsUsed = llmRes.CallsUsed
			issues = append(issues, llmRes.Issues...)
		}
	}

	if len(issues) > d.MaxIssues {
		_ = s.FailExecution(ctx, in.TenantID, in.ResourceID, in.ExecutionKey, in.Fence, "compliance.too_many_issues")
		return d.Report{}, tooManyIssues()
	}

	score := d.CalculateAIFlavorScore(matches, totalRunes)
	riskCount := len(issues)
	openCount := riskCount
	blockCount := 0
	for _, issue := range issues {
		if issue.Severity == d.SeverityBlock {
			blockCount++
		}
	}

	now := time.Now().UTC()
	report.State = d.ReportStateSucceeded
	report.AIFlavorScore = &score
	report.RiskCount = riskCount
	report.OpenCount = openCount
	report.BlockCount = blockCount
	report.Summary = summary
	report.Usage.UsedTokens = usedTokens
	report.Usage.CallsUsed = callsUsed
	report.Revision++
	report.UpdatedAt = now

	err = s.opts.UoW.Within(ctx, func(txctx context.Context) error {
		if err := s.opts.Repositories.Reports.UpdateReport(txctx, report); err != nil {
			return err
		}
		if len(issues) > 0 {
			if err := s.opts.Repositories.Issues.BatchCreateIssues(txctx, issues); err != nil {
				return err
			}
		}
		return nil
	})

	return report, err
}

func (s *Service) CancelExecution(ctx context.Context, tenantID, reportID, key string, fence int64) error {
	return s.opts.UoW.Within(ctx, func(txctx context.Context) error {
		report, err := s.opts.Repositories.Reports.GetReportForUpdate(txctx, tenantID, reportID)
		if err != nil {
			return err
		}
		if report.State != d.ReportStatePending && report.State != d.ReportStateRunning {
			return nil
		}
		report.State = d.ReportStateCancelled
		report.Revision++
		report.UpdatedAt = time.Now().UTC()
		return s.opts.Repositories.Reports.UpdateReport(txctx, report)
	})
}

func (s *Service) FailExecution(ctx context.Context, tenantID, reportID, key string, fence int64, failureCode string) error {
	return s.opts.UoW.Within(ctx, func(txctx context.Context) error {
		report, err := s.opts.Repositories.Reports.GetReportForUpdate(txctx, tenantID, reportID)
		if err != nil {
			return err
		}
		if report.State != d.ReportStatePending && report.State != d.ReportStateRunning {
			return nil
		}
		report.State = d.ReportStateFailed
		report.FailureCode = &failureCode
		report.Revision++
		report.UpdatedAt = time.Now().UTC()
		return s.opts.Repositories.Reports.UpdateReport(txctx, report)
	})
}
