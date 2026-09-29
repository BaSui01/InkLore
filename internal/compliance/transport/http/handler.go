package http

import (
	a "InkLore/internal/compliance/application"
	d "InkLore/internal/compliance/domain"
	"InkLore/internal/platform/cursor"
	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/httpguard"
	"InkLore/internal/platform/idempotency"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/response"
	"InkLore/internal/platform/scope"
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
)

type Options struct {
	Service     *a.Service
	Identity    *identity.Service
	Idempotency *idempotency.Executor
	Cursor      *cursor.Codec
}

type Handler struct {
	Options
}

func NewHandler(o Options) (*Handler, error) {
	if o.Service == nil || o.Identity == nil || o.Idempotency == nil || o.Cursor == nil {
		return nil, errors.New("compliance: service, identity, idempotency and cursor required")
	}
	return &Handler{Options: o}, nil
}

func bad() error {
	return pe.Contract(400, "platform.validation_failed", nil)
}

func (h *Handler) authenticate(c *gin.Context, audience, role string) (identity.Principal, bool) {
	header := c.GetHeader("Authorization")
	if !strings.HasPrefix(header, "Bearer ") || strings.Count(header, " ") != 1 {
		response.Fail(c, pe.Contract(401, "auth.unauthenticated", nil))
		return identity.Principal{}, false
	}
	token := strings.TrimPrefix(header, "Bearer ")
	p, err := h.Identity.Authenticate(c.Request.Context(), token, audience)
	if err != nil {
		response.Fail(c, err)
		return identity.Principal{}, false
	}

	hasRole := false
	for _, r := range p.Roles {
		if r == role {
			hasRole = true
			break
		}
	}
	if !hasRole {
		response.Fail(c, pe.Contract(403, "auth.forbidden", nil))
		return identity.Principal{}, false
	}

	for _, param := range c.Params {
		if !scope.ValidID(param.Value) {
			response.Fail(c, bad())
			return identity.Principal{}, false
		}
	}
	return p, true
}

func principal(p identity.Principal) scope.Principal {
	return scope.Principal{
		TenantID:  p.TenantID,
		ActorID:   p.UserID,
		SessionID: p.SessionID,
		Audience:  p.Audience,
	}
}

func decode[T any](c *gin.Context) (T, error) {
	var input T
	err := httpguard.DecodeJSON(c, &input)
	return input, err
}

func runCommand(h *Handler, c *gin.Context, p identity.Principal, role string, input any, status int, fn func(context.Context) (any, error)) {
	raw, err := json.Marshal(input)
	if err != nil {
		response.Fail(c, err)
		return
	}
	requestID, traceID := response.IDs(c)
	req := idempotency.Request{
		TenantID:  p.TenantID,
		ActorID:   p.UserID,
		Method:    c.Request.Method,
		Path:      c.Request.URL.Path,
		Key:       c.GetHeader("Idempotency-Key"),
		Body:      raw,
		Query:     c.Request.URL.Query(),
		RequestID: requestID,
		TraceID:   traceID,
	}
	result, err := h.Idempotency.Execute(c.Request.Context(), req, func(ctx context.Context) error {
		if _, err := h.Identity.Authorize(ctx, p, role); err != nil {
			return err
		}
		return nil
	}, func(ctx context.Context) (idempotency.CommandOutcome, error) {
		out, err := fn(ctx)
		if err != nil {
			return idempotency.CommandOutcome{}, err
		}
		data, err := json.Marshal(out)
		return idempotency.CommandOutcome{Commit: true, Result: idempotency.Result{Status: status, Body: data}}, err
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	for k, v := range result.Headers() {
		c.Header(k, v)
	}
	c.Data(result.Status, "application/json; charset=utf-8", result.Body)
}

func command[I, O any](h *Handler, c *gin.Context, audience, role string, status int, fn func(context.Context, scope.Principal, I) (O, error)) {
	p, ok := h.authenticate(c, audience, role)
	if !ok {
		return
	}
	input, err := decode[I](c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	if len(c.Request.URL.Query()) != 0 {
		response.Fail(c, bad())
		return
	}
	runCommand(h, c, p, role, input, status, func(ctx context.Context) (any, error) {
		return fn(ctx, principal(p), input)
	})
}

func get[O any](h *Handler, c *gin.Context, audience, role, param string, fn func(context.Context, scope.Principal, string) (O, error)) {
	p, ok := h.authenticate(c, audience, role)
	if !ok {
		return
	}
	if len(c.Request.URL.Query()) != 0 {
		response.Fail(c, bad())
		return
	}
	out, err := fn(c.Request.Context(), principal(p), c.Param(param))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, 200, out)
}

func remove[O any](h *Handler, c *gin.Context, audience, role string, fn func(context.Context, scope.Principal, string, string, int64) (O, error)) {
	p, ok := h.authenticate(c, audience, role)
	if !ok {
		return
	}
	q := c.Request.URL.Query()
	if len(q) != 1 {
		response.Fail(c, bad())
		return
	}
	revision, err := httpguard.QueryRevision(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	input := d.Expected{ExpectedRevision: revision}
	runCommand(h, c, p, role, input, 200, func(ctx context.Context) (any, error) {
		return fn(ctx, principal(p), c.Param("rule_set_id"), c.Param("rule_id"), revision)
	})
}
