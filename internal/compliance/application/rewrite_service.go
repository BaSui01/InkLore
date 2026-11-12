package application

import (
	d "InkLore/internal/compliance/domain"
	"InkLore/internal/platform/scope"
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

func (s *Service) CreateRewrite(ctx context.Context, p scope.Principal, req d.RewriteCreate) (d.Rewrite, error) {
	if !scope.ValidID(req.ReportID) {
		return d.Rewrite{}, bad("report_id")
	}
	if req.ExpectedRevision < 1 {
		return d.Rewrite{}, bad("expected_revision")
	}
	if len(req.SelectedIssueIDs) < 1 || len(req.SelectedIssueIDs) > 100 {
		return d.Rewrite{}, invalidIssueSelection()
	}
	// Check unique selected issue IDs
	seen := make(map[string]bool, len(req.SelectedIssueIDs))
	for _, id := range req.SelectedIssueIDs {
		if !scope.ValidID(id) || seen[id] {
			return d.Rewrite{}, invalidIssueSelection()
		}
		seen[id] = true
	}

	if req.MaxRounds < 1 || req.MaxRounds > d.MaxRounds {
		return d.Rewrite{}, bad("max_rounds")
	}
	if req.Budget.TotalTokens < 1000 || req.Budget.TotalTokens > 120000 {
		return d.Rewrite{}, bad("total_tokens")
	}
	if req.Budget.MaxOutputTokens < 256 || req.Budget.MaxOutputTokens > 8000 || req.Budget.MaxOutputTokens > req.Budget.TotalTokens {
		return d.Rewrite{}, bad("max_output_tokens")
	}
	if req.Budget.MaxCalls < 1 || req.Budget.MaxCalls > 6 {
		return d.Rewrite{}, bad("max_calls")
	}

	var created d.Rewrite
	err := s.opts.UoW.Within(ctx, func(txctx context.Context) error {
		report, err := s.opts.Repositories.Reports.GetReport(txctx, p.TenantID, req.ReportID)
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

		issues, err := s.opts.Repositories.Issues.GetIssuesByIDs(txctx, p.TenantID, req.ReportID, req.SelectedIssueIDs)
		if err != nil || len(issues) != len(req.SelectedIssueIDs) {
			return invalidIssueSelection()
		}
		for _, iss := range issues {
			if iss.State != d.IssueStateOpen {
				return invalidIssueSelection()
			}
		}

		now := time.Now().UTC()
		rewrite := d.Rewrite{
			ID:               uuid.NewString(),
			TenantID:         p.TenantID,
			CreatedAt:        now,
			UpdatedAt:        now,
			Revision:         1,
			OwnerUserID:      p.ActorID,
			ReportID:         req.ReportID,
			ReportRevision:   report.Revision,
			SelectedIssueIDs: req.SelectedIssueIDs,
			State:            d.ReportStatePending,
			MaxRounds:        req.MaxRounds,
			RoundsUsed:       0,
			Budget:           req.Budget,
			Usage: d.Usage{
				UsedTokens:     0,
				ReservedTokens: 0,
				CallsUsed:      0,
			},
			Patches: []d.Patch{},
		}

		if err := s.opts.Repositories.Rewrites.CreateRewrite(txctx, rewrite); err != nil {
			return err
		}

		created = rewrite
		return nil
	})

	return created, err
}

func (s *Service) GetRewrite(ctx context.Context, p scope.Principal, id string) (d.Rewrite, error) {
	if !scope.ValidID(id) {
		return d.Rewrite{}, bad("rewrite_id")
	}
	rw, err := s.opts.Repositories.Rewrites.GetRewrite(ctx, p.TenantID, id)
	if err != nil {
		return d.Rewrite{}, err
	}
	if rw.OwnerUserID != p.ActorID {
		return d.Rewrite{}, missing()
	}
	return rw, nil
}

func (s *Service) ListRewrites(ctx context.Context, p scope.Principal, query d.ListQuery) (d.Page[d.Rewrite], error) {
	if query.ReportID != "" && !scope.ValidID(query.ReportID) {
		return d.Page[d.Rewrite]{}, bad("report_id")
	}
	if query.Limit <= 0 || query.Limit > 100 {
		query.Limit = 20
	}

	items, hasMore, err := s.opts.Repositories.Rewrites.ListRewrites(ctx, p.TenantID, p.ActorID, query)
	if err != nil {
		return d.Page[d.Rewrite]{}, err
	}

	return d.Page[d.Rewrite]{
		Items:   items,
		HasMore: hasMore,
	}, nil
}

func (s *Service) ExecuteRewrite(ctx context.Context, in ExecutionInput) (d.Rewrite, error) {
	var rewrite d.Rewrite
	err := s.opts.UoW.Within(ctx, func(txctx context.Context) error {
		claimed, err := s.opts.Repositories.Rewrites.ClaimRewriteExecution(txctx, in.TenantID, in.ResourceID, in.RunID, in.ExecutionKey, in.Fence)
		if err != nil {
			return err
		}
		rewrite = claimed
		return nil
	})
	if err != nil {
		return d.Rewrite{}, err
	}

	report, err := s.opts.Repositories.Reports.GetReport(ctx, in.TenantID, rewrite.ReportID)
	if err != nil {
		return d.Rewrite{}, err
	}

	ruleSet, err := s.opts.Repositories.RuleSets.GetRuleSet(ctx, in.TenantID, report.RuleSetID)
	if err != nil {
		return d.Rewrite{}, err
	}
	checksum := ""
	if ruleSet.Checksum != nil {
		checksum = *ruleSet.Checksum
	}

	matcher, err := s.getOrCompileSnapshot(ctx, in.TenantID, report.RuleSetID, checksum)
	if err != nil {
		return d.Rewrite{}, ruleSnapshotUnavailable()
	}

	issues, err := s.opts.Repositories.Issues.GetIssuesByIDs(ctx, in.TenantID, rewrite.ReportID, rewrite.SelectedIssueIDs)
	if err != nil {
		return d.Rewrite{}, err
	}

	sourceText := report.SourceText
	bestCandidateText := sourceText
	var bestPatches []d.Patch
	var bestCandidateHash string
	var bestRemainingRiskCount int = len(issues)
	stopReason := d.StopReasonClean
	roundsUsed := 0
	usedTokens := 0
	callsUsed := 0

	for round := 1; round <= rewrite.MaxRounds; round++ {
		roundsUsed = round
		var roundPatches []d.Patch

		if s.opts.Generation != nil {
			llmRes, err := s.opts.Generation.GenerateRewrite(ctx, in.TenantID, rewrite.ID, report.NovelID, report.ChapterID, sourceText, issues, round, rewrite.Budget)
			if err != nil {
				// Model, budget and unknown-dispatch failures must not become a
				// successful deterministic rewrite. Persist the real failed state
				// even when the original request context has been cancelled.
				rewrite.State = d.ReportStateFailed
				failureCode := "compliance.analysis_failed"
				rewrite.FailureCode = &failureCode
				rewrite.Revision++
				rewrite.UpdatedAt = time.Now().UTC()
				cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
				persistErr := s.opts.UoW.Within(cleanup, func(txctx context.Context) error {
					return s.opts.Repositories.Rewrites.UpdateRewrite(txctx, rewrite)
				})
				cancel()
				return d.Rewrite{}, errors.Join(err, persistErr)
			}
			roundPatches = llmRes.Patches
			usedTokens += llmRes.UsedTokens
			callsUsed += llmRes.CallsUsed
		} else {
			// The explicit rules-only mode uses existing scanner suggestions.
			for _, iss := range issues {
				if iss.State != d.IssueStateOpen {
					continue
				}
				roundPatches = append(roundPatches, d.Patch{
					StartOffset: iss.StartOffset,
					EndOffset:   iss.EndOffset,
					OldText:     iss.Excerpt,
					NewText:     iss.Suggestion,
				})
			}
		}

		candText, candHash, err := d.ApplyPatches(sourceText, roundPatches)
		if err != nil {
			if round == 1 {
				rewrite.State = d.ReportStateFailed
				failCode := "compliance.invalid_candidate"
				rewrite.FailureCode = &failCode
				_ = s.opts.UoW.Within(ctx, func(txctx context.Context) error {
					return s.opts.Repositories.Rewrites.UpdateRewrite(txctx, rewrite)
				})
				return d.Rewrite{}, err
			}
			stopReason = d.StopReasonNoImprovement
			break
		}

		// Re-run scanner on candidate text
		candIssues, _, scanErr := matcher.BuildIssues(candText, in.TenantID, rewrite.ReportID)
		if scanErr != nil {
			stopReason = d.StopReasonNoImprovement
			break
		}

		candRemainingRisks := len(candIssues)
		if candRemainingRisks <= bestRemainingRiskCount {
			bestCandidateText = candText
			bestCandidateHash = candHash
			bestPatches = roundPatches
			bestRemainingRiskCount = candRemainingRisks
		}

		if candRemainingRisks == 0 {
			stopReason = d.StopReasonClean
			break
		}

		if candHash == bestCandidateHash && round > 1 {
			stopReason = d.StopReasonNoImprovement
			break
		}

		if callsUsed >= rewrite.Budget.MaxCalls || usedTokens >= rewrite.Budget.TotalTokens {
			stopReason = d.StopReasonBudget
			break
		}

		if round == rewrite.MaxRounds {
			stopReason = d.StopReasonMaxRounds
			break
		}
	}

	now := time.Now().UTC()
	rewrite.State = d.ReportStateSucceeded
	rewrite.RoundsUsed = roundsUsed
	rewrite.Patches = bestPatches
	rewrite.CandidateText = &bestCandidateText
	rewrite.CandidateHash = &bestCandidateHash
	rewrite.RemainingRiskCount = &bestRemainingRiskCount
	rewrite.StopReason = &stopReason
	rewrite.Usage.UsedTokens = usedTokens
	rewrite.Usage.CallsUsed = callsUsed
	rewrite.Revision++
	rewrite.UpdatedAt = now

	err = s.opts.UoW.Within(ctx, func(txctx context.Context) error {
		return s.opts.Repositories.Rewrites.UpdateRewrite(txctx, rewrite)
	})

	return rewrite, err
}
