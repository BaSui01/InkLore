package http

import (
	"context"

	d "InkLore/internal/agent/domain"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/response"

	"github.com/gin-gonic/gin"
)

func (h *Handler) listApprovals(c *gin.Context) {
	p, ok := h.authenticate(c, "creator", false)
	if !ok {
		return
	}
	if h.Approvals == nil {
		response.Fail(c, bad())
		return
	}

	q, binding, err := h.runPageQuery(c, p, "agent_list_approvals", "pending")
	if err != nil {
		response.Fail(c, err)
		return
	}

	items, nextCursor, hasMore, err := h.Approvals.ListApprovals(c.Request.Context(), p, q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	nextCursor, err = h.encodeRunPagePosition(binding, nextCursor)
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.OK(c, 200, d.ApprovalPage{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	})
}

func (h *Handler) getApproval(c *gin.Context) {
	p, ok := h.authenticate(c, "creator", false)
	if !ok {
		return
	}
	if h.Approvals == nil {
		response.Fail(c, bad())
		return
	}

	id := c.Param("approval_id")
	view, err := h.Approvals.GetApproval(c.Request.Context(), p, id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, 200, view)
}

func (h *Handler) approveApproval(c *gin.Context) {
	p, ok := h.authenticate(c, "creator", true)
	if !ok {
		return
	}
	if h.Approvals == nil {
		response.Fail(c, bad())
		return
	}

	id := c.Param("approval_id")
	input, raw, err := readStrictJSON[d.ApprovalDecisionInput](c)
	if err != nil {
		response.Fail(c, err)
		return
	}

	h.command(c, p, raw, 200, func(ctx context.Context, current identity.Principal) error {
		approval, err := h.Approvals.GetApproval(ctx, current, id)
		if err != nil {
			return err
		}
		_, err = h.Runs.GetRun(ctx, current, approval.RunID)
		return err
	}, func(ctx context.Context) (any, error) {
		return h.Approvals.Approve(ctx, p, id, input, c.GetHeader("Idempotency-Key"))
	})
}

func (h *Handler) rejectApproval(c *gin.Context) {
	p, ok := h.authenticate(c, "creator", true)
	if !ok {
		return
	}
	if h.Approvals == nil {
		response.Fail(c, bad())
		return
	}

	id := c.Param("approval_id")
	input, raw, err := readStrictJSON[d.ApprovalDecisionInput](c)
	if err != nil {
		response.Fail(c, err)
		return
	}

	h.command(c, p, raw, 200, func(ctx context.Context, current identity.Principal) error {
		approval, err := h.Approvals.GetApproval(ctx, current, id)
		if err != nil {
			return err
		}
		_, err = h.Runs.GetRun(ctx, current, approval.RunID)
		return err
	}, func(ctx context.Context) (any, error) {
		return h.Approvals.Reject(ctx, p, id, input, c.GetHeader("Idempotency-Key"))
	})
}
