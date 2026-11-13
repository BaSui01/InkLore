package infrastructure

import (
	"context"
	"encoding/json"
	"strings"

	d "InkLore/internal/llm/domain"
)

const creatorPolicySelect = `SELECT jsonb_build_object(
 'policy_id',p.id,'policy_version_id',v.id,'task_key',p.task_key,'title',p.title,
 'version',v.version,'task_kinds',v.task_kinds,'max_output_tokens',v.max_output_tokens,
 'timeout_seconds',v.timeout_seconds)::text AS data
 FROM llm_task_policies p
 JOIN llm_task_policy_versions v ON v.tenant_id=p.tenant_id AND v.id=p.current_version_id
 JOIN llm_models m ON m.tenant_id=v.tenant_id AND m.id=v.primary_model_id
 JOIN llm_providers pr ON pr.tenant_id=m.tenant_id AND pr.id=m.provider_id
 WHERE p.tenant_id=? AND p.enabled=TRUE AND p.deleted_at IS NULL
 AND m.enabled=TRUE AND m.deleted_at IS NULL AND pr.enabled=TRUE AND pr.deleted_at IS NULL
 AND cardinality(v.task_kinds)>0 AND v.output_mode IN ('text','structured')
 AND 'text'=ANY(m.capabilities) AND v.output_mode=ANY(m.capabilities)
 AND v.required_capabilities <@ m.capabilities
 AND v.max_output_tokens<=m.max_output_tokens AND v.max_output_tokens<m.context_window`

func (s *Store) ListCreatorPolicies(ctx context.Context, tenantID string, q d.CreatorPolicyQuery) ([]d.CreatorPolicy, error) {
	db, err := s.manager.DB(ctx)
	if err != nil {
		return nil, err
	}
	kinds, err := json.Marshal(q.AllowedTaskKinds)
	if err != nil {
		return nil, err
	}
	query := creatorPolicySelect + ` AND v.task_kinds && ARRAY(SELECT jsonb_array_elements_text(?::jsonb))`
	args := []any{tenantID, string(kinds)}
	if q.TaskKind != "" {
		query += ` AND ?=ANY(v.task_kinds)`
		args = append(args, q.TaskKind)
	}
	if q.AfterID != "" {
		query += ` AND p.id>?::uuid`
		args = append(args, q.AfterID)
	}
	query += ` ORDER BY p.id LIMIT ?`
	args = append(args, q.Limit)
	var rows []struct{ Data string }
	if err := db.Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, dbError(err)
	}
	items := make([]d.CreatorPolicy, 0, len(rows))
	allowed := make(map[string]bool, len(q.AllowedTaskKinds))
	for _, kind := range q.AllowedTaskKinds {
		allowed[kind] = true
	}
	for _, row := range rows {
		var item d.CreatorPolicy
		if err := json.Unmarshal([]byte(row.Data), &item); err != nil {
			return nil, err
		}
		kinds := make([]string, 0, len(item.TaskKinds))
		for _, kind := range item.TaskKinds {
			if allowed[kind] {
				kinds = append(kinds, kind)
			}
		}
		item.TaskKinds = kinds
		items = append(items, item)
	}
	return items, nil
}

func (s *Store) LockCreatorPolicy(ctx context.Context, tenantID, versionID, taskKind string) (d.CreatorPolicy, error) {
	db, err := s.manager.Require(ctx)
	if err != nil {
		return d.CreatorPolicy{}, err
	}
	var row struct{ Data string }
	result := db.Raw(creatorPolicySelect+` AND v.id=?::uuid AND ?=ANY(v.task_kinds) FOR SHARE OF p,v,m,pr`, tenantID, versionID, taskKind).Scan(&row)
	if result.Error != nil {
		return d.CreatorPolicy{}, dbError(result.Error)
	}
	if result.RowsAffected != 1 || strings.TrimSpace(row.Data) == "" {
		return d.CreatorPolicy{}, d.ErrNotFound
	}
	var policy d.CreatorPolicy
	if err := json.Unmarshal([]byte(row.Data), &policy); err != nil {
		return d.CreatorPolicy{}, err
	}
	return policy, nil
}
