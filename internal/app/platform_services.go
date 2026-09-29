package app

import (
	"context"
	"strconv"

	"InkLore/internal/platform/audit"
	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/files"
	"InkLore/internal/platform/notifications"
	"InkLore/internal/platform/observability"
	"InkLore/internal/platform/response"
	"InkLore/internal/platform/version"

	"github.com/gin-gonic/gin"
)

type fileReferenceReader struct{ app *App }

func (r fileReferenceReader) InUse(ctx context.Context, tenant, id string) (bool, error) {
	return r.app.domains.Content.FileInUse(ctx, tenant, id)
}

type redisProbe struct{ platform *Platform }

func (p redisProbe) Ping(ctx context.Context) error {
	if p.platform.Redis == nil {
		return pe.Contract(503, "platform.dependency_unavailable", nil)
	}
	return p.platform.Redis.Ping(ctx).Err()
}

func (a *App) registerPlatformServices(engine *gin.Engine) error {
	p := a.platform
	if p.Files == nil {
		return pe.Contract(500, "platform.internal_error", nil)
	}
	var err error
	p.Notifications, err = notifications.New(p.UoW, notifications.Options{Identity: p.Identity, Scope: p.Scope, Source: a.domains.Outbox, CreatorOrigin: p.CreatorOrigin, AdminOrigin: p.AdminOrigin})
	if err != nil {
		return err
	}
	p.Observability, err = observability.New(p.UoW, observability.Options{Identity: p.Identity, Redis: redisProbe{p}, Version: version.Version, Cursor: p.Cursor})
	if err != nil {
		return err
	}
	a.registerFileRoutes(engine)
	p.Notifications.RegisterRoutes(engine)
	engine.GET("/v1/platform/tenants", func(c *gin.Context) {
		principal, ok := a.authenticate(c, "admin")
		if !ok {
			return
		}
		if len(c.Request.URL.Query()) != 0 {
			response.Fail(c, pe.Contract(400, "platform.validation_failed", nil))
			return
		}
		tenants, err := p.Identity.ListTenants(c.Request.Context(), principal)
		if err != nil {
			response.Fail(c, err)
			return
		}
		response.OK(c, 200, tenants)
	})
	engine.GET("/v1/platform/status", func(c *gin.Context) {
		principal, ok := a.authenticate(c, "admin")
		if !ok {
			return
		}
		if len(c.Request.URL.Query()) != 0 {
			response.Fail(c, pe.Contract(400, "platform.validation_failed", nil))
			return
		}
		status, err := p.Observability.Status(c.Request.Context(), principal)
		if err != nil {
			response.Fail(c, err)
			return
		}
		response.OK(c, 200, status)
	})
	engine.GET("/v1/platform/secret-references", func(c *gin.Context) {
		principal, ok := a.authenticate(c, "admin")
		if !ok {
			return
		}
		for key, values := range c.Request.URL.Query() {
			if (key != "limit" && key != "cursor") || len(values) != 1 {
				response.Fail(c, pe.Contract(400, "platform.validation_failed", nil))
				return
			}
		}
		limit, err := queryLimit(c)
		if err != nil {
			response.Fail(c, err)
			return
		}
		page, err := p.Observability.ListSecretReferences(c.Request.Context(), principal, limit, c.Query("cursor"))
		if err != nil {
			response.Fail(c, err)
			return
		}
		response.OK(c, 200, page)
	})
	return nil
}

func (a *App) ensureFileService() error {
	p := a.platform
	if p.Files != nil {
		return nil
	}
	var err error
	p.Files, err = files.New(p.UoW, files.Options{Identity: p.Identity, Scope: p.Scope, References: fileReferenceReader{a}, OnEvent: func(ctx context.Context, e files.Event) error {
		requestID, traceID := correlation(ctx)
		revision := strconv.FormatInt(e.File.Revision, 10)
		_, err := p.Audit.Append(ctx, audit.Record{TenantID: e.Principal.TenantID, ActorID: &e.Principal.UserID, Audience: e.Principal.Audience, Action: e.Action, ResourceID: &e.File.ID, Outcome: "succeeded", RequestID: requestID, TraceID: traceID, SafeChanges: []audit.SafeChange{{Field: "revision", After: &revision}}})
		return err
	}})
	return err
}
