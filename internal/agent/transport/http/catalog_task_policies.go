package http

import (
	"bytes"
	"encoding/json"
	"net/url"

	d "InkLore/internal/agent/domain"
	"InkLore/internal/platform/cursor"
	"InkLore/internal/platform/response"
	"InkLore/internal/platform/scope"
	"github.com/gin-gonic/gin"
)

func validateCreatorPolicySelection(in d.CreateRunInput, raw []byte) error {
	// The shared body guard enforces size/UTF-8/duplicate keys, but does not
	// reject unknown DTO fields. Creator input must not silently ignore overrides.
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var checked d.CreateRunInput
	if err := decoder.Decode(&checked); err != nil {
		return d.ErrValidationFailed(nil)
	}
	if !scope.ValidID(in.TaskPolicyVersionID) || in.TaskKey != "" || in.ModelID != nil || in.ModelKey != nil || !d.IsCreatorTaskKind(in.TaskKind) {
		return d.ErrValidationFailed(map[string]any{"reason": "select a published task_policy_version_id for task_kind"})
	}
	return nil
}

func (h *Handler) catalogTaskPolicies(c *gin.Context) {
	p, ok := h.authenticate(c, "creator", false)
	if !ok {
		return
	}
	values, err := url.ParseQuery(c.Request.URL.RawQuery)
	if err != nil {
		response.Fail(c, d.ErrValidationFailed(nil))
		return
	}
	for key, items := range values {
		if (key != "limit" && key != "cursor") || len(items) != 1 {
			response.Fail(c, d.ErrValidationFailed(nil))
			return
		}
	}
	q, err := parsePagination(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	taskKind := ""
	filters, err := json.Marshal(map[string]string{"task_kind": taskKind})
	if err != nil {
		response.Fail(c, err)
		return
	}
	binding := cursor.Binding{TenantID: p.TenantID, ActorID: p.UserID, Audience: p.Audience, PermissionRevision: p.MembershipRevision, Operation: "agent_catalog_task_policies", Filters: filters, Sort: "policy_id_asc"}
	var after string
	if q.BeforeID != "" {
		if err := h.Cursor.Decode(q.BeforeID, binding, &after); err != nil {
			response.Fail(c, err)
			return
		}
		if !scope.ValidID(after) {
			response.Fail(c, d.ErrCursorInvalid())
			return
		}
	}
	items, err := h.Runs.ListCreatorTaskPolicies(c.Request.Context(), p, taskKind, after, q.Limit+1)
	if err != nil {
		response.Fail(c, err)
		return
	}
	page := d.CreatorTaskPolicyPage{Items: items, HasMore: len(items) > q.Limit}
	if page.HasMore {
		page.Items = items[:q.Limit]
		next, err := h.Cursor.Encode(binding, page.Items[len(page.Items)-1].PolicyID)
		if err != nil {
			response.Fail(c, err)
			return
		}
		page.NextCursor = &next
	}
	response.OK(c, 200, page)
}
