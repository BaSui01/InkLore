package main

import (
	"context"
	"errors"
	"fmt"

	llmdomain "InkLore/internal/llm/domain"
	llminfra "InkLore/internal/llm/infrastructure"
	"InkLore/internal/platform/uow"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// checkFrozenRetention executes only inside the explicitly isolated CI database.
func checkFrozenRetention(ctx context.Context, db *gorm.DB, manager *uow.Manager, store *llminfra.Store, a fixture) error {
	used, err := store.InUse(ctx, a.tenant, llmdomain.Policies, a.policy)
	if err != nil || !used {
		return fmt.Errorf("queued Agent Run did not pin its policy: %v", err)
	}

	// Exercise the real persistence deletion path in a transaction. The guard
	// must reject it without modifying current or historical policy versions.
	err = manager.Within(ctx, func(tx context.Context) error {
		return store.Delete(tx, a.tenant, llmdomain.Policies, a.policy, 1)
	})
	if !errors.Is(err, llmdomain.ErrInUse) {
		return fmt.Errorf("referenced policy deletion was not rejected")
	}
	frozen, err := store.ReadFrozenPolicy(ctx, a.tenant, a.version1)
	if err != nil || frozen.VersionID != a.version1 || frozen.ModelID != a.model {
		return fmt.Errorf("failed delete changed an immutable policy version")
	}

	// An unrelated tenant must not inspect or delete another tenant's policy.
	if _, err := store.InUse(ctx, uuid.NewString(), llmdomain.Policies, a.policy); !errors.Is(err, llmdomain.ErrNotFound) {
		return fmt.Errorf("cross-tenant policy reference probe was permitted")
	}

	// Two enabled models may legitimately share an upstream name if they are
	// owned by different providers in the same tenant. Only the frozen ModelID
	// is unambiguous; resolving the name again must fail closed.
	providerID, modelID := uuid.NewString(), uuid.NewString()
	if err := db.WithContext(ctx).Exec(
		"INSERT INTO llm_providers(id,tenant_id,code,title,protocol,enabled) VALUES (?::uuid,?::uuid,'ci_duplicate_provider','CI Duplicate','openai',TRUE)",
		providerID, a.tenant,
	).Error; err != nil {
		return fmt.Errorf("seed duplicate provider: %w", err)
	}
	if err := db.WithContext(ctx).Exec(
		"INSERT INTO llm_models(id,tenant_id,provider_id,upstream_name,title,capabilities,context_window,max_output_tokens,enabled) VALUES (?::uuid,?::uuid,?::uuid,'same-upstream-name','CI Duplicate Model',ARRAY['text']::text[],4096,512,TRUE)",
		modelID, a.tenant, providerID,
	).Error; err != nil {
		return fmt.Errorf("seed ambiguous model: %w", err)
	}
	if _, err := store.ResolveEnabledModel(ctx, a.tenant, "same-upstream-name"); err == nil {
		return fmt.Errorf("ambiguous upstream name incorrectly selected one model")
	}
	frozen, err = store.ReadFrozenPolicy(ctx, a.tenant, a.version1)
	if err != nil || frozen.ModelID != a.model {
		return fmt.Errorf("ambiguous upstream name changed the frozen model binding")
	}
	resolver, err := llminfra.NewTenantChannelResolver(manager)
	if err != nil {
		return err
	}
	target, err := resolver(ctx, a.tenant, frozen.ModelID)
	if err != nil || target.UpstreamModel != a.routeAlias || target.APIKey != a.keyValue {
		return fmt.Errorf("frozen model ID failed to select its own tenant channel")
	}
	return nil
}
