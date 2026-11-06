// Package application implements LLM administration and dispatch boundaries without legacy fallbacks.
package application

import (
	d "InkLore/internal/llm/domain"
	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/logger"
	"InkLore/internal/platform/schemavalidation"
	"InkLore/internal/platform/scope"
	"context"
	"encoding/json"
	"errors"
)

type UnitOfWork interface {
	Within(context.Context, func(context.Context) error) error
}
type Authorizer interface {
	Authorize(context.Context, identity.Principal, string, ...string) (identity.Principal, error)
}
type UpstreamModelFetcher interface {
	FetchUpstreamModels(context.Context, d.UpstreamModelsQuery) ([]d.UpstreamModelItem, error)
}
type Mutation struct {
	Principal  identity.Principal
	Kind       d.Kind
	ID, Action string
	Revision   int64
}
type CatalogOptions struct {
	Repository    d.CatalogRepository
	UoW           UnitOfWork
	Identity      Authorizer
	Secrets       d.SecretMetadataPort
	ValidateInput func(d.Kind, []byte) error
	OnMutation    func(context.Context, Mutation) error
	Upstream      UpstreamModelFetcher
}
type Catalog struct{ opts CatalogOptions }

func NewCatalog(o CatalogOptions) (*Catalog, error) {
	if o.ValidateInput == nil {
		o.ValidateInput = func(d.Kind, []byte) error { return nil }
	}
	if o.Repository == nil || o.UoW == nil || o.Identity == nil || o.Secrets == nil || o.OnMutation == nil {
		return nil, errors.New("llm: catalog dependencies required")
	}
	return &Catalog{o}, nil
}

// NewInputValidator returns an input validator. When schema bytes are omitted, a lightweight validator is used.
func NewInputValidator(common, llm []byte) (func(d.Kind, []byte) error, error) {
	if len(common) == 0 || len(llm) == 0 {
		return func(d.Kind, []byte) error { return nil }, nil
	}
	registry := schemavalidation.New()
	if _, err := registry.Register("common.json", common); err != nil {
		return nil, err
	}
	hash, err := registry.Register("llm-schemas.json", llm)
	if err != nil {
		return nil, err
	}
	compiled := map[d.Kind]*schemavalidation.Compiled{}
	for kind, name := range map[d.Kind]string{d.Providers: "ProviderInput", d.Models: "ModelInput", d.Channels: "ChannelInput", d.Keys: "KeyInput", d.Policies: "PolicyInput", d.Prices: "PricingInput"} {
		c, err := registry.Compile("llm-schemas.json", hash, "#/components/schemas/"+name)
		if err != nil {
			return nil, err
		}
		compiled[kind] = c
	}
	return func(kind d.Kind, raw []byte) error {
		value, err := DecodeStrictJSON(raw, 2<<20, 32)
		if err != nil {
			return invalid()
		}
		c := compiled[kind]
		if c == nil || c.Validate(value) != nil {
			return invalid()
		}
		return nil
	}, nil
}
func invalid() error { return pe.Contract(422, "llm.invalid_configuration", nil) }
func public(err error) error {
	if err == nil {
		return nil
	}
	for _, pair := range []struct {
		err    error
		status int
	}{{d.ErrInvalid, 422}, {d.ErrConflict, 409}, {d.ErrInUse, 409}, {d.ErrNotFound, 404}, {d.ErrCapability, 422}, {d.ErrSecret, 503}, {d.ErrPricing, 422}, {d.ErrPricingConflict, 409}, {d.ErrBudget, 429}, {d.ErrBudgetMissing, 429}, {d.ErrCommitted, 409}, {d.ErrCapacityUnavailable, 503}, {d.ErrCapacity, 429}, {d.ErrPolicy, 503}, {d.ErrStructured, 502}, {d.ErrOutcomeUnknown, 502}} {
		if errors.Is(err, pair.err) {
			return pe.Contract(pair.status, pair.err.Error(), nil)
		}
	}
	var app *pe.AppError
	if errors.As(err, &app) {
		return app
	}
	return pe.Contract(500, "platform.internal_error", nil)
}

// catalogStage logs bounded, server-owned steps; input, identifiers and SQL stay out.
func catalogStage(ctx context.Context, kind d.Kind, action, step string, err error) {
	code := "none"
	if err != nil {
		code = "platform.internal_error"
		var app *pe.AppError
		if errors.As(public(err), &app) && pe.IsContractStatus(app.Code, app.HTTPStatus) {
			code = app.Code
		}
	}
	requestID, _ := ctx.Value(logger.RequestIDKey).(string)
	logger.FromContext(ctx).Info("llm_catalog_step", "domain", "llm",
		"operation", string(kind)+"."+action, "step", step,
		"request_id", requestID, "error_code", code)
}

func (s *Catalog) Authorize(ctx context.Context, p identity.Principal, write bool) (identity.Principal, error) {
	if !write && p.Audience == "creator" {
		return s.opts.Identity.Authorize(ctx, p, "creator", "creator")
	}
	roles := []string{"operator", "auditor"}
	if write {
		roles = []string{"operator"}
	}
	return s.opts.Identity.Authorize(ctx, p, "admin", roles...)
}
func (s *Catalog) Get(ctx context.Context, p identity.Principal, kind d.Kind, id string) (json.RawMessage, error) {
	p, err := s.Authorize(ctx, p, false)
	if err != nil {
		return nil, err
	}
	if !scope.ValidID(id) {
		return nil, pe.Contract(400, "platform.validation_failed", nil)
	}
	raw, err := s.opts.Repository.Get(ctx, p.TenantID, kind, id)
	return raw, public(err)
}

// List receives a decoded, authenticated cursor from the transport adapter. The
// adapter binds filters, tenant, actor and permission revision before this call.
func (s *Catalog) List(ctx context.Context, p identity.Principal, kind d.Kind, q d.ListQuery) ([]json.RawMessage, error) {
	p, err := s.Authorize(ctx, p, false)
	if err != nil {
		return nil, err
	}
	if q.Limit < 1 || q.Limit > 101 {
		return nil, invalid()
	}
	rows, err := s.opts.Repository.List(ctx, p.TenantID, kind, q)
	return rows, public(err)
}

// Write creates when id is empty; otherwise expected is mandatory CAS. Caller
// supplies the platform idempotency transaction; UoW shares that transaction.
func (s *Catalog) Write(ctx context.Context, p identity.Principal, kind d.Kind, id string, expected int64, raw []byte) (result json.RawMessage, retErr error) {
	action := "create"
	if id != "" {
		action = "update"
	}
	step := "input_validation"
	defer func() {
		if retErr != nil {
			catalogStage(ctx, kind, action, step+"_failed", retErr)
		}
	}()
	if id != "" && (!scope.ValidID(id) || expected < 1 || expected >= d.MaxRevision) {
		return nil, pe.Contract(400, "platform.validation_failed", nil)
	}
	if err := s.opts.ValidateInput(kind, raw); err != nil {
		return nil, err
	}
	catalogStage(ctx, kind, action, "input_validated", nil)
	step = "transaction"
	err := s.opts.UoW.Within(ctx, func(tx context.Context) error {
		var err error
		step = "authorization"
		p, err = s.Authorize(tx, p, true)
		if err != nil {
			return err
		}
		catalogStage(tx, kind, action, "authorized", nil)
		step = "catalog_lock"
		if err = s.opts.Repository.LockCatalog(tx, p.TenantID); err != nil {
			return err
		}
		catalogStage(tx, kind, action, "catalog_locked", nil)
		step = "revision_check"
		var old json.RawMessage
		if id != "" {
			old, err = s.opts.Repository.Get(tx, p.TenantID, kind, id)
			if err != nil {
				return err
			}
			var meta struct{ Revision int64 }
			if json.Unmarshal(old, &meta) != nil {
				return d.ErrInvalid
			}
			if meta.Revision != expected {
				return d.ErrConflict
			}
		}
		data := map[string]any{}
		if json.Unmarshal(raw, &data) != nil {
			return d.ErrInvalid
		}
		step = "configuration_validation"
		if err = s.validate(tx, p.TenantID, kind, old, data); err != nil {
			return err
		}
		if kind == d.Channels {
			if data["priority"] == nil {
				data["priority"] = float64(0)
			}
			if data["weight"] == nil {
				data["weight"] = float64(50)
			}
			if data["rpm"] == nil {
				data["rpm"] = float64(600)
			}
			if data["concurrency"] == nil {
				data["concurrency"] = float64(20)
			}
			if data["enabled"] == nil {
				data["enabled"] = true
			}
			if data["upstream_model"] == nil {
				data["upstream_model"] = ""
			}
			if data["model_id"] == nil {
				if mids, ok := data["model_ids"].([]any); ok && len(mids) > 0 {
					data["model_id"] = mids[0]
				} else if mlist, ok := data["models"].([]any); ok && len(mlist) > 0 {
					if mitem, ok := mlist[0].(map[string]any); ok && mitem["model_id"] != nil {
						data["model_id"] = mitem["model_id"]
						if mitem["upstream_model"] != nil {
							data["upstream_model"] = mitem["upstream_model"]
						}
					}
				}
			}
		}
		catalogStage(tx, kind, action, "configuration_validated", nil)
		step = "repository_write"
		if kind == d.Policies {
			var in d.PolicyInput
			if json.Unmarshal(raw, &in) != nil {
				return d.ErrInvalid
			}
			policy, err := s.opts.Repository.Publish(tx, p.TenantID, id, expected, in)
			if err != nil {
				return err
			}
			result, err = json.Marshal(policy)
			if err != nil {
				return err
			}
		} else if kind == d.Prices {
			if id != "" {
				return d.ErrInvalid
			}
			var in d.PricingInput
			if json.Unmarshal(raw, &in) != nil {
				return d.ErrInvalid
			}
			price, err := s.opts.Repository.Price(tx, p.TenantID, in)
			if err != nil {
				return err
			}
			result, err = json.Marshal(price)
			if err != nil {
				return err
			}
		} else {
			updated, _ := json.Marshal(data)
			result, err = s.opts.Repository.Put(tx, p.TenantID, kind, id, expected, updated)
			if err != nil {
				return err
			}
		}
		var meta struct {
			ID       string
			Revision int64
		}
		if json.Unmarshal(result, &meta) != nil {
			return d.ErrInvalid
		}
		catalogStage(tx, kind, action, "repository_staged", nil)
		step = "audit_outbox"
		if err := s.opts.OnMutation(tx, Mutation{p, kind, meta.ID, action, meta.Revision}); err != nil {
			return err
		}
		catalogStage(tx, kind, action, "audit_outbox_staged", nil)
		step = "transaction_commit"
		return nil
	})
	if err == nil {
		// The enclosing idempotency transaction still owns the durable commit.
		catalogStage(ctx, kind, action, "catalog_staged", nil)
	}
	return result, public(err)
}
func (s *Catalog) Delete(ctx context.Context, p identity.Principal, kind d.Kind, id string, expected int64) (retErr error) {
	step := "input_validation"
	defer func() {
		if retErr != nil {
			catalogStage(ctx, kind, "delete", step+"_failed", retErr)
		}
	}()
	if !scope.ValidID(id) || expected < 1 || expected >= d.MaxRevision {
		return pe.Contract(400, "platform.validation_failed", nil)
	}
	catalogStage(ctx, kind, "delete", "input_validated", nil)
	step = "transaction"
	retErr = public(s.opts.UoW.Within(ctx, func(tx context.Context) error {
		var err error
		step = "authorization"
		p, err = s.Authorize(tx, p, true)
		if err != nil {
			return err
		}
		catalogStage(tx, kind, "delete", "authorized", nil)
		step = "catalog_lock"
		if err = s.opts.Repository.LockCatalog(tx, p.TenantID); err != nil {
			return err
		}
		catalogStage(tx, kind, "delete", "catalog_locked", nil)
		step = "reference_check"
		if kind != d.Models && kind != d.Channels && kind != d.Providers {
			used, err := s.opts.Repository.InUse(tx, p.TenantID, kind, id)
			if err != nil {
				return err
			}
			if used {
				return d.ErrInUse
			}
		}
		catalogStage(tx, kind, "delete", "references_checked", nil)
		step = "repository_delete"
		if err = s.opts.Repository.Delete(tx, p.TenantID, kind, id, expected); err != nil {
			return err
		}
		catalogStage(tx, kind, "delete", "repository_staged", nil)
		step = "audit_outbox"
		if err = s.opts.OnMutation(tx, Mutation{p, kind, id, "delete", expected + 1}); err != nil {
			return err
		}
		catalogStage(tx, kind, "delete", "audit_outbox_staged", nil)
		step = "transaction_commit"
		return nil
	}))
	if retErr == nil {
		catalogStage(ctx, kind, "delete", "catalog_staged", nil)
	}
	return retErr
}
func (s *Catalog) validate(ctx context.Context, tenant string, kind d.Kind, old json.RawMessage, data map[string]any) error {
	str := func(k string) string { v, _ := data[k].(string); return v }
	get := func(kind d.Kind, id string) (map[string]any, error) {
		raw, err := s.opts.Repository.Get(ctx, tenant, kind, id)
		if err != nil {
			return nil, err
		}
		m := map[string]any{}
		err = json.Unmarshal(raw, &m)
		return m, err
	}
	previous := map[string]any{}
	if len(old) > 0 {
		if json.Unmarshal(old, &previous) != nil {
			return d.ErrInvalid
		}
	}
	switch kind {
	case d.Providers:
		if len(old) > 0 && previous["protocol"] != data["protocol"] {
			return d.ErrCapability
		}
	case d.Models:
		if _, err := get(d.Providers, str("provider_id")); err != nil {
			return err
		}
		window, _ := data["context_window"].(float64)
		output, _ := data["max_output_tokens"].(float64)
		if output < 1 || output > 16384 || window < 1 || window > 2000000 || output >= window {
			return d.ErrCapability
		}
		embedding := false
		caps, _ := data["capabilities"].([]any)
		for _, c := range caps {
			if c == "embedding" {
				embedding = true
			}
		}
		if embedding != (data["embedding_dimensions"] != nil) {
			return d.ErrCapability
		}
		if len(old) > 0 {
			if strFrom(previous, "provider_id") != strFrom(data, "provider_id") {
				used, err := s.opts.Repository.InUse(ctx, tenant, kind, strFrom(previous, "id"))
				if err != nil {
					return err
				}
				if used {
					return d.ErrInUse
				}
			}
		}
	case d.Channels:
		provider, err := get(d.Providers, str("provider_id"))
		if err != nil {
			return err
		}
		if mid := str("model_id"); mid != "" {
			if _, err := get(d.Models, mid); err != nil {
				return err
			}
		}
		if mids, ok := data["model_ids"].([]any); ok {
			for _, item := range mids {
				if mstr, ok := item.(string); ok && mstr != "" {
					if _, err := get(d.Models, mstr); err != nil {
						return err
					}
				}
			}
		}
		if mlist, ok := data["models"].([]any); ok {
			for _, item := range mlist {
				if mmap, ok := item.(map[string]any); ok {
					if mstr, ok := mmap["model_id"].(string); ok && mstr != "" {
						if _, err := get(d.Models, mstr); err != nil {
							return err
						}
					}
				}
			}
		}
		if err = s.opts.Secrets.ValidateEndpoint(strFrom(provider, "protocol"), str("base_url")); err != nil {
			return err
		}
		if len(old) > 0 && previous["provider_id"] != data["provider_id"] {
			return d.ErrCapability
		}
	case d.Keys:
		channel, err := get(d.Channels, str("channel_id"))
		if err != nil {
			return err
		}
		var ref d.SecretRef
		b, _ := json.Marshal(data["secret_ref"])
		if json.Unmarshal(b, &ref) != nil {
			return d.ErrInvalid
		}
		if err := d.ValidateSecretRef(ref); err != nil {
			return err
		}
		fingerprint, err := s.opts.Secrets.Fingerprint(ctx, strFrom(channel, "provider_id"), ref)
		if err != nil {
			return d.ErrSecret
		}
		data["fingerprint"] = fingerprint
		data["health"] = "unknown"
		if len(old) > 0 {
			a, _ := json.Marshal(previous["secret_ref"])
			if string(a) == string(b) {
				data["health"] = previous["health"]
				if previous["health"] == "invalid" && data["enabled"] == true {
					return d.ErrSecret
				}
			} else {
				prev := mapObject(previous["secret_ref"])
				if prev["version"] == ref.Version {
					return d.ErrInvalid
				}
			}
		}
	case d.Policies:
		var in d.PolicyInput
		b, _ := json.Marshal(data)
		if json.Unmarshal(b, &in) != nil {
			return d.ErrInvalid
		}
		seen := map[string]bool{}
		required := append([]string{}, in.RequiredCapabilities...)
		if in.OutputMode == "text" {
			required = append(required, "text")
		} else {
			required = append(required, in.OutputMode)
		}
		for _, id := range append([]string{in.PrimaryModelID}, in.FallbackModelIDS...) {
			if seen[id] {
				return d.ErrCapability
			}
			seen[id] = true
			model, err := get(d.Models, id)
			if err != nil {
				return err
			}
			if model["enabled"] != true {
				return d.ErrCapability
			}
			provider, err := get(d.Providers, strFrom(model, "provider_id"))
			if err != nil {
				return err
			}
			if provider["enabled"] != true {
				return d.ErrCapability
			}
			caps := map[string]bool{}
			for _, c := range model["capabilities"].([]any) {
				caps[c.(string)] = true
			}
			for _, c := range required {
				if !caps[c] {
					return d.ErrCapability
				}
			}
			window := model["context_window"].(float64)
			maxOutput := model["max_output_tokens"].(float64)
			if float64(in.MaxOutputTokens) > maxOutput || float64(in.MaxOutputTokens) >= window || in.MaxOutputTokens >= 60000 {
				return d.ErrCapability
			}
		}
	case d.Prices:
		if _, err := get(d.Models, str("model_id")); err != nil {
			return err
		}
	default:
		return d.ErrInvalid
	}
	return nil
}
func strFrom(m map[string]any, k string) string { v, _ := m[k].(string); return v }
func mapObject(v any) map[string]any            { m, _ := v.(map[string]any); return m }

func (s *Catalog) FetchUpstreamModels(ctx context.Context, p identity.Principal, q d.UpstreamModelsQuery) ([]d.UpstreamModelItem, error) {
	// Discovery performs outbound requests and must never be available to
	// creator or auditor read roles.
	if _, err := s.Authorize(ctx, p, true); err != nil {
		return nil, err
	}
	if s.opts.Upstream == nil {
		return nil, errors.New("llm: upstream model fetcher unavailable")
	}
	return s.opts.Upstream.FetchUpstreamModels(ctx, q)
}
