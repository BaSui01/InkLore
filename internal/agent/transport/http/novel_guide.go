package http

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	d "InkLore/internal/agent/domain"
	"InkLore/internal/platform/httpguard"
	"InkLore/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *Handler) startNovelGuide(c *gin.Context) {
	p, ok := h.authenticate(c, "creator", true)
	if !ok {
		return
	}
	in, _, err := readStrictJSON[d.NovelGuideStartInput](c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	session, err := h.NovelGuide.Start(c.Request.Context(), p, in.InitialIdea, key)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, 201, session)
}

func (h *Handler) getNovelGuide(c *gin.Context) {
	p, ok := h.authenticate(c, "creator", false)
	if !ok {
		return
	}
	q, err := parseSingleQuery(c)
	if err != nil || len(q) != 0 {
		response.Fail(c, bad())
		return
	}
	if err = httpguard.RequireEmptyBody(c); err != nil {
		response.Fail(c, err)
		return
	}
	session, err := h.NovelGuide.Get(c.Request.Context(), p, c.Param("session_id"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, 200, session)
}

func (h *Handler) answerNovelGuide(c *gin.Context) {
	p, ok := h.authenticate(c, "creator", true)
	if !ok {
		return
	}
	in, raw, err := readStrictJSON[d.NovelGuideAnswerInput](c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if _, err = uuid.Parse(key); err != nil {
		response.Fail(c, bad())
		return
	}
	id := c.Param("session_id")
	sum := sha256.Sum256(raw)
	cmd := &d.NovelGuideCommand{ID: uuid.NewString(), TenantID: p.TenantID, OwnerUserID: p.UserID, SessionID: id, CommandType: "answer", IdempotencyKey: key, RequestHash: hex.EncodeToString(sum[:])}
	var prior *d.NovelGuideCommand
	var created bool
	err = h.NovelGuide.Within(c.Request.Context(), func(ctx context.Context) error {
		var e error
		prior, created, e = h.NovelGuide.BeginCommand(ctx, cmd)
		return e
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	if !created {
		if prior.RequestHash != cmd.RequestHash {
			response.Fail(c, bad())
			return
		}
		if prior.Status == "completed" {
			c.Data(200, "application/json; charset=utf-8", prior.Response)
			return
		}
		response.Fail(c, d.ErrVersionConflict(nil))
		return
	}
	session, err := h.NovelGuide.Answer(c.Request.Context(), p, id, in.ExpectedRevision, in.Answer)
	if err != nil {
		_ = h.NovelGuide.FailCommand(c.Request.Context(), p, id, key, "agent.novel_guide_failed")
		response.Fail(c, err)
		return
	}
	requestID, traceID := response.IDs(c)
	body, err := json.Marshal(response.SuccessEnvelope{Code: "ok", Data: session, RequestID: requestID, TraceID: traceID})
	if err == nil {
		err = h.NovelGuide.CompleteCommand(c.Request.Context(), p, id, key, body)
	}
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, 200, session)
}
