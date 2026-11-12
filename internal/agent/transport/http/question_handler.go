package http

import (
	d "InkLore/internal/agent/domain"
	"InkLore/internal/platform/idempotency"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/response"
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
)

func (h *Handler) getQuestion(c *gin.Context) {
	p, ok := h.authenticate(c, "creator", false)
	if !ok {
		return
	}
	q, err := h.Runs.GetQuestion(c.Request.Context(), p, c.Param("run_id"), c.Param("question_id"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, 200, q)
}
func (h *Handler) answerQuestion(c *gin.Context) {
	p, ok := h.authenticate(c, "creator", true)
	if !ok {
		return
	}
	in, raw, err := readStrictJSON[d.QuestionAnswerInput](c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	runID, questionID := c.Param("run_id"), c.Param("question_id")
	h.commandOutcome(c, p, raw, func(ctx context.Context, current identity.Principal) error {
		_, err := h.Runs.GetQuestion(ctx, current, runID, questionID)
		return err
	}, func(ctx context.Context) (idempotency.CommandOutcome, error) {
		q, public, err := h.Runs.AnswerQuestion(ctx, p, runID, questionID, in)
		if err != nil {
			return idempotency.CommandOutcome{}, err
		}
		if public != nil {
			return idempotency.CommandOutcome{Commit: true, PublicError: public}, nil
		}
		data, err := json.Marshal(q)
		return idempotency.CommandOutcome{Commit: true, Result: idempotency.Result{Status: 200, Body: data}}, err
	})
}
