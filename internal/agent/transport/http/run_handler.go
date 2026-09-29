package http

import (
	"context"

	d "InkLore/internal/agent/domain"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/response"

	"github.com/gin-gonic/gin"
)

func (h *Handler) createRun(c *gin.Context) {
	p, ok := h.authenticate(c, "creator", true)
	if !ok {
		return
	}
	if h.Runs == nil {
		response.Fail(c, bad())
		return
	}

	input, raw, err := readStrictJSON[d.CreateRunInput](c)
	if err != nil {
		response.Fail(c, err)
		return
	}

	h.command(c, p, raw, 202, func(ctx context.Context, current identity.Principal) error {
		return h.Runs.AuthorizeCreateRun(ctx, current, input)
	}, func(ctx context.Context) (any, error) {
		return h.Runs.CreateRun(ctx, p, input, c.GetHeader("Idempotency-Key"))
	})
}

func (h *Handler) listRuns(c *gin.Context) {
	p, ok := h.authenticate(c, "creator", false)
	if !ok {
		return
	}
	if h.Runs == nil {
		response.Fail(c, bad())
		return
	}

	q, err := parsePagination(c)
	if err != nil {
		response.Fail(c, err)
		return
	}

	status := c.Query("status")
	items, nextCursor, hasMore, err := h.Runs.ListRuns(c.Request.Context(), p, status, q)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.OK(c, 200, d.RunSummaryPage{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	})
}

func (h *Handler) getRun(c *gin.Context) {
	p, ok := h.authenticate(c, "creator", false)
	if !ok {
		return
	}
	if h.Runs == nil {
		response.Fail(c, bad())
		return
	}

	id := c.Param("run_id")
	view, err := h.Runs.GetRun(c.Request.Context(), p, id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, 200, view)
}

func (h *Handler) cancelRun(c *gin.Context) {
	p, ok := h.authenticate(c, "creator", true)
	if !ok {
		return
	}
	if h.Runs == nil {
		response.Fail(c, bad())
		return
	}

	id := c.Param("run_id")
	input, raw, err := readStrictJSON[d.CancelRunInput](c)
	if err != nil {
		response.Fail(c, err)
		return
	}

	h.command(c, p, raw, 200, func(ctx context.Context, current identity.Principal) error {
		_, err := h.Runs.GetRun(ctx, current, id)
		return err
	}, func(ctx context.Context) (any, error) {
		return h.Runs.CancelRun(ctx, p, id, input, c.GetHeader("Idempotency-Key"))
	})
}
