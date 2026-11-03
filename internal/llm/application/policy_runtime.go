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

// RecordPolicyVersionReference lets an owning domain pin a frozen policy
// version inside its own creation transaction so catalog deletion refusal
// stays entirely within LLM-owned tables.
func (s *Catalog) RecordPolicyVersionReference(ctx context.Context, tenantID, versionID, referenceKind, referenceID string) error {
	return s.opts.Repository.RecordPolicyVersionReference(ctx, tenantID, versionID, referenceKind, referenceID)
}

// ReleasePolicyVersionReferences drops ledger pins for entities deleted by
// their owning domain, inside that domain's own transaction.
func (s *Catalog) ReleasePolicyVersionReferences(ctx context.Context, tenantID, referenceKind string, referenceIDs []string) error {
	return s.opts.Repository.ReleasePolicyVersionReferences(ctx, tenantID, referenceKind, referenceIDs)
}
