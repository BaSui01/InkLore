package application

import (
	d "InkLore/internal/compliance/domain"
	"InkLore/internal/platform/scope"
	"context"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

func (s *Service) CreateRuleSet(ctx context.Context, p scope.Principal, req d.RuleSetCreate) (d.RuleSet, error) {
	nameLen := utf8.RuneCountInString(req.Name)
	if nameLen < 1 || nameLen > 120 {
		return d.RuleSet{}, bad("name")
	}
	if req.BaseRuleSetID != nil && !scope.ValidID(*req.BaseRuleSetID) {
		return d.RuleSet{}, bad("base_rule_set_id")
	}

	var created d.RuleSet
	err := s.opts.UoW.Within(ctx, func(txctx context.Context) error {
		versionNo, err := s.opts.Repositories.RuleSets.GetNextVersionNo(txctx, p.TenantID)
		if err != nil {
			return err
		}

		now := time.Now().UTC()
		set := d.RuleSet{
			ID:        uuid.NewString(),
			TenantID:  p.TenantID,
			CreatedAt: now,
			UpdatedAt: now,
			Revision:  1,
			Name:      req.Name,
			VersionNo: versionNo,
			State:     d.RuleSetStateDraft,
			CreatedBy: p.ActorID,
		}

		if err := s.opts.Repositories.RuleSets.CreateRuleSet(txctx, set); err != nil {
			return err
		}

		if req.BaseRuleSetID != nil {
			baseSet, err := s.opts.Repositories.RuleSets.GetRuleSet(txctx, p.TenantID, *req.BaseRuleSetID)
			if err != nil {
				return err
			}
			if err := s.opts.Repositories.Rules.CopyRules(txctx, p.TenantID, baseSet.ID, set.ID); err != nil {
				return err
			}
		}

		created = set
		return nil
	})

	return created, err
}

func (s *Service) GetRuleSet(ctx context.Context, p scope.Principal, id string) (d.RuleSet, error) {
	if !scope.ValidID(id) {
		return d.RuleSet{}, bad("rule_set_id")
	}
	return s.opts.Repositories.RuleSets.GetRuleSet(ctx, p.TenantID, id)
}

func (s *Service) ListRuleSets(ctx context.Context, p scope.Principal, query d.ListQuery) (d.Page[d.RuleSet], error) {
	if query.State != "" && query.State != d.RuleSetStateDraft && query.State != d.RuleSetStatePublished && query.State != d.RuleSetStateRetired {
		return d.Page[d.RuleSet]{}, bad("state")
	}
	if query.Limit <= 0 || query.Limit > 100 {
		query.Limit = 20
	}

	items, hasMore, err := s.opts.Repositories.RuleSets.ListRuleSets(ctx, p.TenantID, query)
	if err != nil {
		return d.Page[d.RuleSet]{}, err
	}

	return d.Page[d.RuleSet]{
		Items:   items,
		HasMore: hasMore,
	}, nil
}

func (s *Service) CreateRule(ctx context.Context, p scope.Principal, ruleSetID string, req d.RuleCreate) (d.Rule, error) {
	if !scope.ValidID(ruleSetID) {
		return d.Rule{}, bad("rule_set_id")
	}
	if req.ExpectedRevision < 1 {
		return d.Rule{}, bad("expected_revision")
	}
	if req.Weight < 1 || req.Weight > 100 {
		return d.Rule{}, bad("weight")
	}
	catLen := utf8.RuneCountInString(req.Category)
	if catLen < 1 || catLen > 60 {
		return d.Rule{}, bad("category")
	}
	if utf8.RuneCountInString(req.Advice) > 2000 {
		return d.Rule{}, bad("advice")
	}

	candidateRule := d.Rule{
		RuleSetID:  ruleSetID,
		Kind:       req.Kind,
		Expression: req.Expression,
		Severity:   req.Severity,
		Category:   req.Category,
		Advice:     req.Advice,
		Weight:     req.Weight,
		Enabled:    req.Enabled,
	}

	if _, err := d.CompileRule(candidateRule); err != nil {
		return d.Rule{}, invalidRule()
	}

	var created d.Rule
	err := s.opts.UoW.Within(ctx, func(txctx context.Context) error {
		set, err := s.opts.Repositories.RuleSets.GetRuleSetForUpdate(txctx, p.TenantID, ruleSetID)
		if err != nil {
			return err
		}
		if set.State != d.RuleSetStateDraft {
			return invalidState()
		}
		if set.Revision != req.ExpectedRevision {
			return versionConflict(set.Revision)
		}

		totalCount, patternCount, err := s.opts.Repositories.Rules.CountRulesBySet(txctx, p.TenantID, ruleSetID)
		if err != nil {
			return err
		}
		if totalCount >= d.MaxRules {
			return ruleLimitExceeded()
		}
		if req.Kind == d.RuleKindAIPattern && req.Enabled && patternCount >= d.MaxPatterns {
			return ruleLimitExceeded()
		}

		exists, err := s.opts.Repositories.Rules.ExistsRule(txctx, p.TenantID, ruleSetID, req.Kind, req.Expression, "")
		if err != nil {
			return err
		}
		if exists {
			return ruleDuplicate()
		}

		now := time.Now().UTC()
		rule := d.Rule{
			ID:         uuid.NewString(),
			TenantID:   p.TenantID,
			CreatedAt:  now,
			UpdatedAt:  now,
			Revision:   1,
			RuleSetID:  ruleSetID,
			Kind:       req.Kind,
			Expression: req.Expression,
			Severity:   req.Severity,
			Category:   req.Category,
			Advice:     req.Advice,
			Weight:     req.Weight,
			Enabled:    req.Enabled,
		}

		if err := s.opts.Repositories.Rules.CreateRule(txctx, rule); err != nil {
			return err
		}

		// Increment rule set revision
		set.Revision++
		set.UpdatedAt = now
		if err := s.opts.Repositories.RuleSets.CreateRuleSet(txctx, set); err != nil {
			// Update handled via repository
		}

		created = rule
		return nil
	})

	return created, err
}

func (s *Service) UpdateRule(ctx context.Context, p scope.Principal, ruleSetID, ruleID string, req d.RuleUpdate) (d.Rule, error) {
	if !scope.ValidID(ruleSetID) || !scope.ValidID(ruleID) {
		return d.Rule{}, bad("id")
	}
	if req.ExpectedRevision < 1 {
		return d.Rule{}, bad("expected_revision")
	}
	if req.Weight < 1 || req.Weight > 100 {
		return d.Rule{}, bad("weight")
	}
	catLen := utf8.RuneCountInString(req.Category)
	if catLen < 1 || catLen > 60 {
		return d.Rule{}, bad("category")
	}
	if utf8.RuneCountInString(req.Advice) > 2000 {
		return d.Rule{}, bad("advice")
	}

	candidateRule := d.Rule{
		ID:         ruleID,
		RuleSetID:  ruleSetID,
		Kind:       req.Kind,
		Expression: req.Expression,
		Severity:   req.Severity,
		Category:   req.Category,
		Advice:     req.Advice,
		Weight:     req.Weight,
		Enabled:    req.Enabled,
	}

	if _, err := d.CompileRule(candidateRule); err != nil {
		return d.Rule{}, invalidRule()
	}

	var updated d.Rule
	err := s.opts.UoW.Within(ctx, func(txctx context.Context) error {
		set, err := s.opts.Repositories.RuleSets.GetRuleSetForUpdate(txctx, p.TenantID, ruleSetID)
		if err != nil {
			return err
		}
		if set.State != d.RuleSetStateDraft {
			return invalidState()
		}

		rule, err := s.opts.Repositories.Rules.GetRuleForUpdate(txctx, p.TenantID, ruleSetID, ruleID)
		if err != nil {
			return err
		}
		if rule.Revision != req.ExpectedRevision {
			return versionConflict(rule.Revision)
		}

		if req.Kind != rule.Kind || req.Expression != rule.Expression {
			exists, err := s.opts.Repositories.Rules.ExistsRule(txctx, p.TenantID, ruleSetID, req.Kind, req.Expression, ruleID)
			if err != nil {
				return err
			}
			if exists {
				return ruleDuplicate()
			}
		}

		if req.Kind == d.RuleKindAIPattern && req.Enabled && (!rule.Enabled || rule.Kind != d.RuleKindAIPattern) {
			_, patternCount, err := s.opts.Repositories.Rules.CountRulesBySet(txctx, p.TenantID, ruleSetID)
			if err != nil {
				return err
			}
			if patternCount >= d.MaxPatterns {
				return ruleLimitExceeded()
			}
		}

		now := time.Now().UTC()
		rule.Kind = req.Kind
		rule.Expression = req.Expression
		rule.Severity = req.Severity
		rule.Category = req.Category
		rule.Advice = req.Advice
		rule.Weight = req.Weight
		rule.Enabled = req.Enabled
		rule.Revision++
		rule.UpdatedAt = now

		if err := s.opts.Repositories.Rules.UpdateRule(txctx, rule, req.ExpectedRevision); err != nil {
			return err
		}

		set.Revision++
		set.UpdatedAt = now
		// RuleSet updated in transaction

		updated = rule
		return nil
	})

	return updated, err
}

func (s *Service) DeleteRule(ctx context.Context, p scope.Principal, ruleSetID, ruleID string, expectedRevision int64) (d.Deleted, error) {
	if !scope.ValidID(ruleSetID) || !scope.ValidID(ruleID) {
		return d.Deleted{}, bad("id")
	}
	if expectedRevision < 1 {
		return d.Deleted{}, bad("expected_revision")
	}

	var deleted d.Deleted
	err := s.opts.UoW.Within(ctx, func(txctx context.Context) error {
		set, err := s.opts.Repositories.RuleSets.GetRuleSetForUpdate(txctx, p.TenantID, ruleSetID)
		if err != nil {
			return err
		}
		if set.State != d.RuleSetStateDraft {
			return invalidState()
		}

		rule, err := s.opts.Repositories.Rules.GetRuleForUpdate(txctx, p.TenantID, ruleSetID, ruleID)
		if err != nil {
			return err
		}
		if rule.Revision != expectedRevision {
			return versionConflict(rule.Revision)
		}

		if err := s.opts.Repositories.Rules.DeleteRule(txctx, p.TenantID, ruleSetID, ruleID, expectedRevision); err != nil {
			return err
		}

		set.Revision++
		set.UpdatedAt = time.Now().UTC()

		deleted = d.Deleted{
			ID:       ruleID,
			Revision: expectedRevision + 1,
			Deleted:  true,
		}
		return nil
	})

	return deleted, err
}

func (s *Service) PublishRuleSet(ctx context.Context, p scope.Principal, ruleSetID string, req d.Expected) (d.RuleSet, error) {
	if !scope.ValidID(ruleSetID) {
		return d.RuleSet{}, bad("rule_set_id")
	}
	if req.ExpectedRevision < 1 {
		return d.RuleSet{}, bad("expected_revision")
	}

	// 1. Read-only validation & compile outside lock
	draftSet, err := s.opts.Repositories.RuleSets.GetRuleSet(ctx, p.TenantID, ruleSetID)
	if err != nil {
		return d.RuleSet{}, err
	}
	if draftSet.State != d.RuleSetStateDraft {
		return d.RuleSet{}, invalidState()
	}
	if draftSet.Revision != req.ExpectedRevision {
		return d.RuleSet{}, versionConflict(draftSet.Revision)
	}

	rules, err := s.opts.Repositories.Rules.GetAllRulesBySet(ctx, p.TenantID, ruleSetID)
	if err != nil {
		return d.RuleSet{}, err
	}

	matcher, err := d.NewMatcher(rules)
	if err != nil {
		return d.RuleSet{}, invalidRule()
	}

	checksum := d.CalculateRuleSetChecksum(rules)

	// 2. Atomic publish inside transaction with advisory lock
	var published d.RuleSet
	err = s.opts.UoW.Within(ctx, func(txctx context.Context) error {
		res, err := s.opts.Repositories.RuleSets.PublishRuleSet(txctx, p.TenantID, ruleSetID, checksum, req.ExpectedRevision)
		if err != nil {
			return err
		}
		published = res

		if s.opts.Outbox != nil {
			_ = s.opts.Outbox.Publish(txctx, p.TenantID, "compliance", "compliance.rule_set_published", published.ID, published.Revision, map[string]any{
				"rule_set_id": published.ID,
				"version_no":  published.VersionNo,
				"checksum":    checksum,
			})
		}
		return nil
	})
	if err != nil {
		return d.RuleSet{}, err
	}

	// 3. Update in-memory snapshot cache
	if s.opts.SnapshotCache != nil {
		s.opts.SnapshotCache.Put(p.TenantID, checksum, matcher)
	}

	return published, nil
}

func (s *Service) ListRules(ctx context.Context, p scope.Principal, ruleSetID string, query d.ListQuery) (d.Page[d.Rule], error) {
	if !scope.ValidID(ruleSetID) {
		return d.Page[d.Rule]{}, bad("rule_set_id")
	}
	if query.Limit <= 0 || query.Limit > 100 {
		query.Limit = 20
	}

	items, hasMore, err := s.opts.Repositories.Rules.ListRulesBySet(ctx, p.TenantID, ruleSetID, query)
	if err != nil {
		return d.Page[d.Rule]{}, err
	}

	return d.Page[d.Rule]{
		Items:   items,
		HasMore: hasMore,
	}, nil
}
