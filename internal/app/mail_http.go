package app

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"InkLore/internal/platform/audit"
	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/httpguard"
	"InkLore/internal/platform/idempotency"
	"InkLore/internal/platform/mail"
	"InkLore/internal/platform/response"
	"github.com/gin-gonic/gin"
)

func (a *App) registerMailRoutes(engine *gin.Engine) {
	engine.GET("/v1/platform/mail-settings", func(c *gin.Context) {
		p, ok := a.authenticate(c, "admin")
		if !ok {
			return
		}
		value, err := a.platform.Mail.Read(c.Request.Context(), p)
		if err != nil {
			response.Fail(c, err)
			return
		}
		response.OK(c, 200, value)
	})
	engine.PUT("/v1/platform/mail-settings", func(c *gin.Context) {
		var wire struct {
			mail.UpdateInput
			Enabled *bool `json:"enabled"`
		}
		if err := httpguard.DecodeJSON(c, &wire); err != nil {
			response.Fail(c, err)
			return
		}
		if wire.Enabled == nil {
			response.Fail(c, pe.Contract(400, "platform.validation_failed", nil))
			return
		}
		input := wire.UpdateInput
		input.Enabled = *wire.Enabled
		p, ok := a.authenticate(c, "admin")
		if !ok {
			return
		}
		if !a.sessionOrigin(c, "admin", false) {
			return
		}
		if err := a.platform.Identity.ValidateCSRF(c.Request.Context(), p, c.GetHeader("X-CSRF-Token")); err != nil {
			response.Fail(c, err)
			return
		}
		body, err := json.Marshal(input)
		if err != nil {
			response.Fail(c, err)
			return
		}
		requestID, traceID := response.IDs(c)
		result, err := a.platform.Idempotency.Execute(c.Request.Context(), idempotency.Request{TenantID: p.TenantID, ActorID: p.UserID, Method: http.MethodPut, Path: "/v1/platform/mail-settings", Key: c.GetHeader("Idempotency-Key"), Body: body, RequestID: requestID, TraceID: traceID}, func(ctx context.Context) error { return a.platform.Mail.Authorize(ctx, p) }, func(ctx context.Context) (idempotency.CommandOutcome, error) {
			value, err := a.platform.Mail.Update(ctx, p, input)
			if err != nil {
				return idempotency.CommandOutcome{}, err
			}
			raw, err := json.Marshal(value)
			return idempotency.CommandOutcome{Commit: true, Result: idempotency.Result{Status: 200, Body: raw}}, err
		})
		if err != nil {
			response.Fail(c, err)
			return
		}
		for key, value := range result.Headers() {
			c.Header(key, value)
		}
		c.Data(result.Status, "application/json; charset=utf-8", result.Body)
	})
	engine.POST("/v1/platform/mail-settings/test", func(c *gin.Context) {
		var input mail.TestInput
		if err := httpguard.DecodeJSON(c, &input); err != nil {
			response.Fail(c, err)
			return
		}
		p, ok := a.authenticate(c, "admin")
		if !ok {
			return
		}
		if !a.sessionOrigin(c, "admin", false) {
			return
		}
		if err := a.platform.Identity.ValidateCSRF(c.Request.Context(), p, c.GetHeader("X-CSRF-Token")); err != nil {
			response.Fail(c, err)
			return
		}
		result, err := a.platform.Mail.Test(c.Request.Context(), p, c.GetHeader("Idempotency-Key"), input)
		if err != nil {
			response.Fail(c, err)
			return
		}
		response.OK(c, 200, result)
	})
}
func mailAudit(store *audit.Store) func(context.Context, mail.Event) error {
	return func(ctx context.Context, event mail.Event) error {
		requestID, traceID := correlation(ctx)
		revision := strconv.FormatInt(event.Revision, 10)
		p := event.Principal
		_, err := store.Append(ctx, audit.Record{TenantID: p.TenantID, ActorID: &p.UserID, Audience: "admin", Action: event.Action, Outcome: "succeeded", RequestID: requestID, TraceID: traceID, SafeChanges: []audit.SafeChange{{Field: "revision", After: &revision}}})
		return err
	}
}
