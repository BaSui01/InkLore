// Package transport serves the current worldview HTTP operations.
package transport

import (
	"InkLore/internal/platform/cursor"
	"InkLore/internal/platform/httpguard"
	"InkLore/internal/platform/idempotency"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/response"
	"InkLore/internal/platform/scope"
	"InkLore/internal/worldview/application"
	d "InkLore/internal/worldview/domain"
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type Options struct {
	Service     *application.Service
	Identity    *identity.Service
	Idempotency *idempotency.Executor
	Cursor      *cursor.Codec
}
type handler struct{ o Options }

func RegisterRoutes(engine *gin.Engine, o Options) error {
	if engine == nil || o.Service == nil || o.Identity == nil || o.Idempotency == nil || o.Cursor == nil {
		return errors.New("worldview: HTTP dependencies required")
	}
	if err := o.Service.ValidateOptions(); err != nil {
		return err
	}
	h := handler{o: o}
	for _, r := range routes {
		engine.Handle(r.method, r.path, h.serve(r))
	}
	return nil
}
func (h handler) serve(r route) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if !strings.HasPrefix(token, "Bearer ") {
			response.Fail(c, unauthenticated())
			return
		}
		principal, err := h.o.Identity.Authenticate(c.Request.Context(), strings.TrimPrefix(token, "Bearer "), "creator")
		if err != nil {
			response.Fail(c, err)
			return
		}
		p := scope.Principal{TenantID: principal.TenantID, ActorID: principal.UserID, SessionID: principal.SessionID, Audience: principal.Audience}
		for _, param := range c.Params {
			if !scope.ValidID(param.Value) {
				response.Fail(c, d.Malformed())
				return
			}
		}
		for key, values := range c.Request.URL.Query() {
			if len(values) != 1 || !d.One(key, r.query...) {
				response.Fail(c, d.Malformed())
				return
			}
		}
		if r.method == "GET" {
			value, err := h.get(c, r, principal, p)
			if err != nil {
				response.Fail(c, err)
				return
			}
			response.OK(c, 200, value)
			return
		}
		body, command, err := h.command(c, r, p)
		if err != nil {
			response.Fail(c, err)
			return
		}
		kind, id := target(c)
		requestID, traceID := response.IDs(c)
		request := idempotency.Request{TenantID: p.TenantID, ActorID: p.ActorID, Method: r.method, Path: c.Request.URL.Path, Key: c.GetHeader("Idempotency-Key"), Body: body, Query: c.Request.URL.Query(), RequestID: requestID, TraceID: traceID}
		authorize := func(ctx context.Context) error {
			if _, err := h.o.Identity.Authorize(ctx, principal, "creator"); err != nil {
				return err
			}
			return h.o.Service.AuthorizeReplay(ctx, p, kind, id, r.operation != "checkFacts", r.method == "DELETE" || strings.HasPrefix(r.operation, "restore"))
		}
		execute := h.o.Idempotency.Execute
		if r.operation == "checkFacts" {
			execute = h.o.Idempotency.ExecuteReadConsistent
		}
		result, err := execute(c.Request.Context(), request, authorize, func(ctx context.Context) (idempotency.CommandOutcome, error) {
			value, err := command(ctx)
			if err != nil {
				return idempotency.CommandOutcome{}, err
			}
			raw, err := json.Marshal(value)
			status := 200
			if strings.HasPrefix(r.operation, "create") {
				status = 201
			}
			return idempotency.CommandOutcome{Commit: true, Result: idempotency.Result{Status: status, Body: raw}}, err
		})
		if err != nil {
			response.Fail(c, err)
			return
		}
		for key, value := range result.Headers() {
			c.Header(key, value)
		}
		c.Data(result.Status, "application/json; charset=utf-8", result.Body)
	}
}
func target(c *gin.Context) (string, string) {
	for _, kind := range []string{"novel", "character", "lore", "relation", "fact"} {
		if id := c.Param(kind + "_id"); id != "" {
			return kind, id
		}
	}
	return "", ""
}
func decode[T any](c *gin.Context) (T, json.RawMessage, error) {
	var in T
	raw, err := httpguard.DecodeJSONBytes(c, &in)
	if err != nil {
		return in, nil, err
	}
	if valid, ok := any(in).(interface{ Validate() error }); ok {
		if err := valid.Validate(); err != nil {
			return in, nil, err
		}
	}
	return in, raw, nil
}
func integer(c *gin.Context, key string, def, min, max int) (int, error) {
	text, ok := c.GetQuery(key)
	if !ok {
		return def, nil
	}
	if text == "" {
		return 0, d.Malformed()
	}
	for _, r := range text {
		if r < '0' || r > '9' {
			return 0, d.Malformed()
		}
	}
	n, err := strconv.Atoi(text)
	if err != nil || n < min || n > max {
		return 0, d.Malformed()
	}
	return n, nil
}
