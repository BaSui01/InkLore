// package transport exposes only the INK-163 frozen content HTTP operations.
package transport

import (
	a "InkLore/internal/content/application"
	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/httpguard"
	"InkLore/internal/platform/idempotency"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/response"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type Handler struct{ s *a.Service }

func New(s *a.Service) *Handler { return &Handler{s: s} }
func invalid() error            { return pe.Contract(400, "platform.validation_failed", nil) }
func (h *Handler) principal(c *gin.Context) (identity.Principal, error) {
	header := c.GetHeader("Authorization")
	if !strings.HasPrefix(header, "Bearer ") || len(strings.TrimPrefix(header, "Bearer ")) == 0 {
		return identity.Principal{}, pe.Contract(401, "auth.unauthenticated", nil)
	}
	return h.s.Authenticate(c.Request.Context(), strings.TrimPrefix(header, "Bearer "))
}
func parameters(c *gin.Context, allowed ...string) error {
	for _, p := range c.Params {
		if !httpguard.UUIDv4(p.Value) {
			return invalid()
		}
	}
	set := map[string]bool{}
	for _, key := range allowed {
		set[key] = true
	}
	for k, v := range c.Request.URL.Query() {
		if !set[k] || len(v) != 1 {
			return invalid()
		}
	}
	return nil
}

func decode[T any](c *gin.Context) (T, error) {
	var value T
	if err := httpguard.DecodeJSON(c, &value); err != nil {
		return value, err
	}
	return value, nil
}
func listQuery(c *gin.Context) (a.ListQuery, error) {
	q := a.ListQuery{Limit: 20, Cursor: c.Query("cursor"), Status: c.Query("status")}
	if values, ok := c.Request.URL.Query()["status"]; ok && values[0] == "" {
		return q, invalid()
	}
	if raw, ok := c.GetQuery("limit"); ok {
		n, e := strconv.Atoi(raw)
		if e != nil || n < 1 || n > 100 {
			return q, invalid()
		}
		q.Limit = n
	}
	if values, ok := c.Request.URL.Query()["limit"]; ok && values[0] == "" {
		return q, invalid()
	}
	if values, ok := c.Request.URL.Query()["deleted"]; ok {
		if values[0] != "true" && values[0] != "false" {
			return q, invalid()
		}
		q.Deleted = values[0] == "true"
	}
	return q, nil
}
func command(c *gin.Context, p identity.Principal) a.Command {
	rid, tid := response.IDs(c)
	return a.Command{Principal: p, Request: idempotency.Request{TenantID: p.TenantID, ActorID: p.UserID, Method: c.Request.Method, Path: c.Request.URL.Path, Key: c.GetHeader("Idempotency-Key"), Query: c.Request.URL.Query(), RequestID: rid, TraceID: tid}}
}
func writeResult(c *gin.Context, result idempotency.Result, e error) {
	if e != nil {
		response.Fail(c, e)
		return
	}
	for k, v := range result.Headers() {
		c.Header(k, v)
	}
	c.Data(result.Status, "application/json; charset=utf-8", result.Body)
}
func readRoute(h *Handler, allowed []string, fn func(*gin.Context, identity.Principal) (any, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, e := h.principal(c)
		if e != nil {
			response.Fail(c, e)
			return
		}
		if e = parameters(c, allowed...); e != nil {
			response.Fail(c, e)
			return
		}
		out, e := fn(c, p)
		if e != nil {
			response.Fail(c, e)
			return
		}
		response.OK(c, 200, out)
	}
}
func writeRoute[T any](h *Handler, fn func(*gin.Context, a.Command, T) (idempotency.Result, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, e := h.principal(c)
		if e != nil {
			response.Fail(c, e)
			return
		}
		if e = parameters(c); e != nil {
			response.Fail(c, e)
			return
		}
		if !httpguard.UUIDv4(c.GetHeader("Idempotency-Key")) {
			response.Fail(c, invalid())
			return
		}
		r, e := decode[T](c)
		if e != nil {
			response.Fail(c, e)
			return
		}
		result, e := fn(c, command(c, p), r)
		writeResult(c, result, e)
	}
}
func deleteRoute(h *Handler, fn func(*gin.Context, a.Command, a.RevisionCommand) (idempotency.Result, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, e := h.principal(c)
		if e != nil {
			response.Fail(c, e)
			return
		}
		if e = parameters(c, "expected_revision"); e != nil {
			response.Fail(c, e)
			return
		}
		if !httpguard.UUIDv4(c.GetHeader("Idempotency-Key")) {
			response.Fail(c, invalid())
			return
		}
		revision, e := httpguard.QueryRevision(c)
		if e != nil {
			response.Fail(c, e)
			return
		}
		reader := http.MaxBytesReader(c.Writer, c.Request.Body, 2<<20)
		defer func() { _ = reader.Close() }()
		raw, bodyErr := io.ReadAll(reader)
		if bodyErr != nil {
			response.Fail(c, pe.Contract(413, "platform.request_too_large", nil))
			return
		}
		if len(raw) > 0 {
			response.Fail(c, invalid())
			return
		}
		result, e := fn(c, command(c, p), a.RevisionCommand{ExpectedRevision: revision})
		writeResult(c, result, e)
	}
}
