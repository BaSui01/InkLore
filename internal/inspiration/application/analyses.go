package application

import (
	d "InkLore/internal/inspiration/domain"
	"InkLore/internal/platform/cursor"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/scope"
	"context"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

func (s *Service) CreateAnalysis(ctx context.Context, p scope.Principal, in d.AnalysisCreate) (d.Analysis, error) {
	var out d.Analysis
	switch in.Kind {
	case d.AnalysisKindDeconstruction, d.AnalysisKindStyle, d.AnalysisKindIdeas:
	default:
		return out, validationFailed("kind")
	}

	if in.Kind == d.AnalysisKindDeconstruction || in.Kind == d.AnalysisKindStyle {
		if in.AssetID == nil || !scope.ValidID(*in.AssetID) || in.AssetRevision == nil || *in.AssetRevision < 1 {
			return out, validationFailed("asset_id")
		}
	}

	briefRunes := utf8.RuneCountInString(in.Brief)
	if in.Kind == d.AnalysisKindIdeas {
		if briefRunes < d.MinIdeasBriefRunes || briefRunes > d.MaxBriefRunes {
			return out, validationFailed("brief")
		}
	} else {
		if briefRunes > d.MaxBriefRunes {
			return out, validationFailed("brief")
		}
	}

	// Budget validation and defaults
	budget := in.Budget
	if budget.TotalTokens == 0 {
		budget.TotalTokens = 30000
	}
	if budget.MaxOutputTokens == 0 {
		budget.MaxOutputTokens = 4000
	}
	if budget.MaxCalls == 0 {
		budget.MaxCalls = 6
	}
	if budget.TotalTokens < 1000 || budget.TotalTokens > 120000 {
		return out, validationFailed("budget.total_tokens")
	}
	if budget.MaxOutputTokens < 256 || budget.MaxOutputTokens > 8000 || budget.MaxOutputTokens > budget.TotalTokens {
		return out, validationFailed("budget.max_output_tokens")
	}
	if budget.MaxCalls < 1 || budget.MaxCalls > 20 {
		return out, validationFailed("budget.max_calls")
	}

	if in.NovelID != nil && !scope.ValidID(*in.NovelID) {
		return out, validationFailed("novel_id")
	}

	// Check asset if specified
	if in.AssetID != nil {
		asset, err := s.o.Repository.GetAsset(ctx, p.TenantID, *in.AssetID, false)
		if err != nil || asset.OwnerUserID != p.ActorID {
			return out, missing()
		}
		if asset.State == d.AssetStateDeleted {
			return out, stateError()
		}
		if in.AssetRevision != nil && asset.Revision != *in.AssetRevision {
			return out, sourceChanged()
		}
		if in.Kind == d.AnalysisKindStyle && asset.CodepointCount < d.MinStyleCodepoints {
			return out, validationFailed("asset_codepoints")
		}
	}

	// Rate limit / in-flight check
	tenantCount, authorCount, err := s.o.Repository.CountInFlightAnalyses(ctx, p.TenantID, p.ActorID)
	if err != nil {
		return out, err
	}
	if tenantCount >= 2 || authorCount >= 1 {
		return out, budgetExhausted()
	}

	now := s.now()
	analysis := d.Analysis{
		ID:            uuid.NewString(),
		TenantID:      p.TenantID,
		CreatedAt:     now,
		UpdatedAt:     now,
		Revision:      1,
		OwnerUserID:   p.ActorID,
		NovelID:       in.NovelID,
		AssetID:       in.AssetID,
		AssetRevision: in.AssetRevision,
		Kind:          in.Kind,
		Brief:         in.Brief,
		State:         d.AnalysisStatePending,
		Budget:        budget,
		Usage:         d.Usage{UsedTokens: 0, ReservedTokens: 0, CallsUsed: 0},
	}

	err = s.o.UoW.Within(ctx, func(tx context.Context) error {
		if err := s.o.Repository.CreateAnalysis(tx, analysis); err != nil {
			return err
		}
		return s.event(tx, p, "inspiration.analysis_created", "inspiration_analysis", analysis.ID, 1)
	})
	if err != nil {
		return out, err
	}
	return analysis, nil
}

func (s *Service) GetAnalysis(ctx context.Context, p scope.Principal, id string) (d.Analysis, error) {
	if !scope.ValidID(id) {
		return d.Analysis{}, validationFailed("id")
	}
	analysis, err := s.o.Repository.GetAnalysis(ctx, p.TenantID, id)
	if err != nil {
		return d.Analysis{}, missing()
	}
	if analysis.OwnerUserID != p.ActorID {
		return d.Analysis{}, missing()
	}
	return analysis, nil
}

func (s *Service) ListAnalyses(ctx context.Context, p scope.Principal, novelID *string, state *string, limit int, cursorStr string) (d.PageResult[d.Analysis], error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	var cursorTime *time.Time
	var cursorID string
	if cursorStr != "" && s.o.Cursor != nil {
		var tc timeCursor
		binding := cursor.Binding{TenantID: p.TenantID, ActorID: p.ActorID, Operation: "inspiration_list_analyses"}
		if err := s.o.Cursor.Decode(cursorStr, binding, &tc); err != nil {
			return d.PageResult[d.Analysis]{}, err
		}
		cursorTime = &tc.At
		cursorID = tc.ID
	}

	items, hasMore, err := s.o.Repository.ListAnalyses(ctx, p.TenantID, p.ActorID, novelID, state, limit, cursorTime, cursorID)
	if err != nil {
		return d.PageResult[d.Analysis]{}, err
	}

	var nextCursor *string
	if hasMore && len(items) > 0 && s.o.Cursor != nil {
		last := items[len(items)-1]
		binding := cursor.Binding{TenantID: p.TenantID, ActorID: p.ActorID, Operation: "inspiration_list_analyses"}
		token, err := s.o.Cursor.Encode(binding, timeCursor{At: last.UpdatedAt, ID: last.ID})
		if err == nil {
			nextCursor = &token
		}
	}
	return d.PageResult[d.Analysis]{Items: items, NextCursor: nextCursor, HasMore: hasMore}, nil
}

func (s *Service) GetReport(ctx context.Context, p scope.Principal, analysisID string) (d.Report, error) {
	if !scope.ValidID(analysisID) {
		return d.Report{}, validationFailed("analysis_id")
	}
	analysis, err := s.o.Repository.GetAnalysis(ctx, p.TenantID, analysisID)
	if err != nil || analysis.OwnerUserID != p.ActorID {
		return d.Report{}, missing()
	}
	if analysis.State != d.AnalysisStateSucceeded {
		return d.Report{}, missing()
	}
	report, err := s.o.Repository.GetReportByAnalysisID(ctx, p.TenantID, analysisID)
	if err != nil {
		return d.Report{}, missing()
	}
	return report, nil
}

func (s *Service) ExecuteAnalysis(ctx context.Context, p scope.Principal, analysisID, runID, executionKey string, fence int64, expectedRevision int64) (d.Report, error) {
	var out d.Report
	if !scope.ValidID(analysisID) || !scope.ValidID(runID) || !scope.ValidID(executionKey) {
		return out, validationFailed("parameters")
	}

	claimed, err := s.o.Repository.ClaimAnalysis(ctx, p.TenantID, analysisID, runID, executionKey, fence, expectedRevision)
	if err != nil {
		return out, err
	}

	// Prepare context segments
	var rawRunes []rune
	var segments []d.Segment
	if claimed.AssetID != nil {
		asset, err := s.o.Repository.GetAsset(ctx, p.TenantID, *claimed.AssetID, false)
		if err != nil {
			return out, sourceChanged()
		}
		if asset.Revision != *claimed.AssetRevision {
			return out, sourceChanged()
		}
		bytes, err := s.o.Files.ReadAuthorized(ctx, identity.Principal{TenantID: p.TenantID, UserID: p.ActorID, SessionID: p.SessionID, Audience: p.Audience}, asset.FileID)
		if err != nil {
			return out, err
		}
		norm, err := d.NormalizeText(string(bytes))
		if err != nil {
			return out, err
		}
		rawRunes = []rune(norm)
		segs, _, err := s.o.Repository.GetSegments(ctx, p.TenantID, *claimed.AssetID, 100, 0)
		if err != nil {
			return out, err
		}
		segments = segs
	}

	contextStrings := make([]string, 0, len(segments))
	for _, seg := range segments {
		if seg.EndOffset <= len(rawRunes) {
			contextStrings = append(contextStrings, string(rawRunes[seg.StartOffset:seg.EndOffset]))
		}
	}

	// Call LLM Generation port
	genResp, err := s.o.Generation.Generate(ctx, GenerationRequest{
		TaskKind:        claimed.Kind,
		ContextSegments: contextStrings,
		Brief:           claimed.Brief,
		MaxOutputTokens: claimed.Budget.MaxOutputTokens,
		ExecutionKey:    executionKey,
		RemainingBudget: claimed.Budget.TotalTokens - claimed.Usage.UsedTokens,
	})
	if err != nil {
		_ = s.o.Repository.FailAnalysis(ctx, p.TenantID, analysisID, fence, "inspiration.generation_failed", 0, 0)
		return out, err
	}

	// Validate evidence against source runes
	if claimed.AssetID != nil {
		for _, f := range genResp.Findings {
			for _, ev := range f.Evidence {
				if !d.ValidateEvidence(ev, rawRunes, *claimed.AssetID, *claimed.AssetRevision, ev.ContentHash) {
					_ = s.o.Repository.FailAnalysis(ctx, p.TenantID, analysisID, fence, "inspiration.invalid_evidence", genResp.UsedTokens, genResp.CallsUsed)
					return out, invalidEvidence()
				}
			}
		}
		for _, pat := range genResp.Patterns {
			for _, ev := range pat.Evidence {
				if !d.ValidateEvidence(ev, rawRunes, *claimed.AssetID, *claimed.AssetRevision, ev.ContentHash) {
					_ = s.o.Repository.FailAnalysis(ctx, p.TenantID, analysisID, fence, "inspiration.invalid_evidence", genResp.UsedTokens, genResp.CallsUsed)
					return out, invalidEvidence()
				}
			}
		}
	}

	now := s.now()
	report := d.Report{
		ID:           uuid.NewString(),
		TenantID:     p.TenantID,
		CreatedAt:    now,
		UpdatedAt:    now,
		Revision:     1,
		AnalysisID:   analysisID,
		Summary:      genResp.Summary,
		Findings:     genResp.Findings,
		Patterns:     genResp.Patterns,
		StyleMetrics: genResp.StyleMetrics,
		Ideas:        genResp.Ideas,
	}

	err = s.o.UoW.Within(ctx, func(tx context.Context) error {
		if err := s.o.Repository.CompleteAnalysisWithReport(tx, p.TenantID, analysisID, fence, report, genResp.UsedTokens, genResp.CallsUsed); err != nil {
			return err
		}
		out = report
		return s.event(tx, p, "inspiration.analysis_succeeded", "inspiration_analysis", analysisID, claimed.Revision+1)
	})
	return out, err
}

func (s *Service) CancelExecution(ctx context.Context, p scope.Principal, analysisID, executionKey string, fence int64, reason string) error {
	return s.o.UoW.Within(ctx, func(tx context.Context) error {
		curr, err := s.o.Repository.GetAnalysis(tx, p.TenantID, analysisID)
		if err != nil {
			return missing()
		}
		if curr.State == d.AnalysisStateSucceeded || curr.State == d.AnalysisStateFailed || curr.State == d.AnalysisStateCancelled {
			return stateError()
		}
		return s.o.Repository.CancelAnalysis(tx, p.TenantID, analysisID, curr.Revision, reason)
	})
}

func (s *Service) FailExecution(ctx context.Context, p scope.Principal, analysisID, executionKey string, fence int64, failureCode string, usedTokens, callsUsed int) error {
	return s.o.Repository.FailAnalysis(ctx, p.TenantID, analysisID, fence, failureCode, usedTokens, callsUsed)
}
