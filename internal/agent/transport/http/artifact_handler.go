package http

import (
	d "InkLore/internal/agent/domain"
	"InkLore/internal/platform/response"

	"github.com/gin-gonic/gin"
)

func (h *Handler) listArtifacts(c *gin.Context) {
	p, ok := h.authenticate(c, "creator", false)
	if !ok {
		return
	}
	if h.Artifacts == nil {
		response.Fail(c, bad())
		return
	}

	runID := c.Param("run_id")
	q, err := parsePagination(c)
	if err != nil {
		response.Fail(c, err)
		return
	}

	items, nextCursor, hasMore, err := h.Artifacts.ListArtifacts(c.Request.Context(), p, runID, q)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.OK(c, 200, d.ArtifactPage{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	})
}

func (h *Handler) getArtifact(c *gin.Context) {
	p, ok := h.authenticate(c, "creator", false)
	if !ok {
		return
	}
	if h.Artifacts == nil {
		response.Fail(c, bad())
		return
	}

	id := c.Param("artifact_id")
	art, err := h.Artifacts.GetArtifact(c.Request.Context(), p, id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, 200, art)
}
