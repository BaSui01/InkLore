// package http exposes only the frozen outline HTTP routes.
package http

import (
	a "InkLore/internal/outline/application"
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
type Handler struct{ Options }

func NewHandler(o Options) (*Handler, error) {
	if o.Service == nil || o.Identity == nil || o.Idempotency == nil || o.Cursor == nil {
		return nil, errors.New("outline: service, identity, idempotency and cursor required")
	}
	return &Handler{Options: o}, nil
}
func bad() error { return pe.Contract(400, "platform.validation_failed", nil) }
func (h *Handler) authenticate(c *gin.Context) (identity.Principal, bool) {
	header := c.GetHeader("Authorization")
	if !strings.HasPrefix(header, "Bearer ") || strings.Count(header, " ") != 1 {
		response.Fail(c, pe.Contract(401, "auth.unauthenticated", nil))
		return identity.Principal{}, false
	}
	p, err := h.Identity.Authenticate(c.Request.Context(), strings.TrimPrefix(header, "Bearer "), "creator")
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
	return p, true
}
func principal(p identity.Principal) scope.Principal {
	return scope.Principal{TenantID: p.TenantID, ActorID: p.UserID, SessionID: p.SessionID, Audience: p.Audience}
}
func decode[T any](c *gin.Context) (T, error) {
	var input T
	if err := httpguard.DecodeJSON(c, &input); err != nil {
		return input, err
	}
	return input, nil
}
func command[I, O any](h *Handler, c *gin.Context, kind, param string, status int, fn func(context.Context, scope.Principal, string, I) (O, error)) {
	p, ok := h.authenticate(c)
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
	runCommand(h, c, p, kind, c.Param(param), input, status, func(ctx context.Context) (any, error) { return fn(ctx, principal(p), c.Param(param), input) })
}
func runCommand(h *Handler, c *gin.Context, p identity.Principal, kind, id string, input any, status int, fn func(context.Context) (any, error)) {
	raw, err := json.Marshal(input)
	if err != nil {
		response.Fail(c, err)
		return
	}
	requestID, traceID := response.IDs(c)
	req := idempotency.Request{TenantID: p.TenantID, ActorID: p.UserID, Method: c.Request.Method, Path: c.Request.URL.Path, Key: c.GetHeader("Idempotency-Key"), Body: raw, Query: c.Request.URL.Query(), RequestID: requestID, TraceID: traceID}
	result, err := h.Idempotency.Execute(c.Request.Context(), req, func(ctx context.Context) error {
		if _, err := h.Identity.Authorize(ctx, p, "creator"); err != nil {
			return err
		}
		_, err := h.Service.AuthorizeCommand(ctx, principal(p), kind, id, c.Request.Method == "DELETE" || strings.HasSuffix(c.Request.URL.Path, "/restore"))
		return err
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
func get[O any](h *Handler, c *gin.Context, param string, fn func(context.Context, scope.Principal, string) (O, error)) {
	p, ok := h.authenticate(c)
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
