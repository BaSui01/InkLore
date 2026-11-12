package infrastructure

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	d "InkLore/internal/llm/domain"
	"InkLore/internal/platform/scope"
	"InkLore/internal/platform/uow"
)

// GatewayRuntimeRepository supplies trusted catalog metadata and persists call
// identities. It never reads Agent, Creator, or another business domain's tables.
type GatewayRuntimeRepository struct{ manager *uow.Manager }

func NewGatewayRuntimeRepository(manager *uow.Manager) (*GatewayRuntimeRepository, error) {
	if manager == nil {
		return nil, errors.New("llm gateway: database required")
	}
	return &GatewayRuntimeRepository{manager: manager}, nil
}

func (r *GatewayRuntimeRepository) Plan(ctx context.Context, req d.TextGenerationRequest) (d.GatewayPlan, error) {
	var plan d.GatewayPlan
	if !scope.ValidID(req.TenantID) || !scope.ValidID(req.ModelID) || !scope.ValidID(req.PolicyVersionID) || strings.TrimSpace(req.ExecutionKey) == "" || len(req.ExecutionKey) > 256 {
		return plan, d.ErrInvalid
	}
	db, err := r.manager.DB(ctx)
	if err != nil {
		return plan, err
	}
	var row struct {
		TimeoutSeconds, MaxOutputTokens int
		Temperature                     float64
		Tools, Structured               bool
	}
	q := db.Raw(`SELECT v.timeout_seconds,LEAST(v.max_output_tokens,m.max_output_tokens) AS max_output_tokens,v.temperature,
		'tools'=ANY(m.capabilities) AS tools,'structured'=ANY(m.capabilities) AS structured
		FROM llm_task_policy_versions v
		JOIN llm_task_policies p ON p.tenant_id=v.tenant_id AND p.id=v.policy_id
		JOIN llm_models m ON m.tenant_id=v.tenant_id AND (m.id=v.primary_model_id OR m.id=ANY(v.fallback_model_ids))
		JOIN llm_providers pr ON pr.tenant_id=m.tenant_id AND pr.id=m.provider_id
		WHERE v.tenant_id=? AND v.id=? AND m.id=? AND p.task_key=?
		AND p.enabled AND p.deleted_at IS NULL AND m.enabled AND m.deleted_at IS NULL
		AND pr.enabled AND pr.deleted_at IS NULL AND v.required_capabilities <@ m.capabilities
		AND 'text'=ANY(m.capabilities)`, req.TenantID, req.PolicyVersionID, req.ModelID, req.TaskKey).Scan(&row)
	if q.Error != nil {
		return plan, q.Error
	}
	if q.RowsAffected != 1 {
		return plan, d.ErrPolicy
	}
	if req.MaxTokens < 0 || req.MaxTokens > row.MaxOutputTokens || (len(req.Tools) > 0 && !row.Tools) || (req.ResponseFormat != nil && req.ResponseFormat.Type != d.ResponseFormatText && !row.Structured) {
		return plan, d.ErrCapability
	}
	var candidates []struct {
		d.Selection
		SecretVersion string
	}
	q = db.Raw(`SELECT c.id AS channel_id,k.id AS key_id,cm.priority AS channel_priority,cm.weight AS channel_weight,
		k.weight AS key_weight,c.rpm AS channel_rpm,k.rpm AS key_rpm,c.concurrency AS channel_concurrency,k.concurrency AS key_concurrency,k.secret_version
		FROM llm_channel_models cm
		JOIN llm_channels c ON c.tenant_id=cm.tenant_id AND c.id=cm.channel_id
		JOIN llm_models m ON m.tenant_id=cm.tenant_id AND m.id=cm.model_id
		JOIN llm_providers pr ON pr.tenant_id=c.tenant_id AND pr.id=c.provider_id
		JOIN llm_keys k ON k.tenant_id=c.tenant_id AND k.channel_id=c.id
		WHERE cm.tenant_id=? AND cm.model_id=? AND cm.enabled AND cm.deleted_at IS NULL
		AND (m.provider_id=c.provider_id OR btrim(cm.upstream_model)<>'')
		AND c.enabled AND c.deleted_at IS NULL AND pr.enabled AND pr.deleted_at IS NULL
		AND k.enabled AND k.deleted_at IS NULL AND k.health<>'invalid' AND k.fingerprint IS NOT NULL
		ORDER BY cm.priority,c.id,k.id LIMIT 1001`, req.TenantID, req.ModelID).Scan(&candidates)
	if q.Error != nil {
		return plan, q.Error
	}
	if len(candidates) > 1000 {
		return plan, d.ErrPolicy
	}
	plan.TimeoutSeconds, plan.MaxOutputTokens, plan.Temperature = row.TimeoutSeconds, row.MaxOutputTokens, row.Temperature
	plan.KeyVersions = make(map[string]string, len(candidates))
	for _, candidate := range candidates {
		plan.Candidates = append(plan.Candidates, candidate.Selection)
		plan.KeyVersions[candidate.KeyID] = candidate.SecretVersion
	}
	return plan, nil
}

func (r *GatewayRuntimeRepository) BindInvocation(ctx context.Context, req d.TextGenerationRequest) (d.GatewayInvocation, error) {
	var out d.GatewayInvocation
	// Include trusted fields explicitly: they are deliberately absent from the
	// provider JSON payload, but must be part of replay identity.
	raw, err := json.Marshal(struct {
		Tenant, Policy, Model string
		Request               d.TextGenerationRequest
	}{req.TenantID, req.PolicyVersionID, req.ModelID, req})
	if err != nil {
		return out, d.ErrInvalid
	}
	digest := sha256.Sum256(raw)
	hash := hex.EncodeToString(digest[:])
	err = r.manager.Within(ctx, func(tx context.Context) error {
		db, err := r.manager.Require(tx)
		if err != nil {
			return err
		}
		if err = db.Exec(`INSERT INTO llm_generation_invocations(tenant_id,operation_key,request_hash)
			VALUES(?,?,?) ON CONFLICT(tenant_id,operation_key) DO NOTHING`, req.TenantID, req.ExecutionKey, hash).Error; err != nil {
			return err
		}
		var row struct {
			ID, RequestHash string
			RawResponse     []byte
			UsageRecordID   *string
		}
		q := db.Raw(`SELECT id,request_hash,response::text AS raw_response,usage_record_id FROM llm_generation_invocations
			WHERE tenant_id=? AND operation_key=? FOR UPDATE`, req.TenantID, req.ExecutionKey).Scan(&row)
		if q.Error != nil {
			return q.Error
		}
		if q.RowsAffected != 1 {
			return d.ErrNotFound
		}
		if row.RequestHash != hash {
			return d.ErrConflict
		}
		out.ID = row.ID
		if len(row.RawResponse) > 0 {
			var response d.TextGenerationResponse
			if err = json.Unmarshal(row.RawResponse, &response); err != nil {
				return err
			}
			var usage string
			q = db.Raw(`SELECT row_to_json(u)::text FROM llm_usage_records u WHERE tenant_id=? AND id=? AND invocation_id=?`, req.TenantID, row.UsageRecordID, row.ID).Scan(&usage)
			if q.Error != nil {
				return q.Error
			}
			if q.RowsAffected != 1 {
				return d.ErrBudgetState
			}
			if err = json.Unmarshal([]byte(usage), &response.Receipt); err != nil {
				return err
			}
			out.Response = &response
		} else {
			// Recovery can settle an interrupted dispatch without a reusable model
			// response. Return its durable accounting receipt without dispatching
			// the operation again or treating its absent result as success.
			var usage string
			q = db.Raw(`SELECT row_to_json(u)::text FROM llm_usage_records u WHERE tenant_id=? AND invocation_id=? AND attempt=1`, req.TenantID, row.ID).Scan(&usage)
			if q.Error != nil {
				return q.Error
			}
			if q.RowsAffected == 1 {
				if err = json.Unmarshal([]byte(usage), &out.Receipt); err != nil {
					return err
				}
			}
		}
		return nil
	})
	return out, err
}

func (r *GatewayRuntimeRepository) SaveResult(ctx context.Context, tenant, invocation string, response d.TextGenerationResponse) error {
	if response.Receipt == nil || response.Receipt.InvocationID != invocation {
		return d.ErrInvalid
	}
	db, err := r.manager.Require(ctx)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(response)
	if err != nil {
		return err
	}
	q := db.Exec(`UPDATE llm_generation_invocations SET response=?::jsonb,usage_record_id=?,updated_at=clock_timestamp()
		WHERE tenant_id=? AND id=? AND response IS NULL`, string(raw), response.Receipt.ID, tenant, invocation)
	if q.Error != nil {
		return q.Error
	}
	if q.RowsAffected != 1 {
		return d.ErrConflict
	}
	return nil
}

func (r *GatewayRuntimeRepository) AdmissionRecovery(ctx context.Context, tenant string) (d.RecoverySnapshot, error) {
	var snapshot d.RecoverySnapshot
	db, err := r.manager.DB(ctx)
	if err != nil {
		return snapshot, err
	}
	var row struct{ Deadline *time.Time }
	// A lost Redis instance has also lost its 60-second RPM window. Completed
	// calls still participate in recovery until that window has elapsed.
	err = db.Raw(`SELECT max(GREATEST(dispatched_at+interval '60 seconds',
		CASE WHEN status='dispatched' THEN lease_until ELSE dispatched_at END)) AS deadline
		FROM llm_reservations WHERE tenant_id=? AND dispatched_at IS NOT NULL
		AND ((status='dispatched' AND lease_until>clock_timestamp()) OR dispatched_at+interval '60 seconds'>clock_timestamp())`, tenant).Scan(&row).Error
	if err != nil {
		return snapshot, err
	}
	if row.Deadline != nil {
		snapshot.Inflight = true
		snapshot.LatestDeadline = *row.Deadline
	}
	return snapshot, nil
}

func (r *GatewayRuntimeRepository) InvalidateCredential(ctx context.Context, tenant, key, version string) error {
	return r.manager.Within(ctx, func(tx context.Context) error {
		db, err := r.manager.Require(tx)
		if err != nil {
			return err
		}
		return db.Exec(`UPDATE llm_keys SET health='invalid',revision=revision+1,updated_at=clock_timestamp()
			WHERE tenant_id=? AND id=? AND secret_version=? AND health<>'invalid' AND deleted_at IS NULL`, tenant, key, version).Error
	})
}

// ResolveDispatch cannot pick another channel/key or fall back to process
// credentials. Both the PG dispatch and current secret version must match.
func (r *GatewayRuntimeRepository) ResolveDispatch(ctx context.Context, req d.TextGenerationRequest) (TenantChannelTarget, error) {
	if req.Dispatch == nil {
		return TenantChannelTarget{}, d.ErrPolicy
	}
	l := req.Dispatch.Lease
	db, err := r.manager.DB(ctx)
	if err != nil {
		return TenantChannelTarget{}, err
	}
	var row tenantChannelRow
	q := db.Raw(`SELECT c.base_url,c.provider_id,cm.upstream_model,m.upstream_name AS model_name,pr.protocol,k.secret_kind,k.secret_locator,k.secret_version
		FROM llm_reservations r
		JOIN llm_channel_models cm ON cm.tenant_id=r.tenant_id AND cm.channel_id=r.channel_id AND cm.model_id=r.model_id
		JOIN llm_channels c ON c.tenant_id=cm.tenant_id AND c.id=cm.channel_id
		JOIN llm_models m ON m.tenant_id=cm.tenant_id AND m.id=cm.model_id
		JOIN llm_providers pr ON pr.tenant_id=c.tenant_id AND pr.id=c.provider_id
		JOIN llm_keys k ON k.tenant_id=r.tenant_id AND k.id=r.key_id AND k.channel_id=c.id
		WHERE r.tenant_id=? AND r.id=? AND r.invocation_id=? AND r.model_id=? AND r.policy_version_id=?
		AND r.status='dispatched' AND r.lease_until>clock_timestamp() AND r.channel_id=? AND r.key_id=? AND k.secret_version=?
		AND cm.enabled AND cm.deleted_at IS NULL AND c.enabled AND c.deleted_at IS NULL
		AND m.enabled AND m.deleted_at IS NULL AND pr.enabled AND pr.deleted_at IS NULL
		AND k.enabled AND k.deleted_at IS NULL AND k.health<>'invalid'`, req.TenantID, req.Dispatch.ReservationID, l.InvocationID, req.ModelID, req.PolicyVersionID, l.ChannelID, l.KeyID, l.KeyVersion).Scan(&row)
	if q.Error != nil {
		return TenantChannelTarget{}, q.Error
	}
	if q.RowsAffected != 1 {
		return TenantChannelTarget{}, d.ErrPolicy
	}
	return resolveTenantChannelRow(row)
}

var _ d.GatewayRuntimeRepository = (*GatewayRuntimeRepository)(nil)
var _ d.RecoveryProbePort = (*GatewayRuntimeRepository)(nil)
var _ d.KeyHealthPort = (*GatewayRuntimeRepository)(nil)
