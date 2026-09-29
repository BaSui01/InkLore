package http

import (
	a "InkLore/internal/agent/application"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/response"
	"context"

	"github.com/gin-gonic/gin"
)

func (h *Handler) listDefinitions(c *gin.Context) {
	p, ok := h.authenticate(c, "admin", false)
	if !ok {
		return
	}
	q, err := parsePagination(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	page, err := h.Service.ListDefinitions(c.Request.Context(), p, q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, 200, page)
}

func (h *Handler) createDefinition(c *gin.Context) {
	p, ok := h.authenticate(c, "admin", true)
	if !ok {
		return
	}
	input, raw, err := readStrictJSON[a.DefinitionInput](c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	h.command(c, p, raw, 201, func(ctx context.Context, current identity.Principal) error {
		return h.Service.AuthorizeAdmin(current, true)
	}, func(ctx context.Context) (any, error) {
		return h.Service.CreateDefinition(ctx, p, input)
	})
}

func (h *Handler) getDefinition(c *gin.Context) {
	p, ok := h.authenticate(c, "admin", false)
	if !ok {
		return
	}
	id := c.Param("definition_id")
	view, err := h.Service.GetDefinition(c.Request.Context(), p, id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, 200, view)
}

func (h *Handler) updateDefinition(c *gin.Context) {
	p, ok := h.authenticate(c, "admin", true)
	if !ok {
		return
	}
	id := c.Param("definition_id")
	input, raw, err := readStrictJSON[a.DefinitionUpdate](c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	h.command(c, p, raw, 200, func(ctx context.Context, current identity.Principal) error {
		_, err := h.Service.GetDefinition(ctx, current, id)
		return err
	}, func(ctx context.Context) (any, error) {
		return h.Service.UpdateDefinition(ctx, p, id, input)
	})
}

func (h *Handler) deleteDefinition(c *gin.Context) {
	p, ok := h.authenticate(c, "admin", true)
	if !ok {
		return
	}
	id := c.Param("definition_id")
	rev, err := parseQueryRevision(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	h.command(c, p, []byte("{}"), 200, func(ctx context.Context, current identity.Principal) error {
		// Tenant/actor are bound by the executor; a deleted target must not block replay.
		return h.Service.AuthorizeAdmin(current, true)
	}, func(ctx context.Context) (any, error) {
		return h.Service.DeleteDefinition(ctx, p, id, rev)
	})
}
