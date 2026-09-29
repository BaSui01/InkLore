package app

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"InkLore/internal/platform/cursor"
	platformerr "InkLore/internal/platform/errors"
	"InkLore/internal/platform/httpguard"
	"InkLore/internal/platform/idempotency"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/membership"
	"InkLore/internal/platform/response"

	"github.com/gin-gonic/gin"
)

type pageAnchor struct {
	At time.Time `json:"at"`
	ID string    `json:"id"`
}
type listResult[T any] struct {
	Items      []T     `json:"items"`
	NextCursor *string `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
}

func queryLimit(c *gin.Context) (int, error) {
	values, exists := c.Request.URL.Query()["limit"]
	if !exists {
		return 20, nil
	}
	if len(values) != 1 {
		return 0, platformerr.Contract(400, "platform.validation_failed", nil)
	}
	limit, err := strconv.Atoi(values[0])
	if err != nil || limit < 1 || limit > 100 {
		return 0, platformerr.Contract(400, "platform.validation_failed", nil)
	}
	return limit, nil
}
func memberBinding(p identity.Principal) cursor.Binding {
	return cursor.Binding{TenantID: p.TenantID, ActorID: p.UserID, Audience: p.Audience, PermissionRevision: p.MembershipRevision, Operation: "platformListMembers", Sort: "updated_at:desc,id:desc"}
}
func (a *App) registerMembershipRoutes(engine *gin.Engine) {
	service := a.platform.Members
	engine.GET("/v1/platform/members", func(c *gin.Context) {
		p, ok := a.authenticate(c, "admin")
		if !ok {
			return
		}
		limit, err := queryLimit(c)
		if err != nil {
			response.Fail(c, err)
			return
		}
		query := membership.ListQuery{Limit: limit}
		if token := c.Query("cursor"); token != "" {
			var key pageAnchor
			if err := a.platform.Cursor.Decode(token, memberBinding(p), &key); err != nil {
				response.Fail(c, err)
				return
			}
			query.BeforeUpdatedAt = key.At
			query.BeforeID = key.ID
		}
		page, err := service.List(c.Request.Context(), p, query)
		if err != nil {
			response.Fail(c, err)
			return
		}
		output := listResult[membership.Member]{Items: page.Items, HasMore: page.HasMore}
		if page.HasMore && len(page.Items) > 0 {
			last := page.Items[len(page.Items)-1]
			token, err := a.platform.Cursor.Encode(memberBinding(p), pageAnchor{At: last.UpdatedAt, ID: last.ID})
			if err != nil {
				response.Fail(c, err)
				return
			}
			output.NextCursor = &token
		}
		response.OK(c, 200, output)
	})
	engine.GET("/v1/platform/members/:member_id", func(c *gin.Context) {
		p, ok := a.authenticate(c, "admin")
		if !ok {
			return
		}
		id := c.Param("member_id")
		if !httpguard.UUIDv4(id) {
			response.Fail(c, platformerr.Contract(400, "platform.validation_failed", nil))
			return
		}
		member, err := service.Get(c.Request.Context(), p, id)
		if err != nil {
			response.Fail(c, err)
			return
		}
		response.OK(c, 200, member)
	})
	engine.POST("/v1/platform/members", func(c *gin.Context) {
		var input membership.CreateMember
		if err := httpguard.DecodeJSON(c, &input); err != nil {
			response.Fail(c, err)
			return
		}
		a.memberCommand(c, input, 201, func(ctx context.Context, p identity.Principal) (any, error) { return service.Create(ctx, p, input) })
	})
	engine.PUT("/v1/platform/members/:member_id", func(c *gin.Context) {
		var input membership.UpdateMember
		if err := httpguard.DecodeJSON(c, &input); err != nil {
			response.Fail(c, err)
			return
		}
		a.memberCommand(c, input, 200, func(ctx context.Context, p identity.Principal) (any, error) {
			return service.Update(ctx, p, c.Param("member_id"), input)
		})
	})
	engine.POST("/v1/platform/members/:member_id/reset-password", func(c *gin.Context) {
		var input membership.ResetPassword
		if err := httpguard.DecodeJSON(c, &input); err != nil {
			response.Fail(c, err)
			return
		}
		a.memberCommand(c, input, 200, func(ctx context.Context, p identity.Principal) (any, error) {
			return service.ResetPassword(ctx, p, c.Param("member_id"), input)
		})
	})
	engine.POST("/v1/platform/members/:member_id/revoke-sessions", func(c *gin.Context) {
		var input membership.RevisionCommand
		if err := httpguard.DecodeJSON(c, &input); err != nil {
			response.Fail(c, err)
			return
		}
		a.memberCommand(c, input, 200, func(ctx context.Context, p identity.Principal) (any, error) {
			return service.RevokeSessions(ctx, p, c.Param("member_id"), input)
		})
	})
	engine.DELETE("/v1/platform/members/:member_id", func(c *gin.Context) {
		p, ok := a.authenticate(c, "admin")
		if !ok {
			return
		}
		id := c.Param("member_id")
		if !httpguard.UUIDv4(id) {
			response.Fail(c, platformerr.Contract(400, "platform.validation_failed", nil))
			return
		}
		if err := service.Delete(c.Request.Context(), p, id); err != nil {
			response.Fail(c, err)
			return
		}
		response.OK(c, 200, gin.H{"deleted": true})
	})
}
func (a *App) memberCommand(c *gin.Context, input any, status int, command func(context.Context, identity.Principal) (any, error)) {
	p, ok := a.authenticate(c, "admin")
	if !ok {
		return
	}
	if id := c.Param("member_id"); id != "" && !httpguard.UUIDv4(id) {
		response.Fail(c, platformerr.Contract(400, "platform.validation_failed", nil))
		return
	}
	raw, err := json.Marshal(input)
	if err != nil {
		response.Fail(c, err)
		return
	}
	requestID, traceID := response.IDs(c)
	request := idempotency.Request{TenantID: p.TenantID, ActorID: p.UserID, Method: c.Request.Method, Path: c.Request.URL.Path, Key: c.GetHeader("Idempotency-Key"), Body: raw, RequestID: requestID, TraceID: traceID}
	result, err := a.platform.Idempotency.Execute(c.Request.Context(), request, func(ctx context.Context) error {
		if id := c.Param("member_id"); id != "" {
			return a.platform.Members.AuthorizeMember(ctx, p, id, true)
		}
		return a.platform.Members.Authorize(ctx, p, true)
	}, func(ctx context.Context) (idempotency.CommandOutcome, error) {
		value, err := command(ctx, p)
		if err != nil {
			return idempotency.CommandOutcome{}, err
		}
		if err = a.auditMemberCommand(ctx, p, c.Request.URL.Path, c.Param("member_id"), value); err != nil {
			return idempotency.CommandOutcome{}, err
		}
		data, err := json.Marshal(value)
		return idempotency.CommandOutcome{Commit: true, Result: idempotency.Result{Status: status, Body: data}}, err
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	for name, value := range result.Headers() {
		c.Header(name, value)
	}
	c.Data(result.Status, "application/json; charset=utf-8", result.Body)
}
