package main

import (
	"context"
	"fmt"
	"os"

	llminfra "InkLore/internal/llm/infrastructure"

	"gorm.io/gorm"
)

func checkTenantRoutes(
	ctx context.Context, db *gorm.DB, resolver llminfra.TenantChannelResolver,
	store *llminfra.Store, a, b fixture,
) error {
	for _, f := range []fixture{a, b} {
		got, err := resolver(ctx, f.tenant, f.model)
		if err != nil {
			return fmt.Errorf("valid tenant channel rejected: %w", err)
		}
		if err := require(
			got.APIKey == f.keyValue && got.UpstreamModel == f.routeAlias &&
				got.Protocol == "openai" && got.BaseURL == "https://ci-"+f.tag+".invalid/v1",
			"resolved route has incorrect secret, provider, model or tenant",
		); err != nil {
			return err
		}
	}
	if _, err := resolver(ctx, a.tenant, b.model); err == nil {
		return fmt.Errorf("cross-tenant model was resolved")
	}
	if _, err := resolver(ctx, b.tenant, a.model); err == nil {
		return fmt.Errorf("reverse cross-tenant model was resolved")
	}
	oldA, err := store.CurrentPolicyVersion(ctx, a.tenant, "draft")
	if err != nil || oldA != a.version1 {
		return fmt.Errorf("tenant A enabled policy did not resolve its initial version")
	}
	oldB, err := store.CurrentPolicyVersion(ctx, b.tenant, "draft")
	if err != nil || oldB != b.version1 {
		return fmt.Errorf("tenant B enabled policy did not resolve its initial version")
	}
	frozenA, err := store.ReadFrozenPolicy(ctx, a.tenant, a.version1)
	if err != nil || frozenA.ModelID != a.model || frozenA.VersionID != a.version1 ||
		frozenA.MaxOutputTokens != 128 || frozenA.Temperature != 0.2 {
		return fmt.Errorf("frozen tenant A policy is incorrect: %v", err)
	}
	if _, err := store.ReadFrozenPolicy(ctx, a.tenant, b.version1); err == nil {
		return fmt.Errorf("cross-tenant frozen policy leaked")
	}
	if _, err := store.ReadFrozenPolicy(ctx, b.tenant, a.version1); err == nil {
		return fmt.Errorf("reverse cross-tenant frozen policy leaked")
	}
	if _, err := store.ReadFrozenPolicy(ctx, a.tenant, "not-a-uuid"); err == nil {
		return fmt.Errorf("malformed frozen policy version accepted")
	}

	// State changes occur only in the disposable inklore_ci DB. Restore after each case.
	type blocked struct {
		table string
		where string
		args  []any
	}
	cases := []blocked{
		{"llm_channel_models", "tenant_id=? AND channel_id=? AND model_id=?", []any{a.tenant, a.channel, a.model}},
		{"llm_channels", "tenant_id=? AND id=?", []any{a.tenant, a.channel}},
		{"llm_keys", "tenant_id=? AND id=?", []any{a.tenant, a.key}},
		{"llm_providers", "tenant_id=? AND id=?", []any{a.tenant, a.provider}},
		{"llm_models", "tenant_id=? AND id=?", []any{a.tenant, a.model}},
	}
	for _, item := range cases {
		query := "UPDATE " + item.table + " SET enabled=? WHERE " + item.where
		if err := db.WithContext(ctx).Exec(query, append([]any{false}, item.args...)...).Error; err != nil {
			return err
		}
		_, routeErr := resolver(ctx, a.tenant, a.model)
		restoreErr := db.WithContext(ctx).Exec(query, append([]any{true}, item.args...)...).Error
		if restoreErr != nil {
			return restoreErr
		}
		if routeErr == nil {
			return fmt.Errorf("disabled %s still permitted a tenant route", item.table)
		}
	}
	if err := os.Unsetenv(a.keyEnv); err != nil {
		return err
	}
	_, routeErr := resolver(ctx, a.tenant, a.model)
	if err := os.Setenv(a.keyEnv, a.keyValue); err != nil {
		return err
	}
	if routeErr == nil {
		return fmt.Errorf("missing tenant-scoped key fell back to another credential")
	}
	if err := db.WithContext(ctx).Exec(
		"UPDATE llm_keys SET secret_kind='secret_store' WHERE tenant_id=? AND id=?",
		a.tenant, a.key,
	).Error; err != nil {
		return err
	}
	_, routeErr = resolver(ctx, a.tenant, a.model)
	if err := db.WithContext(ctx).Exec(
		"UPDATE llm_keys SET secret_kind='env' WHERE tenant_id=? AND id=?",
		a.tenant, a.key,
	).Error; err != nil {
		return err
	}
	if routeErr == nil {
		return fmt.Errorf("unsupported secret_store locator unexpectedly resolved")
	}
	// Enabled tenant B is unaffected by all tenant A mutations.
	if _, err := resolver(ctx, b.tenant, b.model); err != nil {
		return fmt.Errorf("tenant B route changed when tenant A was disabled: %w", err)
	}
	return nil
}
