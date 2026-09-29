package app

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"InkLore/internal/platform/cursor"
	platformerr "InkLore/internal/platform/errors"
	"InkLore/internal/platform/logger"
	"InkLore/internal/platform/response"

	"github.com/gin-gonic/gin"
)

// registerProjectLogRoutes exposes only the bounded, safe operational projection.
// Unlike append-only audit facts, these rows can be dropped on write failure.
func (a *App) registerProjectLogRoutes(engine *gin.Engine) {
	engine.GET("/v1/platform/project-logs", func(c *gin.Context) {
		principal, ok := a.authenticate(c, "admin")
		if !ok {
			return
		}
		params := c.Request.URL.Query()
		for key, values := range params {
			switch key {
			case "limit", "cursor", "level", "domain", "operation", "from", "to", "search":
			default:
				response.Fail(c, platformerr.Contract(400, "platform.validation_failed", nil))
				return
			}
			if len(values) != 1 {
				response.Fail(c, platformerr.Contract(400, "platform.validation_failed", nil))
				return
			}
		}
		q := logger.ProjectLogQuery{Limit: 20, Level: params.Get("level"), Domain: params.Get("domain"), Operation: params.Get("operation"), Search: params.Get("search")}
		if raw := params.Get("limit"); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 1 || n > 100 {
				response.Fail(c, platformerr.Contract(400, "platform.validation_failed", nil))
				return
			}
			q.Limit = n
		}
		if q.Level != "" && q.Level != "DEBUG" && q.Level != "INFO" && q.Level != "WARN" && q.Level != "ERROR" ||
			len(q.Domain) > 80 || len(q.Operation) > 160 || len(q.Search) > 800 || len([]rune(q.Search)) > 200 || strings.ContainsAny(q.Search, "\x00\r\n") {
			response.Fail(c, platformerr.Contract(400, "platform.validation_failed", nil))
			return
		}
		for _, field := range []struct {
			value  string
			target **time.Time
		}{{params.Get("from"), &q.From}, {params.Get("to"), &q.To}} {
			if field.value == "" {
				continue
			}
			t, err := time.Parse(time.RFC3339Nano, field.value)
			if err != nil || !strings.HasSuffix(field.value, "Z") {
				response.Fail(c, platformerr.Contract(400, "platform.validation_failed", nil))
				return
			}
			t = t.UTC()
			*field.target = &t
		}
		if q.From != nil && q.To != nil && q.From.After(*q.To) {
			response.Fail(c, platformerr.Contract(400, "platform.validation_failed", nil))
			return
		}
		filters, _ := json.Marshal(map[string]string{"level": q.Level, "domain": q.Domain, "operation": q.Operation, "from": params.Get("from"), "to": params.Get("to"), "search": q.Search})
		binding := cursor.Binding{TenantID: principal.TenantID, ActorID: principal.UserID, Audience: principal.Audience, PermissionRevision: principal.MembershipRevision, Operation: "platformListProjectLogs", Sort: "created_at:desc,id:desc", Filters: filters}
		if token := params.Get("cursor"); token != "" {
			var anchor pageAnchor
			if err := a.platform.Cursor.Decode(token, binding, &anchor); err != nil {
				response.Fail(c, err)
				return
			}
			q.BeforeAt = &anchor.At
			q.BeforeID = anchor.ID
		} else if _, has := params["cursor"]; has {
			response.Fail(c, platformerr.Contract(400, "platform.cursor_invalid", nil))
			return
		}
		var page logger.ProjectLogPage
		// Log retrieval is operational tooling: a stalled PostgreSQL read must not
		// pin a request indefinitely, and errors must remain visible to the client.
		readCtx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()
		err := a.platform.UoW.Within(readCtx, func(ctx context.Context) error {
			current, err := a.platform.Identity.Authorize(ctx, principal, "operator", "auditor")
			if err != nil {
				return err
			}
			page, err = a.platform.ProjectLogs.List(ctx, logger.ProjectLogReader{TenantID: current.TenantID, ActorID: current.UserID, Audience: current.Audience, Roles: current.Roles}, q)
			return err
		})
		if err != nil {
			response.Fail(c, err)
			return
		}
		out := listResult[logger.ProjectLog]{Items: page.Items, HasMore: page.HasMore}
		if page.HasMore && len(page.Items) > 0 {
			last := page.Items[len(page.Items)-1]
			next, err := a.platform.Cursor.Encode(binding, pageAnchor{At: last.CreatedAt, ID: last.ID})
			if err != nil {
				response.Fail(c, err)
				return
			}
			out.NextCursor = &next
		}
		response.OK(c, 200, out)
	})
}
