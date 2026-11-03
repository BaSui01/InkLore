package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"

	a "InkLore/internal/llm/application"
	d "InkLore/internal/llm/domain"
	"InkLore/internal/platform/cursor"
	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/httpguard"
	"InkLore/internal/platform/idempotency"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/logger"
	"InkLore/internal/platform/response"
	"InkLore/internal/platform/scope"
	"InkLore/internal/platform/timeutil"

	"github.com/gin-gonic/gin"
)

type HTTPOptions struct {
	Catalog     *a.Catalog
	Budget      *a.Budget
	Identity    *identity.Service
	Idempotency *idempotency.Executor
	Cursor      *cursor.Codec
}
type Options = HTTPOptions
type Handler struct{ HTTPOptions }

func NewHandler(o HTTPOptions) (*Handler, error) {
	if o.Catalog == nil || o.Budget == nil || o.Identity == nil || o.Idempotency == nil || o.Cursor == nil {
		return nil, errors.New("llm: catalog, budget, identity, idempotency and cursor required")
	}
	return &Handler{o}, nil
}
func bad() error         { return pe.Contract(400, "platform.validation_failed", nil) }
func unavailable() error { return pe.Contract(503, "llm.policy_unavailable", nil) }

// logCatalogStep emits only server-owned stage names and the Gin route template.
func logCatalogStep(c *gin.Context, step string) {
	logger.FromContext(c.Request.Context()).Info("llm_catalog_step",
		"domain", "llm", "operation", c.Request.Method+" "+c.FullPath(),
		"request_id", c.GetString(response.ContextKeyRequestID), "step", step)
}

func (h *Handler) authenticate(c *gin.Context, write bool) (identity.Principal, bool) {
	header := c.GetHeader("Authorization")
	if !strings.HasPrefix(header, "Bearer ") || strings.Count(header, " ") != 1 {
		response.Fail(c, pe.Contract(401, "auth.unauthenticated", nil))
		return identity.Principal{}, false
	}
	aud := "admin"
	if !write {
		aud = "either"
	}
	p, err := h.Identity.Authenticate(c.Request.Context(), strings.TrimPrefix(header, "Bearer "), aud)
	if err == nil {
		p, err = h.Catalog.Authorize(c.Request.Context(), p, write)
	}
	if err != nil {
		response.Fail(c, err)
		return p, false
	}
	for _, param := range c.Params {
		if !scope.ValidID(param.Value) {
			response.Fail(c, bad())
			return p, false
		}
	}
	logCatalogStep(c, "authorized")
	return p, true
}
func readObject(c *gin.Context) (map[string]any, []byte, error) {
	var object map[string]any
	raw, err := httpguard.DecodeJSONBytes(c, &object)
	return object, raw, err
}

func revision(value any) (int64, error) {
	number, ok := value.(json.Number)
	if !ok {
		return 0, bad()
	}
	n, err := number.Int64()
	if err != nil || n < 1 || n >= d.MaxRevision {
		return 0, bad()
	}
	return n, nil
}
func noBody(c *gin.Context) bool {
	if c.Request.Body == nil {
		return true
	}
	b, err := io.ReadAll(io.LimitReader(c.Request.Body, 1))
	return err == nil && len(b) == 0
}
func (h *Handler) get(c *gin.Context, kind d.Kind, param string) {
	p, ok := h.authenticate(c, false)
	if !ok {
		return
	}
	query, err := parseQuery(c)
	if err != nil || len(query) != 0 || !noBody(c) {
		response.Fail(c, bad())
		return
	}
	out, err := h.Catalog.Get(c.Request.Context(), p, kind, c.Param(param))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, 200, out)
}
func (h *Handler) write(c *gin.Context, kind d.Kind, param string) {
	p, ok := h.authenticate(c, true)
	if !ok {
		return
	}
	query, err := parseQuery(c)
	if err != nil || len(query) != 0 {
		response.Fail(c, bad())
		return
	}
	obj, body, err := readObject(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	id := c.Param(param)
	expected := int64(0)
	status := 201
	if c.Request.Method == "PUT" {
		expected, err = revision(obj["expected_revision"])
		if err != nil {
			response.Fail(c, err)
			return
		}
		delete(obj, "expected_revision")
		status = 200
	}
	raw, err := json.Marshal(obj)
	if err != nil {
		response.Fail(c, bad())
		return
	}
	logCatalogStep(c, "input_validated")
	h.command(c, p, body, status, func(ctx context.Context) (any, error) { return h.Catalog.Write(ctx, p, kind, id, expected, raw) })
}
func (h *Handler) delete(c *gin.Context, kind d.Kind, param string) {
	p, ok := h.authenticate(c, true)
	if !ok {
		return
	}
	query, err := parseQuery(c)
	if err != nil || len(query) != 1 || len(query["expected_revision"]) != 1 || !noBody(c) {
		response.Fail(c, bad())
		return
	}
	expected, err := strconv.ParseInt(query.Get("expected_revision"), 10, 64)
	if err != nil || expected < 1 || expected >= d.MaxRevision {
		response.Fail(c, bad())
		return
	}
	id := c.Param(param)
	logCatalogStep(c, "input_validated")
	h.command(c, p, []byte("{}"), 200, func(ctx context.Context) (any, error) {
		if err := h.Catalog.Delete(ctx, p, kind, id, expected); err != nil {
			return nil, err
		}
		return map[string]any{"id": id, "revision": expected + 1, "deleted": true}, nil
	})
}
func (h *Handler) command(c *gin.Context, p identity.Principal, body []byte, status int, fn func(context.Context) (any, error)) {
	requestID, traceID := response.IDs(c)
	req := idempotency.Request{TenantID: p.TenantID, ActorID: p.UserID, Method: c.Request.Method, Path: c.Request.URL.Path, Key: c.GetHeader("Idempotency-Key"), Body: body, Query: c.Request.URL.Query(), RequestID: requestID, TraceID: traceID}
	logCatalogStep(c, "idempotency_started")
	result, err := h.Idempotency.Execute(c.Request.Context(), req, func(ctx context.Context) error { _, err := h.Catalog.Authorize(ctx, p, true); return err }, func(ctx context.Context) (idempotency.CommandOutcome, error) {
		out, err := fn(ctx)
		if err != nil {
			return idempotency.CommandOutcome{}, err
		}
		raw, err := json.Marshal(out)
		return idempotency.CommandOutcome{Commit: true, Result: idempotency.Result{Status: status, Body: raw}}, err
	})
	if err != nil {
		logCatalogStep(c, "idempotency_failed")
		response.Fail(c, err)
		return
	}
	if result.Status >= 400 {
		var envelope response.ErrorEnvelope
		if json.Unmarshal(result.Body, &envelope) == nil && pe.IsContractStatus(envelope.Code, result.Status) {
			c.Set(response.ContextKeyErrorCode, envelope.Code)
		}
	}
	for key, value := range result.Headers() {
		c.Header(key, value)
	}
	logCatalogStep(c, "idempotency_finished")
	c.Data(result.Status, "application/json; charset=utf-8", result.Body)
}

// Budget uses the persisted ledger; GET never provisions a missing tenant.
func (h *Handler) budget(c *gin.Context) {
	p, ok := h.authenticate(c, c.Request.Method == "PUT")
	if !ok {
		return
	}
	query, err := parseQuery(c)
	if err != nil || len(query) != 0 {
		response.Fail(c, bad())
		return
	}
	if c.Request.Method == "GET" {
		if !noBody(c) {
			response.Fail(c, bad())
			return
		}
		out, err := h.Budget.Get(c.Request.Context(), p)
		if err != nil {
			response.Fail(c, err)
			return
		}
		response.OK(c, 200, out)
		return
	}
	obj, raw, err := readObject(c)
	if err != nil || len(obj) != 2 {
		response.Fail(c, bad())
		return
	}
	expected, err := revision(obj["expected_revision"])
	if err != nil {
		response.Fail(c, err)
		return
	}
	number, ok := obj["daily_limit_micros"].(json.Number)
	if !ok {
		response.Fail(c, bad())
		return
	}
	limit, err := number.Int64()
	if err != nil || limit < 0 || limit > d.MaxMicros {
		response.Fail(c, bad())
		return
	}
	h.command(c, p, raw, 200, func(ctx context.Context) (any, error) {
		return h.Budget.Configure(ctx, p, d.BudgetUpdate{ExpectedRevision: expected, DailyLimitMicros: limit})
	})
}

// Register mounts the 32 declared administration routes. Budget GET/PUT use the
// persisted accounting service; no Gateway or provider dispatch route is invented.
func (h *Handler) Register(engine *gin.Engine) {
	for _, entry := range []struct {
		kind  d.Kind
		param string
	}{{d.Providers, "provider_id"}, {d.Models, "model_id"}, {d.Channels, "channel_id"}, {d.Keys, "key_id"}, {d.Policies, "policy_id"}} {
		kind, param := entry.kind, entry.param
		path := "/v1/llm/" + string(kind)
		item := path + "/:" + param
		engine.GET(path, func(c *gin.Context) { h.list(c, kind) })
		engine.POST(path, func(c *gin.Context) { h.write(c, kind, "") })
		engine.GET(item, func(c *gin.Context) { h.get(c, kind, param) })
		engine.PUT(item, func(c *gin.Context) { h.write(c, kind, param) })
		engine.DELETE(item, func(c *gin.Context) { h.delete(c, kind, param) })
	}
	engine.GET("/v1/llm/pricing", func(c *gin.Context) { h.list(c, d.Prices) })
	engine.POST("/v1/llm/pricing", func(c *gin.Context) { h.write(c, d.Prices, "") })
	engine.GET("/v1/llm/pricing/:pricing_id", func(c *gin.Context) { h.get(c, d.Prices, "pricing_id") })
	engine.GET("/v1/llm/usage", func(c *gin.Context) { h.list(c, d.Usages) })
	engine.GET("/v1/llm/usage/:usage_id", func(c *gin.Context) { h.get(c, d.Usages, "usage_id") })
	engine.GET("/v1/llm/budget", h.budget)
	engine.PUT("/v1/llm/budget", h.budget)
	engine.POST("/v1/llm/upstream-models", h.fetchUpstreamModels)
}

func (h *Handler) fetchUpstreamModels(c *gin.Context) {
	p, ok := h.authenticate(c, true)
	if !ok {
		return
	}
	var input struct {
		Protocol string `json:"protocol"`
		BaseURL  string `json:"base_url"`
		APIKey   string `json:"api_key"`
	}
	if err := httpguard.DecodeJSON(c, &input); err != nil {
		response.Fail(c, bad())
		return
	}
	items, err := h.Catalog.FetchUpstreamModels(c.Request.Context(), p, d.UpstreamModelsQuery{
		Protocol: input.Protocol,
		BaseURL:  input.BaseURL,
		APIKey:   input.APIKey,
	})
	if err != nil && len(items) == 0 {
		// Avoid logging a caller-controlled URL or upstream response body: both
		// may contain query secrets or credential-bearing diagnostic text.
		logger.FromContext(c.Request.Context()).Warn("fetch_upstream_models_failed",
			"protocol", input.Protocol, "reason", "upstream_model_probe_failed")
		response.OK(c, 200, gin.H{
			"items": []any{},
		})
		return
	}
	if items == nil {
		items = make([]d.UpstreamModelItem, 0)
	}
	response.OK(c, 200, gin.H{
		"items": items,
	})
}

type anchor struct {
	At string `json:"at"`
	ID string `json:"id"`
}

func parseQuery(c *gin.Context) (url.Values, error) {
	q, err := url.ParseQuery(c.Request.URL.RawQuery)
	if err != nil {
		return nil, bad()
	}
	for _, values := range q {
		if len(values) != 1 {
			return nil, bad()
		}
	}
	return q, nil
}
func (h *Handler) list(c *gin.Context, kind d.Kind) {
	p, ok := h.authenticate(c, false)
	if !ok {
		return
	}
	values, err := parseQuery(c)
	if err != nil || !noBody(c) {
		response.Fail(c, bad())
		return
	}
	allowed := map[string]bool{"cursor": true, "limit": true}
	if kind == d.Prices || kind == d.Usages {
		allowed["model_id"] = true
	}
	if kind == d.Usages {
		for _, key := range []string{"date_from", "date_to", "channel_id", "status"} {
			allowed[key] = true
		}
	}
	for key := range values {
		if !allowed[key] {
			response.Fail(c, bad())
			return
		}
	}
	limit := 50
	if value, exists := values["limit"]; exists {
		limit, err = strconv.Atoi(value[0])
		if err != nil || limit < 1 || limit > 100 {
			response.Fail(c, bad())
			return
		}
	}
	q := d.ListQuery{Limit: limit + 1, ParentID: values.Get("model_id"), ChannelID: values.Get("channel_id"), Status: values.Get("status"), DateFrom: values.Get("date_from"), DateTo: values.Get("date_to")}
	for _, id := range []string{q.ParentID, q.ChannelID} {
		if id != "" && !scope.ValidID(id) {
			response.Fail(c, bad())
			return
		}
	}
	if q.Status != "" && q.Status != "succeeded" && q.Status != "failed" && q.Status != "cancelled" && q.Status != "unknown" {
		response.Fail(c, bad())
		return
	}
	if kind == d.Usages {
		now := timeutil.BeijingNow()
		if q.DateTo == "" {
			q.DateTo = now.Format(time.DateOnly)
		}
		to, err := time.ParseInLocation(time.DateOnly, q.DateTo, timeutil.BeijingLocation())
		if err != nil {
			response.Fail(c, bad())
			return
		}
		if q.DateFrom == "" {
			q.DateFrom = to.AddDate(0, 0, -30).Format(time.DateOnly)
		}
		from, err := time.ParseInLocation(time.DateOnly, q.DateFrom, timeutil.BeijingLocation())
		if err != nil || from.After(to) || to.Sub(from) > 30*24*time.Hour {
			response.Fail(c, bad())
			return
		}
	}
	filters := map[string]any{"limit": limit, "model_id": q.ParentID, "channel_id": q.ChannelID, "status": q.Status, "date_from": q.DateFrom, "date_to": q.DateTo}
	filterJSON, _ := json.Marshal(filters)
	sort := "updated_at desc,id desc"
	if kind == d.Prices || kind == d.Usages {
		sort = "created_at desc,id desc"
	}
	binding := cursor.Binding{TenantID: p.TenantID, ActorID: p.UserID, Audience: p.Audience, Operation: "GET /v1/llm/" + string(kind), PermissionRevision: p.MembershipRevision, Filters: filterJSON, Sort: sort}
	if token := values.Get("cursor"); token != "" {
		var key anchor
		if err = h.Cursor.Decode(token, binding, &key); err != nil {
			response.Fail(c, err)
			return
		}
		at, err := time.Parse(time.RFC3339Nano, key.At)
		if err != nil || at.IsZero() || !scope.ValidID(key.ID) {
			response.Fail(c, pe.Contract(400, "platform.cursor_invalid", nil))
			return
		}
		q.BeforeID = key.ID
		q.BeforeTime = key.At
	}
	rows, err := h.Catalog.List(c.Request.Context(), p, kind, q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	var next *string
	if hasMore {
		var last map[string]any
		if json.Unmarshal(rows[len(rows)-1], &last) != nil {
			response.Fail(c, pe.Contract(500, "platform.internal_error", nil))
			return
		}
		column := "updated_at"
		if kind == d.Prices || kind == d.Usages {
			column = "created_at"
		}
		at, _ := last[column].(string)
		id, _ := last["id"].(string)
		if at == "" || !scope.ValidID(id) {
			response.Fail(c, pe.Contract(500, "platform.internal_error", nil))
			return
		}
		token, err := h.Cursor.Encode(binding, anchor{at, id})
		if err != nil {
			response.Fail(c, err)
			return
		}
		next = &token
	}
	response.OK(c, 200, map[string]any{"items": rows, "has_more": hasMore, "next_cursor": next})
}
