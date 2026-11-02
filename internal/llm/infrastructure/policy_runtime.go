package infrastructure

import (
	"context"

	d "InkLore/internal/llm/domain"
)

// CurrentPolicyVersion is only for new work. A running Agent must use ReadFrozenPolicy.
func (s *Store) CurrentPolicyVersion(ctx context.Context, tenantID, taskKey string) (string, error) {
	db, err := s.manager.DB(ctx)
	if err != nil {
		return "", err
	}
	var versionID string
	err = db.Raw(`SELECT current_version_id::text
		FROM llm_task_policies
		WHERE tenant_id=? AND task_key=? AND enabled=TRUE
		  AND deleted_at IS NULL AND current_version_id IS NOT NULL`, tenantID, taskKey).Scan(&versionID).Error
	return versionID, err
}

// ReadFrozenPolicy reads the immutable version, independent of which version is current.
func (s *Store) ReadFrozenPolicy(ctx context.Context, tenantID, versionID string) (d.FrozenPolicy, error) {
	db, err := s.manager.DB(ctx)
	if err != nil {
		return d.FrozenPolicy{}, err
	}
	var policy d.FrozenPolicy
	result := db.Raw(`SELECT v.id::text AS version_id, p.task_key,
		       m.id::text AS model_id, m.upstream_name AS model_name,
		       v.max_output_tokens, v.temperature
		FROM llm_task_policy_versions v
		JOIN llm_task_policies p ON p.tenant_id=v.tenant_id AND p.id=v.policy_id
		JOIN llm_models m ON m.tenant_id=v.tenant_id AND m.id=v.primary_model_id
		JOIN llm_providers pr ON pr.tenant_id=m.tenant_id AND pr.id=m.provider_id
		WHERE v.tenant_id=? AND v.id=? AND m.enabled=TRUE AND m.deleted_at IS NULL
		  AND pr.enabled=TRUE AND pr.deleted_at IS NULL`, tenantID, versionID).Scan(&policy)
	if result.Error != nil {
		return d.FrozenPolicy{}, result.Error
	}
	if result.RowsAffected != 1 || policy.VersionID == "" {
		return d.FrozenPolicy{}, d.ErrNotFound
	}
	return policy, nil
}

// ResolveEnabledModel checks explicit Creator model selections within the same tenant.
func (s *Store) ResolveEnabledModel(ctx context.Context, tenantID, model string) (d.EnabledModel, error) {
	db, err := s.manager.DB(ctx)
	if err != nil {
		return d.EnabledModel{}, err
	}
	var models []d.EnabledModel
	err = db.Raw(`SELECT m.id::text AS id, m.upstream_name AS name
		FROM llm_models m
		JOIN llm_providers pr ON pr.tenant_id=m.tenant_id AND pr.id=m.provider_id
		WHERE m.tenant_id=? AND (m.id::text=? OR m.upstream_name=?)
		  AND m.enabled=TRUE AND m.deleted_at IS NULL
		  AND pr.enabled=TRUE AND pr.deleted_at IS NULL
		ORDER BY m.id LIMIT 2`, tenantID, model, model).Scan(&models).Error
	if err != nil {
		return d.EnabledModel{}, err
	}
	if len(models) != 1 {
		return d.EnabledModel{}, d.ErrNotFound
	}
	return models[0], nil
}

// CurrentPolicySnapshot provides one coherent policy version for new novel guides.
func (s *Store) CurrentPolicySnapshot(ctx context.Context, tenantID, taskKey string) (d.FrozenPolicy, error) {
	db, err := s.manager.DB(ctx)
	if err != nil {
		return d.FrozenPolicy{}, err
	}
	var policy d.FrozenPolicy
	result := db.Raw(`SELECT v.id::text AS version_id, p.task_key,
		       m.id::text AS model_id, m.upstream_name AS model_name,
		       v.max_output_tokens, v.temperature
		FROM llm_task_policies p
		JOIN llm_task_policy_versions v ON v.tenant_id=p.tenant_id AND v.id=p.current_version_id
		JOIN llm_models m ON m.tenant_id=v.tenant_id AND m.id=v.primary_model_id
		JOIN llm_providers pr ON pr.tenant_id=m.tenant_id AND pr.id=m.provider_id
		WHERE p.tenant_id=? AND p.task_key=? AND p.enabled=TRUE AND p.deleted_at IS NULL
		  AND m.enabled=TRUE AND m.deleted_at IS NULL AND pr.enabled=TRUE
		  AND pr.deleted_at IS NULL`, tenantID, taskKey).Scan(&policy)
	if result.Error != nil {
		return d.FrozenPolicy{}, result.Error
	}
	if result.RowsAffected != 1 || policy.VersionID == "" {
		return d.FrozenPolicy{}, d.ErrNotFound
	}
	return policy, nil
}
