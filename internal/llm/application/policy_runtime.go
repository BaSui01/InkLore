package application

import (
	"context"

	d "InkLore/internal/llm/domain"
)

// These queries are the LLM domain's only public runtime policy boundary.
// Existing runs read their immutable version rather than the mutable policy head.
func (s *Catalog) CurrentPolicyVersion(ctx context.Context, tenantID, taskKey string) (string, error) {
	return s.opts.Repository.CurrentPolicyVersion(ctx, tenantID, taskKey)
}

func (s *Catalog) ReadFrozenPolicy(ctx context.Context, tenantID, versionID string) (d.FrozenPolicy, error) {
	return s.opts.Repository.ReadFrozenPolicy(ctx, tenantID, versionID)
}

func (s *Catalog) ResolveEnabledModel(ctx context.Context, tenantID, model string) (d.EnabledModel, error) {
	return s.opts.Repository.ResolveEnabledModel(ctx, tenantID, model)
}

func (s *Catalog) CurrentPolicySnapshot(ctx context.Context, tenantID, taskKey string) (d.FrozenPolicy, error) {
	return s.opts.Repository.CurrentPolicySnapshot(ctx, tenantID, taskKey)
}
