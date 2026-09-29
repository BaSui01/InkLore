package http

import (
	a "InkLore/internal/agent/application"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/response"
	"context"

	"github.com/gin-gonic/gin"
)

func (h *Handler) listMCPServers(c *gin.Context) {
	p, ok := h.authenticate(c, "admin", false)
	if !ok {
		return
	}
	q, err := parsePagination(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	page, err := h.Service.ListMCPServers(c.Request.Context(), p, q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, 200, page)
}

func (h *Handler) createMCPServer(c *gin.Context) {
	p, ok := h.authenticate(c, "admin", true)
	if !ok {
		return
	}
	input, raw, err := readStrictJSON[a.MCPServerInput](c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	h.command(c, p, raw, 201, func(ctx context.Context, current identity.Principal) error {
		return h.Service.AuthorizeAdmin(current, true)
	}, func(ctx context.Context) (any, error) {
		return h.Service.CreateMCPServer(ctx, p, input)
	})
}

func (h *Handler) getMCPServer(c *gin.Context) {
	p, ok := h.authenticate(c, "admin", false)
	if !ok {
		return
	}
	id := c.Param("mcp_server_id")
	view, err := h.Service.GetMCPServer(c.Request.Context(), p, id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, 200, view)
}

func (h *Handler) updateMCPServer(c *gin.Context) {
	p, ok := h.authenticate(c, "admin", true)
	if !ok {
		return
	}
	id := c.Param("mcp_server_id")
	input, raw, err := readStrictJSON[a.MCPServerUpdate](c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	h.command(c, p, raw, 200, func(ctx context.Context, current identity.Principal) error {
		_, err := h.Service.GetMCPServer(ctx, current, id)
		return err
	}, func(ctx context.Context) (any, error) {
		return h.Service.UpdateMCPServer(ctx, p, id, input)
	})
}

func (h *Handler) deleteMCPServer(c *gin.Context) {
	p, ok := h.authenticate(c, "admin", true)
	if !ok {
		return
	}
	id := c.Param("mcp_server_id")
	rev, err := parseQueryRevision(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	h.command(c, p, []byte("{}"), 200, func(ctx context.Context, current identity.Principal) error {
		// Tenant/actor are bound by the executor; a deleted target must not block replay.
		return h.Service.AuthorizeAdmin(current, true)
	}, func(ctx context.Context) (any, error) {
		return h.Service.DeleteMCPServer(ctx, p, id, rev)
	})
}
