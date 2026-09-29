package application

import (
	d "InkLore/internal/compliance/domain"
	pe "InkLore/internal/platform/errors"
	"context"
	"errors"
)

type Options struct {
	Repositories  d.Repositories
	UoW           UnitOfWork
	Scope         ScopePort
	Generation    GenerationPort
	Outbox        OutboxPort
	SnapshotCache RuleSnapshotCache
}

type Service struct {
	opts Options
}

func NewService(opts Options) (*Service, error) {
	if opts.Repositories.RuleSets == nil ||
		opts.Repositories.Rules == nil ||
		opts.Repositories.Reports == nil ||
		opts.Repositories.Issues == nil ||
		opts.Repositories.Decisions == nil ||
		opts.Repositories.Reviews == nil ||
		opts.Repositories.Rewrites == nil ||
		opts.UoW == nil ||
		opts.Scope == nil {
		return nil, errors.New("compliance: repositories, UoW and scope required")
	}
	return &Service{opts: opts}, nil
}

func bad(msg string) error {
	return pe.Contract(400, "platform.validation_failed", map[string]any{
		"field_errors": []map[string]string{{"field": msg, "reason": "invalid"}},
	})
}

func missing() error {
	return pe.Contract(404, "platform.resource_not_found", nil)
}

func versionConflict(currentRevision int64) error {
	return pe.Contract(409, "compliance.version_conflict", map[string]any{
		"current_revision": currentRevision,
	})
}

func invalidState() error {
	return pe.Contract(409, "compliance.invalid_state", nil)
}

func ruleDuplicate() error {
	return pe.Contract(409, "compliance.rule_duplicate", nil)
}

func snapshotMismatch() error {
	return pe.Contract(409, "compliance.snapshot_mismatch", nil)
}

func reviewStale() error {
	return pe.Contract(409, "compliance.review_stale", nil)
}

func invalidRule() error {
	return pe.Contract(422, "compliance.invalid_rule", nil)
}

func ruleLimitExceeded() error {
	return pe.Contract(422, "compliance.rule_limit_exceeded", nil)
}

func hashMismatch() error {
	return pe.Contract(422, "compliance.hash_mismatch", nil)
}

func tooManyIssues() error {
	return pe.Contract(422, "compliance.too_many_issues", nil)
}

func invalidIssueSelection() error {
	return pe.Contract(422, "compliance.invalid_issue_selection", nil)
}

func reviewRequired() error {
	return pe.Contract(422, "compliance.review_required", nil)
}

func analysisTooLarge() error {
	return pe.Contract(422, "compliance.analysis_too_large", nil)
}

func budgetExhausted() error {
	return pe.Contract(429, "compliance.budget_exhausted", map[string]any{
		"retry_after_seconds": int64(60),
	})
}

func ruleSnapshotUnavailable() error {
	return pe.Contract(503, "compliance.rule_snapshot_unavailable", nil)
}

// getOrCompileSnapshot loads or compiles the matcher for a given rule set and checksum.
func (s *Service) getOrCompileSnapshot(ctx context.Context, tenantID, ruleSetID, checksum string) (*d.Matcher, error) {
	if s.opts.SnapshotCache != nil && checksum != "" {
		if cached, ok := s.opts.SnapshotCache.Get(tenantID, checksum); ok {
			return cached, nil
		}
	}

	rules, err := s.opts.Repositories.Rules.GetAllRulesBySet(ctx, tenantID, ruleSetID)
	if err != nil {
		return nil, err
	}

	matcher, err := d.NewMatcher(rules)
	if err != nil {
		return nil, ruleSnapshotUnavailable()
	}

	if s.opts.SnapshotCache != nil && checksum != "" {
		s.opts.SnapshotCache.Put(tenantID, checksum, matcher)
	}

	return matcher, nil
}
