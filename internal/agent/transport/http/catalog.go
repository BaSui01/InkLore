package http

import (
	"encoding/json"

	d "InkLore/internal/agent/domain"
	"InkLore/internal/platform/cursor"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/response"

	"github.com/gin-gonic/gin"
)

func (h *Handler) catalogQuery(c *gin.Context, p identity.Principal, operation string) (d.ListQuery, cursor.Binding, error) {
	q, err := parsePagination(c)
	binding := cursor.Binding{TenantID: p.TenantID, ActorID: p.UserID, Audience: p.Audience, PermissionRevision: p.MembershipRevision, Operation: operation, Sort: "updated_at_desc_id_desc", Filters: json.RawMessage(`{}`)}
	if err != nil {
		return q, binding, err
	}
	if q.BeforeID != "" {
		var position string
		if err := h.Cursor.Decode(q.BeforeID, binding, &position); err != nil {
			return q, binding, err
		}
		q.BeforeID = position
	}
	return q, binding, nil
}

func (h *Handler) catalogCursor(binding cursor.Binding, position *string) (*string, error) {
	if position == nil {
		return nil, nil
	}
	next, err := h.Cursor.Encode(binding, *position)
	return &next, err
}

func (h *Handler) catalogDefinitions(c *gin.Context) {
	p, ok := h.authenticate(c, "creator", false)
	if !ok {
		return
	}
	q, binding, err := h.catalogQuery(c, p, "agent_catalog_definitions")
	if err != nil {
		response.Fail(c, err)
		return
	}
	page, err := h.Service.ListCatalogDefinitions(c.Request.Context(), p, q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	page.NextCursor, err = h.catalogCursor(binding, page.NextCursor)
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
	q, binding, err := h.catalogQuery(c, p, "agent_catalog_workflows")
	if err != nil {
		response.Fail(c, err)
		return
	}
	page, err := h.Service.ListCatalogWorkflows(c.Request.Context(), p, q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	page.NextCursor, err = h.catalogCursor(binding, page.NextCursor)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, 200, page)
}
