package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/files"
	"InkLore/internal/platform/httpguard"
	"InkLore/internal/platform/idempotency"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/response"

	"github.com/gin-gonic/gin"
)

func (a *App) registerFileRoutes(engine *gin.Engine) {
	engine.POST("/v1/platform/files", func(c *gin.Context) {
		p, ok := a.authenticate(c, "creator")
		if !ok {
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, files.InputLimit+(64<<10))
		form, err := c.MultipartForm()
		if c.Request.MultipartForm != nil {
			defer c.Request.MultipartForm.RemoveAll()
		}
		if err != nil {
			var size *http.MaxBytesError
			if errors.As(err, &size) {
				response.Fail(c, pe.Contract(413, "platform.file_too_large", nil))
			} else {
				response.Fail(c, pe.Contract(400, "platform.validation_failed", nil))
			}
			return
		}
		if form == nil || len(form.File) != 1 || len(form.File["file"]) != 1 || len(form.Value["purpose"]) != 1 {
			response.Fail(c, pe.Contract(400, "platform.validation_failed", nil))
			return
		}
		for name, values := range form.Value {
			if (name != "purpose" && name != "novel_id") || len(values) != 1 {
				response.Fail(c, pe.Contract(400, "platform.validation_failed", nil))
				return
			}
		}
		var novel *string
		if values, exists := form.Value["novel_id"]; exists {
			if !httpguard.UUIDv4(values[0]) {
				response.Fail(c, pe.Contract(400, "platform.validation_failed", nil))
				return
			}
			novel = &values[0]
		}
		file := form.File["file"][0]
		reader, err := file.Open()
		if err != nil {
			response.Fail(c, err)
			return
		}
		defer func() { _ = reader.Close() }()
		prepared, err := files.PrepareUpload(reader, file.Filename, file.Header.Get("Content-Type"), form.Value["purpose"][0], novel)
		if err != nil {
			response.Fail(c, err)
			return
		}
		body, err := json.Marshal(prepared.DigestMetadata())
		if err != nil {
			response.Fail(c, err)
			return
		}
		a.fileMutation(c, p, body, nil, 201, func(ctx context.Context) error { return a.platform.Files.AuthorizePut(ctx, p, prepared) }, func(ctx context.Context) (files.Descriptor, error) { return a.platform.Files.Put(ctx, p, prepared) })
	})
	engine.GET("/v1/platform/files/:file_id", func(c *gin.Context) {
		p, ok := a.authenticate(c, "either")
		if !ok {
			return
		}
		value, err := a.platform.Files.Get(c.Request.Context(), p, c.Param("file_id"))
		if err != nil {
			response.Fail(c, err)
			return
		}
		response.OK(c, 200, value)
	})
	engine.GET("/v1/platform/files/:file_id/download", func(c *gin.Context) {
		p, ok := a.authenticate(c, "either")
		if !ok {
			return
		}
		err := a.platform.Files.Download(c.Request.Context(), p, c.Param("file_id"), c.Writer, c.Request)
		if err != nil && !c.Writer.Written() {
			response.Fail(c, err)
		}
	})
	engine.DELETE("/v1/platform/files/:file_id", func(c *gin.Context) {
		p, ok := a.authenticate(c, "creator")
		if !ok {
			return
		}
		if err := httpguard.RequireEmptyBody(c); err != nil {
			response.Fail(c, err)
			return
		}
		revision, err := httpguard.QueryRevision(c)
		if err != nil {
			response.Fail(c, err)
			return
		}
		for key, values := range c.Request.URL.Query() {
			if key != "expected_revision" || len(values) != 1 {
				response.Fail(c, pe.Contract(400, "platform.validation_failed", nil))
				return
			}
		}
		id := c.Param("file_id")
		a.fileMutation(c, p, json.RawMessage("{}"), c.Request.URL.Query(), 200, func(ctx context.Context) error { return a.platform.Files.AuthorizeDelete(ctx, p, id) }, func(ctx context.Context) (files.Descriptor, error) {
			return a.platform.Files.Delete(ctx, p, id, revision)
		})
	})
}
func (a *App) fileMutation(c *gin.Context, p identity.Principal, body json.RawMessage, query map[string][]string, status int, authorize idempotency.Authorize, command func(context.Context) (files.Descriptor, error)) {
	requestID, traceID := response.IDs(c)
	result, err := a.platform.Idempotency.Execute(c.Request.Context(), idempotency.Request{TenantID: p.TenantID, ActorID: p.UserID, Method: c.Request.Method, Path: c.Request.URL.Path, Key: c.GetHeader("Idempotency-Key"), Body: body, Query: query, RequestID: requestID, TraceID: traceID}, authorize, func(ctx context.Context) (idempotency.CommandOutcome, error) {
		value, err := command(ctx)
		if err != nil {
			return idempotency.CommandOutcome{}, err
		}
		raw, err := json.Marshal(value)
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
