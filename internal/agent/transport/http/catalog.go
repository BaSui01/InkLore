package http

import (
	"InkLore/internal/platform/response"

	"github.com/gin-gonic/gin"
)

func (h *Handler) catalogDefinitions(c *gin.Context) {
	p, ok := h.authenticate(c, "creator", false)
	if !ok {
		return
	}
	q, err := parsePagination(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	page, err := h.Service.ListCatalogDefinitions(c.Request.Context(), p, q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, 200, page)
}

func (h *Handler) catalogWorkflows(c *gin.Context) {
	p, ok := h.authenticate(c, "creator", false)
	if !ok {
		return
	}
	q, err := parsePagination(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	page, err := h.Service.ListCatalogWorkflows(c.Request.Context(), p, q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, 200, page)
}
