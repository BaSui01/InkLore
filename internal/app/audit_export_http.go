package app

import (
	"bytes"
	"context"
	"encoding/json"

	"InkLore/internal/platform/audit"
	platformerr "InkLore/internal/platform/errors"
	"InkLore/internal/platform/response"

	"github.com/gin-gonic/gin"
)

func (a *App) registerAuditExportRoute(engine *gin.Engine) {
	engine.GET("/v1/platform/audit-events/export", func(c *gin.Context) {
		principal, ok := a.authenticate(c, "admin")
		if !ok {
			return
		}
		values := c.Request.URL.Query()
		for key, v := range values {
			if (key != "from" && key != "to" && key != "action") || len(v) != 1 {
				response.Fail(c, platformerr.Contract(400, "platform.validation_failed", nil))
				return
			}
		}
		query, err := audit.ParseExportGETQuery(audit.ExportGETQuery{From: c.Query("from"), To: c.Query("to"), Action: c.Query("action")})
		if err != nil {
			response.Fail(c, err)
			return
		}
		var events []audit.Audit
		err = a.platform.UoW.Within(c.Request.Context(), func(ctx context.Context) error {
			current, err := a.platform.Identity.Authorize(ctx, principal, "operator", "auditor")
			if err != nil {
				return err
			}
			events, err = a.platform.Audit.Export(ctx, audit.VerifiedReader{TenantID: current.TenantID, ActorID: current.UserID, Audience: current.Audience, Roles: current.Roles}, query)
			return err
		})
		if err != nil {
			response.Fail(c, err)
			return
		}
		var buffer bytes.Buffer
		encoder := json.NewEncoder(&buffer)
		for _, event := range events {
			if err = encoder.Encode(event); err != nil {
				response.Fail(c, err)
				return
			}
		}
		c.Header("Content-Disposition", `attachment; filename="audit.ndjson"`)
		c.Header("Cache-Control", "no-store")
		c.Data(200, "application/x-ndjson", buffer.Bytes())
	})
}
