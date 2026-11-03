// Package r1 provides PostgreSQL and deployment adapters for frozen LLM ports.
package infrastructure

import (
	d "InkLore/internal/llm/domain"
	"InkLore/internal/platform/uow"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Store struct{ manager *uow.Manager }

func NewStore(manager *uow.Manager) (*Store, error) {
	if manager == nil {
		return nil, errors.New("llm: UoW required")
	}
	return &Store{manager}, nil
}
func (s *Store) LockCatalog(ctx context.Context, tenant string) error {
	db, err := s.manager.Require(ctx)
	if err != nil {
		return err
	}
	return db.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,0))", "llm.catalog:"+tenant).Error
}
func table(kind d.Kind) string {
	switch kind {
	case d.Providers:
		return "llm_providers"
	case d.Models:
		return "llm_models"
	case d.Channels:
		return "llm_channels"
	case d.Keys:
		return "llm_keys"
	case d.Policies:
		return "llm_task_policies"
	case d.Prices:
		return "llm_pricing_revisions"
	case d.Usages:
		return "llm_usage_records"
	}
	return ""
}
func projection(kind d.Kind) string {
	switch kind {
	case d.Channels:
		return "(to_jsonb(t)-'tenant_id'-'deleted_at') || jsonb_build_object('model_ids', coalesce((SELECT jsonb_agg(m.model_id) FROM llm_channel_models m WHERE m.tenant_id=t.tenant_id AND m.channel_id=t.id AND m.deleted_at IS NULL), '[]'::jsonb), 'models', coalesce((SELECT jsonb_agg(jsonb_build_object('id', m.id, 'channel_id', m.channel_id, 'model_id', m.model_id, 'upstream_model', m.upstream_model, 'priority', m.priority, 'weight', m.weight, 'enabled', m.enabled)) FROM llm_channel_models m WHERE m.tenant_id=t.tenant_id AND m.channel_id=t.id AND m.deleted_at IS NULL), '[]'::jsonb))"
	case d.Keys:
		return "(to_jsonb(t)-'tenant_id'-'deleted_at'-'secret_kind'-'secret_locator'-'secret_version') || jsonb_build_object('secret_ref',jsonb_build_object('kind',t.secret_kind,'locator',t.secret_locator,'version',t.secret_version))"
	case d.Policies:
		return "(to_jsonb(t)-'tenant_id'-'deleted_at') || (to_jsonb(v)-'id'-'tenant_id'-'policy_id'-'created_at'-'updated_at')"
	case d.Usages:
		return "(to_jsonb(t)-'tenant_id'-'actor_user_id'-'secret_version') || jsonb_build_object('adjustment_micros',coalesce((SELECT sum(a.delta_micros) FROM llm_usage_adjustments a WHERE a.tenant_id=t.tenant_id AND a.usage_id=t.id),0))"
	default:
		return "to_jsonb(t)-'tenant_id'-'deleted_at'"
	}
}
func from(kind d.Kind) string {
	value := table(kind) + " t"
	if kind == d.Policies {
		value += " JOIN llm_task_policy_versions v ON v.tenant_id=t.tenant_id AND v.id=t.current_version_id"
	}
	return value
}
func alive(kind d.Kind) string {
	if kind == d.Prices || kind == d.Usages {
		return ""
	}
	return " AND t.deleted_at IS NULL"
}
func normalize(kind d.Kind, raw []byte) (json.RawMessage, error) {
	var obj map[string]any
	if json.Unmarshal(raw, &obj) != nil {
		return nil, d.ErrInvalid
	}
	for _, key := range []string{"created_at", "updated_at", "effective_at"} {
		if value, ok := obj[key].(string); ok {
			t, err := time.Parse(time.RFC3339Nano, value)
			if err != nil {
				return nil, d.ErrInvalid
			}
			obj[key] = t.UTC().Format(time.RFC3339Nano)
		}
	}
	if kind == d.Keys {
		if ref, ok := obj["secret_ref"].(map[string]any); ok {
			if loc, ok := ref["locator"].(string); ok {
				ref["locator"] = MaskSecret(loc)
			}
		}
	}
	return json.Marshal(obj)
}
func (s *Store) Get(ctx context.Context, tenant string, kind d.Kind, id string) (json.RawMessage, error) {
	if table(kind) == "" {
		return nil, d.ErrInvalid
	}
	db, err := s.manager.DB(ctx)
	if err != nil {
		return nil, err
	}
	var raw []byte
	err = db.Raw("SELECT ("+projection(kind)+")::text FROM "+from(kind)+" WHERE t.tenant_id=? AND t.id=?"+alive(kind), tenant, id).Row().Scan(&raw)
	if err != nil {
		return nil, dbError(err)
	}
	return normalize(kind, raw)
}
func (s *Store) List(ctx context.Context, tenant string, kind d.Kind, q d.ListQuery) ([]json.RawMessage, error) {
	if table(kind) == "" || q.Limit < 1 || q.Limit > 101 {
		return nil, d.ErrInvalid
	}
	db, err := s.manager.DB(ctx)
	if err != nil {
		return nil, err
	}
	column := "updated_at"
	if kind == d.Usages || kind == d.Prices {
		column = "created_at"
	}
	query := "SELECT (" + projection(kind) + ")::text FROM " + from(kind) + " WHERE t.tenant_id=?" + alive(kind)
	args := []any{tenant}
	if q.BeforeID != "" {
		query += " AND (t." + column + ",t.id)<(?::timestamptz,?::uuid)"
		args = append(args, q.BeforeTime, q.BeforeID)
	}
	if q.ParentID != "" {
		if kind != d.Prices && kind != d.Usages {
			return nil, d.ErrInvalid
		}
		query += " AND t.model_id=?"
		args = append(args, q.ParentID)
	}
	if kind == d.Usages {
		if q.ChannelID != "" {
			query += " AND t.channel_id=?"
			args = append(args, q.ChannelID)
		}
		if q.Status != "" {
			query += " AND t.status=?"
			args = append(args, q.Status)
		}
		if q.DateFrom != "" {
			query += " AND t.business_date>=?::date"
			args = append(args, q.DateFrom)
		}
		if q.DateTo != "" {
			query += " AND t.business_date<=?::date"
			args = append(args, q.DateTo)
		}
	}
	query += " ORDER BY t." + column + " DESC,t.id DESC LIMIT ?"
	args = append(args, q.Limit)
	rows, err := db.Raw(query, args...).Rows()
	if err != nil {
		return nil, dbError(err)
	}
	defer func() { _ = rows.Close() }()
	out := []json.RawMessage{}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		value, err := normalize(kind, raw)
		if err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	return out, rows.Err()
}
func columns(kind d.Kind) []string {
	switch kind {
	case d.Providers:
		return strings.Fields("code title protocol enabled")
	case d.Models:
		return strings.Fields("provider_id upstream_name title capabilities context_window max_output_tokens embedding_dimensions enabled")
	case d.Channels:
		return strings.Fields("provider_id model_id title base_url upstream_model priority weight rpm concurrency enabled")
	case d.Keys:
		return strings.Fields("channel_id title secret_kind secret_locator secret_version fingerprint weight rpm concurrency enabled health")
	}
	return nil
}
func (s *Store) Put(ctx context.Context, tenant string, kind d.Kind, id string, expected int64, raw json.RawMessage) (json.RawMessage, error) {
	cols := columns(kind)
	if len(cols) == 0 {
		return nil, d.ErrInvalid
	}
	db, err := s.manager.Require(ctx)
	if err != nil {
		return nil, err
	}
	if kind == d.Keys {
		obj := map[string]any{}
		if json.Unmarshal(raw, &obj) != nil {
			return nil, d.ErrInvalid
		}
		ref, ok := obj["secret_ref"].(map[string]any)
		if !ok {
			return nil, d.ErrInvalid
		}
		loc, _ := ref["locator"].(string)
		loc = strings.TrimSpace(loc)
		if id != "" && IsMaskedSecret(loc) {
			var existingLoc string
			if err := db.Raw("SELECT secret_locator FROM llm_keys WHERE tenant_id=? AND id=?", tenant, id).Row().Scan(&existingLoc); err == nil && existingLoc != "" {
				loc = existingLoc
			}
		} else if loc != "" && !envNameRegex.MatchString(loc) && !IsEncryptedSecret(loc) {
			encrypted, encErr := EncryptSecret(loc)
			if encErr == nil {
				loc = encrypted
				ref["kind"] = "plain"
			}
		}
		obj["secret_kind"] = ref["kind"]
		obj["secret_locator"] = loc
		obj["secret_version"] = ref["version"]
		delete(obj, "secret_ref")
		raw, _ = json.Marshal(obj)
	}
	tbl := table(kind)
	if id == "" {
		id = uuid.NewString()
		query := "INSERT INTO " + tbl + " (id,tenant_id," + strings.Join(cols, ",") + ") SELECT ?::uuid,?::uuid," + strings.Join(cols, ",") + " FROM jsonb_populate_record(NULL::" + tbl + ",?::jsonb)"
		if err = db.Exec(query, id, tenant, string(raw)).Error; err != nil {
			return nil, dbError(err)
		}
	} else {
		setters := []string{}
		for _, col := range cols {
			setters = append(setters, col+"=v."+col)
		}
		result := db.Exec("UPDATE "+tbl+" t SET "+strings.Join(setters, ",")+",revision=t.revision+1,updated_at=clock_timestamp() FROM jsonb_populate_record(NULL::"+tbl+",?::jsonb) v WHERE t.tenant_id=? AND t.id=? AND t.revision=? AND t.deleted_at IS NULL", string(raw), tenant, id, expected)
		if result.Error != nil {
			return nil, dbError(result.Error)
		}
		if result.RowsAffected != 1 {
			return nil, d.ErrConflict
		}
	}
	if kind == d.Channels {
		syncChannelModels(db, tenant, id, raw)
	}
	return s.Get(ctx, tenant, kind, id)
}

func syncChannelModels(db *gorm.DB, tenant, channelID string, raw json.RawMessage) {
	var input struct {
		ModelID       string                       `json:"model_id"`
		ModelIDs      []string                     `json:"model_ids"`
		Models        []d.ChannelModelBindingInput `json:"models"`
		UpstreamModel string                       `json:"upstream_model"`
		Priority      int64                        `json:"priority"`
		Weight        int64                        `json:"weight"`
		Enabled       bool                         `json:"enabled"`
	}
	if json.Unmarshal(raw, &input) != nil {
		return
	}

	type modelItem struct {
		modelID       string
		upstreamModel string
		priority      int64
		weight        int64
		enabled       bool
	}

	targetModels := make([]modelItem, 0)
	seen := make(map[string]bool)

	for _, m := range input.Models {
		if m.ModelID != "" && !seen[m.ModelID] {
			seen[m.ModelID] = true
			upModel := m.UpstreamModel
			if upModel == "" {
				upModel = input.UpstreamModel
			}
			p := input.Priority
			if m.Priority != nil {
				p = *m.Priority
			}
			w := input.Weight
			if m.Weight != nil {
				w = *m.Weight
			}
			en := input.Enabled
			if m.Enabled != nil {
				en = *m.Enabled
			}
			targetModels = append(targetModels, modelItem{
				modelID:       m.ModelID,
				upstreamModel: upModel,
				priority:      p,
				weight:        w,
				enabled:       en,
			})
		}
	}

	for _, mid := range input.ModelIDs {
		if mid != "" && !seen[mid] {
			seen[mid] = true
			targetModels = append(targetModels, modelItem{
				modelID:       mid,
				upstreamModel: input.UpstreamModel,
				priority:      input.Priority,
				weight:        input.Weight,
				enabled:       input.Enabled,
			})
		}
	}

	if input.ModelID != "" && !seen[input.ModelID] {
		seen[input.ModelID] = true
		targetModels = append(targetModels, modelItem{
			modelID:       input.ModelID,
			upstreamModel: input.UpstreamModel,
			priority:      input.Priority,
			weight:        input.Weight,
			enabled:       input.Enabled,
		})
	}

	if len(targetModels) == 0 {
		return
	}

	mids := make([]string, 0, len(targetModels))
	for _, tm := range targetModels {
		mids = append(mids, tm.modelID)
	}
	_ = db.Exec("DELETE FROM llm_channel_models WHERE tenant_id = ? AND channel_id = ? AND model_id NOT IN (?)", tenant, channelID, mids)

	for _, tm := range targetModels {
		w := tm.weight
		if w < 1 {
			w = 100
		}
		_ = db.Exec(`
			INSERT INTO llm_channel_models (tenant_id, channel_id, model_id, upstream_model, priority, weight, enabled, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, clock_timestamp())
			ON CONFLICT (tenant_id, channel_id, model_id)
			DO UPDATE SET upstream_model = EXCLUDED.upstream_model,
			              priority = EXCLUDED.priority,
			              weight = EXCLUDED.weight,
			              enabled = EXCLUDED.enabled,
			              deleted_at = NULL,
			              updated_at = clock_timestamp()
		`, tenant, channelID, tm.modelID, tm.upstreamModel, tm.priority, w, tm.enabled)
	}
}

func (s *Store) Delete(ctx context.Context, tenant string, kind d.Kind, id string, expected int64) error {
	if len(columns(kind)) == 0 && kind != d.Policies {
		return d.ErrInvalid
	}
	db, err := s.manager.Require(ctx)
	if err != nil {
		return err
	}
	// Guard immutable versions even if a caller bypasses the Catalog service.
	// This runs in the existing mutation transaction.
	if kind == d.Policies {
		used, err := s.InUse(ctx, tenant, kind, id)
		if err != nil {
			return err
		}
		if used {
			return d.ErrInUse
		}
	}
	if kind == d.Models {
		_ = db.Exec("DELETE FROM llm_channel_models WHERE tenant_id=? AND model_id=?", tenant, id)
		_ = db.Exec("DELETE FROM llm_pricing_revisions WHERE tenant_id=? AND model_id=?", tenant, id)
		_ = db.Exec("DELETE FROM llm_keys WHERE tenant_id=? AND channel_id IN (SELECT id FROM llm_channels WHERE tenant_id=? AND model_id=?)", tenant, tenant, id)
		_ = db.Exec("DELETE FROM llm_channels WHERE tenant_id=? AND model_id=?", tenant, id)
		_ = db.Exec("UPDATE llm_task_policies SET current_version_id=NULL WHERE tenant_id=? AND current_version_id IN (SELECT id FROM llm_task_policy_versions WHERE tenant_id=? AND primary_model_id=?)", tenant, tenant, id)
		_ = db.Exec("DELETE FROM llm_task_policy_versions WHERE tenant_id=? AND primary_model_id=?", tenant, id)
		_ = db.Exec("DELETE FROM llm_task_policies WHERE tenant_id=? AND current_version_id IS NULL AND id NOT IN (SELECT policy_id FROM llm_task_policy_versions WHERE tenant_id=?)", tenant, tenant)
	} else if kind == d.Channels {
		_ = db.Exec("DELETE FROM llm_channel_models WHERE tenant_id=? AND channel_id=?", tenant, id)
		_ = db.Exec("DELETE FROM llm_keys WHERE tenant_id=? AND channel_id=?", tenant, id)
	} else if kind == d.Providers {
		_ = db.Exec("DELETE FROM llm_channel_models WHERE tenant_id=? AND channel_id IN (SELECT id FROM llm_channels WHERE tenant_id=? AND provider_id=?)", tenant, tenant, id)
		_ = db.Exec("DELETE FROM llm_keys WHERE tenant_id=? AND channel_id IN (SELECT id FROM llm_channels WHERE tenant_id=? AND provider_id=?)", tenant, tenant, id)
		_ = db.Exec("DELETE FROM llm_channels WHERE tenant_id=? AND provider_id=?", tenant, id)
		_ = db.Exec("DELETE FROM llm_pricing_revisions WHERE tenant_id=? AND model_id IN (SELECT id FROM llm_models WHERE tenant_id=? AND provider_id=?)", tenant, tenant, id)
		_ = db.Exec("UPDATE llm_task_policies SET current_version_id=NULL WHERE tenant_id=? AND current_version_id IN (SELECT v.id FROM llm_task_policy_versions v JOIN llm_models m ON m.tenant_id=v.tenant_id AND m.id=v.primary_model_id WHERE m.tenant_id=? AND m.provider_id=?)", tenant, tenant, id)
		_ = db.Exec("DELETE FROM llm_task_policy_versions WHERE tenant_id=? AND primary_model_id IN (SELECT id FROM llm_models WHERE tenant_id=? AND provider_id=?)", tenant, id)
		_ = db.Exec("DELETE FROM llm_task_policies WHERE tenant_id=? AND current_version_id IS NULL AND id NOT IN (SELECT policy_id FROM llm_task_policy_versions WHERE tenant_id=?)", tenant, tenant)
		_ = db.Exec("DELETE FROM llm_models WHERE tenant_id=? AND provider_id=?", tenant, id)
	} else if kind == d.Policies {
		_ = db.Exec("UPDATE llm_task_policies SET current_version_id=NULL WHERE tenant_id=? AND id=?", tenant, id)
		_ = db.Exec("DELETE FROM llm_task_policy_versions WHERE tenant_id=? AND policy_id=?", tenant, id)
	}
	result := db.Exec("DELETE FROM "+table(kind)+" WHERE tenant_id=? AND id=? AND revision=?", tenant, id, expected)
	if result.Error != nil {
		return dbError(result.Error)
	}
	if result.RowsAffected != 1 {
		return d.ErrConflict
	}
	return nil
}
func (s *Store) InUse(ctx context.Context, tenant string, kind d.Kind, id string) (bool, error) {
	db, err := s.manager.DB(ctx)
	if err != nil {
		return false, err
	}
	if kind == d.Policies {
		// Serialize deletion with CreateRun's FOR SHARE on the policy head.
		// Within Catalog.Delete's UoW this lock remains held until commit.
		var lockedID string
		if err := db.Raw("SELECT id::text FROM llm_task_policies WHERE tenant_id=? AND id=? FOR UPDATE", tenant, id).Row().Scan(&lockedID); err != nil {
			return false, dbError(err)
		}
	}
	var query string
	switch kind {
	case d.Providers:
		query = "SELECT EXISTS(SELECT 1 FROM llm_models WHERE tenant_id=? AND provider_id=?)"
	case d.Models:
		query = "SELECT EXISTS(SELECT 1 FROM llm_channels WHERE tenant_id=? AND model_id=? UNION ALL SELECT 1 FROM llm_task_policies p JOIN llm_task_policy_versions v ON v.tenant_id=p.tenant_id AND v.id=p.current_version_id WHERE p.tenant_id=? AND (v.primary_model_id=? OR ?::uuid=ANY(v.fallback_model_ids)))"
	case d.Channels:
		query = "SELECT EXISTS(SELECT 1 FROM llm_keys WHERE tenant_id=? AND channel_id=?)"
	case d.Keys:
		query = "SELECT EXISTS(SELECT 1 FROM llm_reservations WHERE tenant_id=? AND key_id=? AND status IN ('reserved','dispatched'))"
	case d.Policies:
		query = "SELECT EXISTS(SELECT 1 FROM llm_task_policy_versions v " +
			"WHERE v.tenant_id=? AND v.policy_id=? AND (" +
			"EXISTS(SELECT 1 FROM llm_reservations r WHERE r.tenant_id=v.tenant_id AND r.policy_version_id=v.id AND r.status IN ('reserved','dispatched')) " +
			"OR EXISTS(SELECT 1 FROM agent_runs run WHERE run.tenant_id=v.tenant_id AND run.policy_version_id=v.id) " +
			"OR EXISTS(SELECT 1 FROM agent_run_questions question WHERE question.tenant_id=v.tenant_id AND question.policy_version_id=v.id) " +
			"OR EXISTS(SELECT 1 FROM agent_novel_creation_sessions guide WHERE guide.tenant_id=v.tenant_id AND guide.policy_version_id=v.id)))"
	default:
		return false, d.ErrInvalid
	}
	args := []any{tenant, id}
	if kind == d.Models {
		args = append(args, tenant, id, id)
	}
	var used bool
	err = db.Raw(query, args...).Row().Scan(&used)
	return used, dbError(err)
}
func dbError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) || err.Error() == "sql: no rows in result set" {
		return d.ErrNotFound
	}
	type sqlstate interface{ SQLState() string }
	var state sqlstate
	if errors.As(err, &state) {
		switch state.SQLState() {
		case "23505":
			return d.ErrConflict
		case "23503":
			return d.ErrInUse
		case "23514", "22P02":
			return d.ErrInvalid
		}
	}
	return errors.New("llm: persistence operation failed")
}

func (s *Store) Publish(ctx context.Context, tenant, id string, expected int64, input d.PolicyInput) (d.Policy, error) {
	var out d.Policy
	db, err := s.manager.Require(ctx)
	if err != nil {
		return out, err
	}
	version := int64(1)
	if id == "" {
		id = uuid.NewString()
		err = db.Exec("INSERT INTO llm_task_policies(id,tenant_id,task_key,title,enabled) VALUES (?::uuid,?::uuid,?,?,?)", id, tenant, input.TaskKey, input.Title, input.Enabled).Error
		if err != nil {
			return out, dbError(err)
		}
	} else {
		var old d.Policy
		raw, err := s.Get(ctx, tenant, d.Policies, id)
		if err != nil {
			return out, err
		}
		if json.Unmarshal(raw, &old) != nil {
			return out, d.ErrInvalid
		}
		if old.Revision != expected {
			return out, d.ErrConflict
		}
		if old.TaskKey != input.TaskKey {
			return out, d.ErrInvalid
		}
		version = old.Version + 1
		if version > 2147483647 {
			return out, d.ErrInvalid
		}
	}
	versionID := uuid.NewString()
	raw, _ := json.Marshal(input)
	err = db.Exec("INSERT INTO llm_task_policy_versions(id,tenant_id,policy_id,version,primary_model_id,fallback_model_ids,max_output_tokens,temperature,timeout_seconds,max_retries,required_capabilities,output_mode) SELECT ?::uuid,?::uuid,?::uuid,?,primary_model_id,fallback_model_ids,max_output_tokens,temperature,timeout_seconds,max_retries,required_capabilities,output_mode FROM jsonb_populate_record(NULL::llm_task_policy_versions,?::jsonb)", versionID, tenant, id, version, string(raw)).Error
	if err != nil {
		return out, dbError(err)
	}
	rev := expected
	if rev == 0 {
		rev = 1
	}
	next := rev + 1
	if expected == 0 {
		next = 1
	}
	result := db.Exec("UPDATE llm_task_policies SET current_version_id=?::uuid,title=?,enabled=?,revision=?,updated_at=clock_timestamp() WHERE tenant_id=? AND id=? AND revision=? AND deleted_at IS NULL", versionID, input.Title, input.Enabled, next, tenant, id, rev)
	if result.Error != nil {
		return out, dbError(result.Error)
	}
	if result.RowsAffected != 1 {
		return out, d.ErrConflict
	}
	value, err := s.Get(ctx, tenant, d.Policies, id)
	if err != nil {
		return out, err
	}
	err = json.Unmarshal(value, &out)
	return out, err
}
func (s *Store) Price(ctx context.Context, tenant string, input d.PricingInput) (d.Pricing, error) {
	var out d.Pricing
	db, err := s.manager.Require(ctx)
	if err != nil {
		return out, err
	}
	var current time.Time
	if err = db.Raw("SELECT clock_timestamp()").Row().Scan(&current); err != nil {
		return out, dbError(err)
	}
	effective, err := time.Parse(time.RFC3339Nano, input.EffectiveAt)
	if err != nil || effective.Before(current) {
		return out, d.ErrPricingConflict
	}
	var version int64
	err = db.Raw("SELECT coalesce(max(version),0)+1 FROM llm_pricing_revisions WHERE tenant_id=? AND model_id=?", tenant, input.ModelID).Row().Scan(&version)
	if err != nil {
		return out, dbError(err)
	}
	if version > 2147483647 {
		return out, d.ErrInvalid
	}
	id := uuid.NewString()
	raw, _ := json.Marshal(input)
	err = db.Exec("INSERT INTO llm_pricing_revisions(id,tenant_id,model_id,version,currency,input_micros_per_million,output_micros_per_million,cache_micros_per_million,effective_at) SELECT ?::uuid,?::uuid,model_id,?,currency,input_micros_per_million,output_micros_per_million,cache_micros_per_million,effective_at FROM jsonb_populate_record(NULL::llm_pricing_revisions,?::jsonb)", id, tenant, version, string(raw)).Error
	if err != nil {
		return out, dbError(err)
	}
	value, err := s.Get(ctx, tenant, d.Prices, id)
	if err != nil {
		return out, err
	}
	err = json.Unmarshal(value, &out)
	return out, err
}

// SecretBinding is deployment metadata, never a credential value.
type SecretBinding struct {
	ProviderID string
	Ref        d.SecretRef
}

// Resolve returns an owned byte slice that this adapter clears after use.
type SecretOptions struct {
	Bindings  []SecretBinding
	Endpoints map[string][]string
	Pepper    []byte
	Resolve   func(context.Context, SecretBinding) ([]byte, error)
}
type Secrets struct {
	bindings  map[SecretBinding]bool
	endpoints map[string]map[string]bool
	pepper    []byte
	resolve   func(context.Context, SecretBinding) ([]byte, error)
}

func NewSecrets(o SecretOptions) (*Secrets, error) {
	if len(o.Pepper) < 32 || o.Resolve == nil {
		return nil, d.ErrSecret
	}
	s := &Secrets{bindings: map[SecretBinding]bool{}, endpoints: map[string]map[string]bool{}, pepper: append([]byte{}, o.Pepper...), resolve: o.Resolve}
	env := regexp.MustCompile("^[A-Z_][A-Z0-9_]*$")
	for _, binding := range o.Bindings {
		ref := binding.Ref
		if binding.ProviderID == "" || len(ref.Locator) < 1 || len(ref.Locator) > 1024 || len(ref.Version) < 1 || len(ref.Version) > 64 {
			return nil, d.ErrSecret
		}
		if (ref.Kind != "env" && ref.Kind != "secret_store" && ref.Kind != "plain") || (ref.Kind == "env" && !env.MatchString(ref.Locator)) {
			return nil, d.ErrSecret
		}
		if strings.IndexFunc(ref.Locator, func(r rune) bool { return r < 32 || r == 92 }) >= 0 || strings.Contains(ref.Locator, "..") {
			return nil, d.ErrSecret
		}
		s.bindings[binding] = true
	}
	for protocol, urls := range o.Endpoints {
		for _, endpoint := range urls {
			u, err := url.Parse(endpoint)
			if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "http") {
				return nil, d.ErrInvalid
			}
			if s.endpoints[protocol] == nil {
				s.endpoints[protocol] = map[string]bool{}
			}
			s.endpoints[protocol][endpoint] = true
		}
	}
	return s, nil
}

func isLocalOrPrivateHost(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	if sh, _, err := net.SplitHostPort(h); err == nil {
		h = sh
	}
	if h == "localhost" || h == "127.0.0.1" || h == "::1" || h == "[::1]" || strings.HasSuffix(h, ".local") {
		return true
	}
	if ip := net.ParseIP(h); ip != nil {
		return ip.IsLoopback() || ip.IsPrivate()
	}
	return false
}

func (s *Secrets) ValidateEndpoint(protocol, endpoint string) error {
	if s == nil {
		return d.ErrInvalid
	}
	if s.endpoints[protocol] != nil && s.endpoints[protocol][endpoint] {
		return nil
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return d.ErrInvalid
	}
	if u.Scheme == "https" || (u.Scheme == "http" && (protocol == "ollama" || isLocalOrPrivateHost(u.Host))) {
		return nil
	}
	return d.ErrInvalid
}
func (s *Secrets) Fingerprint(ctx context.Context, provider string, ref d.SecretRef) (string, error) {
	binding := SecretBinding{provider, ref}
	if s == nil {
		return "", d.ErrSecret
	}
	if len(s.bindings) > 0 && !s.bindings[binding] && ref.Kind != "plain" {
		return "", d.ErrSecret
	}
	secret, err := s.resolve(ctx, binding)
	if err != nil || len(secret) == 0 {
		return "", d.ErrSecret
	}
	defer clear(secret)
	mac := hmac.New(sha256.New, s.pepper)
	mac.Write([]byte(provider))
	mac.Write([]byte{0})
	mac.Write(secret)
	return hex.EncodeToString(mac.Sum(nil)), nil
}

// WithSecret restricts plaintext to a synchronous driver callback. Drivers must
// enforce TLS and disable redirects. Callback error text is never propagated.
func (s *Secrets) WithSecret(ctx context.Context, provider string, ref d.SecretRef, fn func([]byte) error) error {
	binding := SecretBinding{provider, ref}
	if s == nil || fn == nil {
		return d.ErrSecret
	}
	if len(s.bindings) > 0 && !s.bindings[binding] && ref.Kind != "plain" {
		return d.ErrSecret
	}
	secret, err := s.resolve(ctx, binding)
	if err != nil || len(secret) == 0 {
		return d.ErrSecret
	}
	defer clear(secret)
	return fn(secret)
}
