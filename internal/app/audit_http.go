package app

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"InkLore/internal/platform/audit"
	"InkLore/internal/platform/cursor"
	platformerr "InkLore/internal/platform/errors"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/logger"
	"InkLore/internal/platform/membership"
	"InkLore/internal/platform/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (a *App) registerAuditRoutes(engine *gin.Engine) {
	engine.GET("/v1/platform/audit-events", func(c *gin.Context) {
		p, ok := a.authenticate(c, "admin")
		if !ok {
			return
		}
		values := c.Request.URL.Query()
		for _, key := range []string{"limit", "cursor", "from", "to", "action"} {
			if len(values[key]) > 1 {
				response.Fail(c, platformerr.Contract(400, "platform.validation_failed", nil))
				return
			}
		}
		input := audit.GETQuery{From: c.Query("from"), To: c.Query("to"), Action: c.Query("action"), Limit: c.Query("limit"), Cursor: c.Query("cursor")}
		query, err := audit.ParseGETQuery(input)
		if err != nil {
			response.Fail(c, err)
			return
		}
		filters, _ := json.Marshal(map[string]string{"from": input.From, "to": input.To, "action": input.Action})
		binding := cursor.Binding{TenantID: p.TenantID, ActorID: p.UserID, Audience: p.Audience, PermissionRevision: p.MembershipRevision, Operation: "platformListAudit", Sort: "created_at:desc,id:desc", Filters: filters}
		if input.Cursor != "" {
			var key pageAnchor
			if err = a.platform.Cursor.Decode(input.Cursor, binding, &key); err != nil {
				response.Fail(c, err)
				return
			}
			query.BeforeCreatedAt = &key.At
			query.BeforeID = key.ID
		}
		var page audit.Page
		err = a.platform.UoW.Within(c.Request.Context(), func(ctx context.Context) error {
			current, err := a.platform.Identity.Authorize(ctx, p, "operator", "auditor")
			if err != nil {
				return err
			}
			page, err = a.platform.Audit.List(ctx, audit.VerifiedReader{TenantID: current.TenantID, ActorID: current.UserID, Audience: current.Audience, Roles: current.Roles}, query)
			return err
		})
		if err != nil {
			response.Fail(c, err)
			return
		}
		output := listResult[audit.Audit]{Items: page.Items, HasMore: page.HasMore}
		if page.HasMore && len(page.Items) > 0 {
			last := page.Items[len(page.Items)-1]
			token, err := a.platform.Cursor.Encode(binding, pageAnchor{At: last.CreatedAt, ID: last.ID})
			if err != nil {
				response.Fail(c, err)
				return
			}
			output.NextCursor = &token
		}
		response.OK(c, 200, output)
	})
}
func correlation(ctx context.Context) (string, string) {
	requestID, _ := ctx.Value(logger.RequestIDKey).(string)
	traceID, _ := ctx.Value(logger.TraceIDKey).(string)
	if requestID == "" {
		requestID = uuid.NewString()
	}
	if traceID == "" {
		traceID = strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	return requestID, traceID
}
func identityAudit(store *audit.Store) func(context.Context, identity.Event) error {
	return func(ctx context.Context, event identity.Event) error {
		p := event.Principal
		if p.TenantID == "" || p.UserID == "" {
			return nil
		}
		requestID, traceID := correlation(ctx)
		var resourceID *string
		if event.ResourceID != "" {
			resourceID = &event.ResourceID
		} else if p.SessionID != "" {
			resourceID = &p.SessionID
		}
		var changes []audit.SafeChange
		if p.SessionID != "" {
			changes = []audit.SafeChange{{Field: "session_id", After: &p.SessionID}}
		}
		_, err := store.Append(ctx, audit.Record{
			TenantID:    p.TenantID,
			ActorID:     &p.UserID,
			Audience:    p.Audience,
			Action:      event.Action,
			Outcome:     event.Outcome,
			ResourceID:  resourceID,
			RequestID:   requestID,
			TraceID:     traceID,
			SafeChanges: changes,
		})
		return err
	}
}
func (a *App) auditMemberCommand(ctx context.Context, p identity.Principal, path, id string, value any) error {
	action := audit.ActionUpdateMember
	changes := []audit.SafeChange{}
	switch result := value.(type) {
	case membership.Member:
		if id == "" {
			action = audit.ActionCreateMember
			id = result.ID
		}
		roles, err := audit.RolesValue(result.Roles)
		if err != nil {
			return err
		}
		revision := strconv.FormatInt(result.Revision, 10)
		changes = append(changes, audit.SafeChange{Field: "roles", After: &roles}, audit.SafeChange{Field: "status", After: &result.Status}, audit.SafeChange{Field: "revision", After: &revision}, audit.SafeChange{Field: "user_id", After: &result.UserID})
	case membership.RevokeSessions:
		action = audit.ActionRevokeSessions
		if strings.HasSuffix(path, "/reset-password") {
			action = audit.ActionResetPassword
		}
		count := strconv.FormatInt(result.RevokedCount, 10)
		revision := strconv.FormatInt(result.Revision, 10)
		changes = append(changes, audit.SafeChange{Field: "revoked_count", After: &count}, audit.SafeChange{Field: "revision", After: &revision})
	}
	requestID, traceID := correlation(ctx)
	_, err := a.platform.Audit.Append(ctx, audit.Record{TenantID: p.TenantID, ActorID: &p.UserID, Audience: p.Audience, Action: action, Outcome: "succeeded", ResourceID: &id, RequestID: requestID, TraceID: traceID, SafeChanges: changes})
	return err
}
