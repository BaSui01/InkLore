package http

import (
	a "InkLore/internal/agent/application"
	d "InkLore/internal/agent/domain"
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
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// HTTPOptions configures the HTTP handler for agent governance and runtime.
type HTTPOptions struct {
	Service     *a.GovernanceService
	Runs        *a.RunService
	Approvals   *a.ApprovalService
	Events      *a.EventService
	Artifacts   *a.ArtifactService
	NovelGuide  *a.NovelGuideService
	Identity    *identity.Service
	Idempotency *idempotency.Executor
	Cursor      *cursor.Codec
}

type Options = HTTPOptions

// Handler mounts agent governance and runtime endpoints onto Gin.
type Handler struct {
	HTTPOptions
}

// NewHandler creates a new HTTP handler.
func NewHandler(o HTTPOptions) (*Handler, error) {
	if o.Service == nil || o.Runs == nil || o.Approvals == nil || o.Events == nil || o.Artifacts == nil || o.NovelGuide == nil || o.Identity == nil || o.Idempotency == nil || o.Cursor == nil {
		return nil, errors.New("agent transport: service, runs, approvals, events, artifacts, identity, idempotency and cursor required")
	}
	return &Handler{HTTPOptions: o}, nil
}

func bad() error {
	return pe.Contract(400, "platform.validation_failed", nil)
}

func (h *Handler) authenticate(c *gin.Context, audience string, write bool) (identity.Principal, bool) {
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

	if audience == "admin" {
		if h.Service != nil {
			if err := h.Service.AuthorizeAdmin(p, write); err != nil {
				response.Fail(c, err)
				return p, false
			}
		} else {
			hasRole := false
			for _, r := range p.Roles {
				if r == "operator" || (!write && r == "auditor") {
					hasRole = true
					break
				}
			}
			if !hasRole {
				response.Fail(c, pe.Contract(403, "auth.forbidden", nil))
				return p, false
			}
		}
	} else if audience == "creator" {
		if h.Service != nil {
			if err := h.Service.AuthorizeCreator(p); err != nil {
				response.Fail(c, err)
				return p, false
			}
		} else if h.Runs != nil {
			if err := h.Runs.AuthorizeCreator(p); err != nil {
				response.Fail(c, err)
				return p, false
			}
		} else {
			hasRole := false
			for _, r := range p.Roles {
				if r == "creator" {
					hasRole = true
					break
				}
			}
			if !hasRole {
				response.Fail(c, pe.Contract(403, "auth.forbidden", nil))
				return p, false
			}
		}
	}

	for _, param := range c.Params {
		if !scope.ValidID(param.Value) {
			response.Fail(c, bad())
			return p, false
		}
	}

	return p, true
}

func readStrictJSON[T any](c *gin.Context) (T, []byte, error) {
	var target T
	raw, err := httpguard.DecodeJSONBytes(c, &target)
	return target, raw, err
}

func parsePagination(c *gin.Context) (d.ListQuery, error) {
	limit := 20
	if limitStr := c.Query("limit"); limitStr != "" {
		parsed, err := strconv.Atoi(limitStr)
		if err != nil || parsed < 1 || parsed > 100 {
			return d.ListQuery{}, bad()
		}
		limit = parsed
	}

	cursorStr := c.Query("cursor")
	return d.ListQuery{
		Limit:      limit,
		BeforeTime: "",
		BeforeID:   cursorStr,
	}, nil
}

func parseQueryRevision(c *gin.Context) (int64, error) {
	if err := httpguard.RequireEmptyBody(c); err != nil {
		return 0, err
	}
	for name := range c.Request.URL.Query() {
		if name != "expected_revision" {
			return 0, bad()
		}
	}
	return httpguard.QueryRevision(c)
}

type commandAuthorizer func(context.Context, identity.Principal) error

func (h *Handler) command(c *gin.Context, p identity.Principal, body []byte, status int, authorize commandAuthorizer, fn func(context.Context) (any, error)) {
	key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if !scope.ValidID(key) {
		response.Fail(c, bad())
		return
	}
	requestID, traceID := response.IDs(c)
	req := idempotency.Request{
		TenantID:  p.TenantID,
		ActorID:   p.UserID,
		Method:    c.Request.Method,
		Path:      c.Request.URL.Path,
		Key:       key,
		Body:      body,
		Query:     c.Request.URL.Query(),
		RequestID: requestID,
		TraceID:   traceID,
	}
	result, err := h.Idempotency.Execute(c.Request.Context(), req, func(ctx context.Context) error {
		role := "creator"
		if p.Audience == "admin" {
			role = "operator"
		}
		current, err := h.Identity.Authorize(ctx, p, role)
		if err != nil {
			return err
		}
		return authorize(ctx, current)
	}, func(ctx context.Context) (idempotency.CommandOutcome, error) {
		out, err := fn(ctx)
		if err != nil {
			return idempotency.CommandOutcome{}, err
		}
		raw, err := json.Marshal(out)
		return idempotency.CommandOutcome{Commit: true, Result: idempotency.Result{Status: status, Body: raw}}, err
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

func parseSingleQuery(c *gin.Context) (url.Values, error) {
	q, err := url.ParseQuery(c.Request.URL.RawQuery)
	if err != nil {
		return nil, bad()
	}
	for _, vals := range q {
		if len(vals) != 1 {
			return nil, bad()
		}
	}
	return q, nil
}
