package app

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	platformerr "InkLore/internal/platform/errors"
	"InkLore/internal/platform/httpguard"
	"InkLore/internal/platform/idempotency"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/response"

	"github.com/gin-gonic/gin"
)

func (a *App) registerPasswordRoute(engine *gin.Engine) {
	engine.PUT("/v1/auth/password", func(c *gin.Context) {
		var wire struct {
			CurrentPassword  *string `json:"current_password"`
			NewPassword      string  `json:"new_password"`
			ExpectedRevision int64   `json:"expected_revision"`
		}
		if err := httpguard.DecodeJSON(c, &wire); err != nil {
			response.Fail(c, err)
			return
		}
		if wire.CurrentPassword == nil {
			response.Fail(c, platformerr.Contract(400, "platform.validation_failed", nil))
			return
		}
		if err := httpguard.ExpectedRevision(wire.ExpectedRevision); err != nil {
			response.Fail(c, err)
			return
		}
		input := identity.ChangePasswordInput{CurrentPassword: *wire.CurrentPassword, NewPassword: wire.NewPassword, ExpectedRevision: wire.ExpectedRevision}
		principal, ok := a.authenticate(c, "either")
		if !ok {
			return
		}
		body, err := json.Marshal(input)
		if err != nil {
			response.Fail(c, err)
			return
		}
		requestID, traceID := response.IDs(c)
		result, err := a.platform.Idempotency.Execute(c.Request.Context(), idempotency.Request{TenantID: principal.TenantID, ActorID: principal.UserID, Method: http.MethodPut, Path: "/v1/auth/password", Key: c.GetHeader("Idempotency-Key"), Body: body, RequestID: requestID, TraceID: traceID}, func(ctx context.Context) error { return a.platform.Identity.AuthorizePasswordChange(ctx, principal) }, func(ctx context.Context) (idempotency.CommandOutcome, error) {
			value, err := a.platform.Identity.ChangePassword(ctx, principal, input)
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
		if result.Status >= 200 && result.Status < 300 && !result.Replay {
			a.setRefreshCookie(c, principal.Audience, "", time.Time{})
		}
		for key, value := range result.Headers() {
			c.Header(key, value)
		}
		c.Data(result.Status, "application/json; charset=utf-8", result.Body)
	})
}
