package http

import (
	"encoding/json"
	"strings"
	"time"

	d "InkLore/internal/agent/domain"
	"InkLore/internal/platform/cursor"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/response"
	"InkLore/internal/platform/scope"

	"github.com/gin-gonic/gin"
)

type runPageCursor struct {
	Time string `json:"time"`
	ID   string `json:"id"`
}

func (h *Handler) runPageQuery(c *gin.Context, p identity.Principal, operation, status string) (d.ListQuery, cursor.Binding, error) {
	q, err := parsePagination(c)
	filters, _ := json.Marshal(struct {
		Status string `json:"status"`
	}{Status: status})
	binding := cursor.Binding{TenantID: p.TenantID, ActorID: p.UserID, Audience: p.Audience, PermissionRevision: p.MembershipRevision, Operation: operation, Sort: "updated_at_desc_id_desc", Filters: filters}
	if operation == "agent_list_approvals" {
		binding.Sort = "created_at_desc_id_desc"
	}
	if err != nil {
		return q, binding, err
	}
	if q.BeforeID != "" {
		var after runPageCursor
		if err := h.Cursor.Decode(q.BeforeID, binding, &after); err != nil {
			return q, binding, err
		}
		if _, err := time.Parse(time.RFC3339Nano, after.Time); err != nil || !scope.ValidID(after.ID) {
			return q, binding, d.ErrCursorInvalid()
		}
		q.BeforeTime, q.BeforeID = after.Time, after.ID
	}
	return q, binding, nil
}

func (h *Handler) encodeRunPagePosition(binding cursor.Binding, raw *string) (*string, error) {
	if raw == nil {
		return nil, nil
	}
	stamp, id, found := strings.Cut(*raw, "_")
	if !found {
		return nil, d.ErrCursorInvalid()
	}
	next, err := h.Cursor.Encode(binding, runPageCursor{Time: stamp, ID: id})
	return &next, err
}

func (h *Handler) listAuditRuns(c *gin.Context) {
	p, ok := h.authenticate(c, "admin", false)
	if !ok {
		return
	}
	q, binding, err := h.runPageQuery(c, p, "agent_list_audit_runs", "")
	if err != nil {
		response.Fail(c, err)
		return
	}
	page, err := h.Runs.ListAuditRuns(c.Request.Context(), p, q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	if page.HasMore && len(page.Items) > 0 {
		last := page.Items[len(page.Items)-1]
		next, err := h.Cursor.Encode(binding, runPageCursor{Time: last.UpdatedAt.UTC().Format(time.RFC3339Nano), ID: last.ID})
		if err != nil {
			response.Fail(c, err)
			return
		}
		page.NextCursor = &next
	}
	response.OK(c, 200, page)
}

func (h *Handler) getAuditRun(c *gin.Context) {
	p, ok := h.authenticate(c, "admin", false)
	if !ok {
		return
	}
	view, err := h.Runs.GetAuditRun(c.Request.Context(), p, c.Param("run_id"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, 200, view)
}
