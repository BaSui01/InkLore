package application

import (
	"context"
	"encoding/json"

	d "InkLore/internal/llm/domain"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/scope"
)

func creatorModelProjection(raw json.RawMessage) (json.RawMessage, error) {
	// Model's explicit DTO contains public model metadata only; later storage
	// columns cannot accidentally expose routing or credential configuration.
	var model d.Model
	if err := json.Unmarshal(raw, &model); err != nil {
		return nil, err
	}
	return json.Marshal(model)
}

func (s *Catalog) ListCreatorPolicies(ctx context.Context, p identity.Principal, q d.CreatorPolicyQuery) ([]d.CreatorPolicy, error) {
	if p.Audience != "creator" {
		return nil, public(d.ErrInvalid)
	}
	p, err := s.Authorize(ctx, p, false)
	if err != nil {
		return nil, err
	}
	if q.Limit < 1 || q.Limit > 101 || len(q.AllowedTaskKinds) == 0 || len(q.AllowedTaskKinds) > 16 || (q.AfterID != "" && !scope.ValidID(q.AfterID)) {
		return nil, invalid()
	}
	repo, ok := s.opts.Repository.(d.CreatorPolicyRepository)
	if !ok {
		return nil, public(d.ErrPolicy)
	}
	items, err := repo.ListCreatorPolicies(ctx, p.TenantID, q)
	return items, public(err)
}

// LockCreatorPolicy checks the selected version against the current enabled
// head inside CreateRun's transaction. Frozen legacy runs use ReadFrozenPolicy.
func (s *Catalog) LockCreatorPolicy(ctx context.Context, p identity.Principal, versionID, taskKind string) (d.CreatorPolicy, error) {
	if p.Audience != "creator" || !scope.ValidID(versionID) || taskKind == "" {
		return d.CreatorPolicy{}, public(d.ErrInvalid)
	}
	p, err := s.Authorize(ctx, p, false)
	if err != nil {
		return d.CreatorPolicy{}, err
	}
	repo, ok := s.opts.Repository.(d.CreatorPolicyRepository)
	if !ok {
		return d.CreatorPolicy{}, public(d.ErrPolicy)
	}
	policy, err := repo.LockCreatorPolicy(ctx, p.TenantID, versionID, taskKind)
	return policy, public(err)
}
