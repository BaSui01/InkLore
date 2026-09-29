package http

import (
	a "InkLore/internal/outline/application"
	d "InkLore/internal/outline/domain"
	"InkLore/internal/platform/cursor"
	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/response"
	"InkLore/internal/platform/scope"
	"encoding/json"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type anchor struct {
	At time.Time `json:"at"`
	ID string    `json:"id"`
}

func (h *Handler) query(c *gin.Context, p identity.Principal, resource string) (a.ListQuery, cursor.Binding, error) {
	q := a.ListQuery{Limit: 20}
	values := c.Request.URL.Query()
	allowed := map[string]bool{"limit": true, "cursor": true}
	if resource != "events" {
		allowed["deleted"] = true
	}
	switch resource {
	case "nodes":
		allowed["parent_id"] = true
		allowed["level"] = true
	case "beats", "scenes":
		allowed["node_id"] = true
	case "clues":
		allowed["status"] = true
		allowed["planned_reveal_node_id"] = true
	}
	for k, v := range values {
		if !allowed[k] || len(v) != 1 || v[0] == "" {
			return q, cursor.Binding{}, bad()
		}
	}
	if v := values.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 100 {
			return q, cursor.Binding{}, bad()
		}
		q.Limit = n
	}
	if v := values.Get("deleted"); v != "" {
		if v != "true" && v != "false" {
			return q, cursor.Binding{}, bad()
		}
		q.Deleted = v == "true"
	}
	q.ParentID = values.Get("parent_id")
	q.Level = values.Get("level")
	q.NodeID = values.Get("node_id")
	q.Status = values.Get("status")
	q.PlannedRevealNodeID = values.Get("planned_reveal_node_id")
	for _, v := range []string{q.ParentID, q.NodeID, q.PlannedRevealNodeID} {
		if v != "" && !scope.ValidID(v) {
			return q, cursor.Binding{}, bad()
		}
	}
	if (resource == "beats" || resource == "scenes") && q.NodeID == "" {
		return q, cursor.Binding{}, bad()
	}
	if q.Level != "" && q.Level != "master" && q.Level != "volume" && q.Level != "chapter" {
		return q, cursor.Binding{}, bad()
	}
	if q.Status != "" {
		switch q.Status {
		case "planned", "planted", "developing", "revealed", "closed", "abandoned":
		default:
			return q, cursor.Binding{}, bad()
		}
	}
	filters := map[string]any{"novel_id": c.Param("novel_id"), "clue_id": c.Param("clue_id"), "parent_id": q.ParentID, "level": q.Level, "node_id": q.NodeID, "status": q.Status, "planned_reveal_node_id": q.PlannedRevealNodeID, "deleted": q.Deleted}
	raw, _ := json.Marshal(filters)
	sort := "updated_at:desc,id:desc"
	if resource == "events" {
		sort = "created_at:desc,id:desc"
	}
	b := cursor.Binding{TenantID: p.TenantID, ActorID: p.UserID, Audience: p.Audience, PermissionRevision: p.MembershipRevision, Operation: "outline.list." + resource, Filters: raw, Sort: sort}
	if token := values.Get("cursor"); token != "" {
		var key anchor
		if err := h.Cursor.Decode(token, b, &key); err != nil {
			return q, b, err
		}
		if !scope.ValidID(key.ID) || key.At.IsZero() {
			return q, b, pe.Contract(400, "platform.cursor_invalid", nil)
		}
		q.BeforeAt = key.At
		q.BeforeID = key.ID
	}
	return q, b, nil
}
func sendPage[T any](h *Handler, c *gin.Context, b cursor.Binding, result a.ListResult[T], key func(T) anchor) {
	out := d.Page[T]{Items: result.Items, HasMore: result.HasMore}
	if result.HasMore && len(result.Items) > 0 {
		token, err := h.Cursor.Encode(b, key(result.Items[len(result.Items)-1]))
		if err != nil {
			response.Fail(c, err)
			return
		}
		out.NextCursor = &token
	}
	response.OK(c, 200, out)
}
func (h *Handler) listNodes(c *gin.Context) {
	p, ok := h.authenticate(c)
	if !ok {
		return
	}
	q, b, err := h.query(c, p, "nodes")
	if err != nil {
		response.Fail(c, err)
		return
	}
	result, err := h.Service.ListNodes(c.Request.Context(), principal(p), c.Param("novel_id"), q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	sendPage(h, c, b, result, func(v d.Node) anchor { return anchor{At: v.UpdatedAt, ID: v.ID} })
}
func (h *Handler) listBeats(c *gin.Context) {
	p, ok := h.authenticate(c)
	if !ok {
		return
	}
	q, b, err := h.query(c, p, "beats")
	if err != nil {
		response.Fail(c, err)
		return
	}
	result, err := h.Service.ListBeats(c.Request.Context(), principal(p), c.Param("novel_id"), q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	sendPage(h, c, b, result, func(v d.Beat) anchor { return anchor{At: v.UpdatedAt, ID: v.ID} })
}
func (h *Handler) listScenes(c *gin.Context) {
	p, ok := h.authenticate(c)
	if !ok {
		return
	}
	q, b, err := h.query(c, p, "scenes")
	if err != nil {
		response.Fail(c, err)
		return
	}
	result, err := h.Service.ListScenes(c.Request.Context(), principal(p), c.Param("novel_id"), q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	sendPage(h, c, b, result, func(v d.Scene) anchor { return anchor{At: v.UpdatedAt, ID: v.ID} })
}
func (h *Handler) listClues(c *gin.Context) {
	p, ok := h.authenticate(c)
	if !ok {
		return
	}
	q, b, err := h.query(c, p, "clues")
	if err != nil {
		response.Fail(c, err)
		return
	}
	result, err := h.Service.ListClues(c.Request.Context(), principal(p), c.Param("novel_id"), q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	sendPage(h, c, b, result, func(v d.Clue) anchor { return anchor{At: v.UpdatedAt, ID: v.ID} })
}
func (h *Handler) listClueEvents(c *gin.Context) {
	p, ok := h.authenticate(c)
	if !ok {
		return
	}
	q, b, err := h.query(c, p, "events")
	if err != nil {
		response.Fail(c, err)
		return
	}
	result, err := h.Service.ListClueEvents(c.Request.Context(), principal(p), c.Param("clue_id"), q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	sendPage(h, c, b, result, func(v d.ClueEvent) anchor { return anchor{At: v.CreatedAt, ID: v.ID} })
}
