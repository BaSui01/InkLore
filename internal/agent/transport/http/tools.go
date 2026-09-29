package http

import (
	a "InkLore/internal/agent/application"
	d "InkLore/internal/agent/domain"
	"InkLore/internal/platform/cursor"
	"InkLore/internal/platform/response"
	"encoding/json"
	"sort"

	"github.com/gin-gonic/gin"
)

// listBuiltinTools lists registered handlers with tenant-scoped execution status.
// The signed cursor is scoped to the Admin principal and tool-key ordering.
func (h *Handler) listBuiltinTools(c *gin.Context) {
	p, ok := h.authenticate(c, "admin", false)
	if !ok {
		return
	}
	q, err := parsePagination(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	binding := cursor.Binding{TenantID: p.TenantID, ActorID: p.UserID, Audience: "admin", Operation: "agent_list_builtin_tools", Sort: "tool_key_asc", Filters: json.RawMessage(`{}`)}
	var after string
	if q.BeforeID != "" {
		if err := h.Cursor.Decode(q.BeforeID, binding, &after); err != nil {
			response.Fail(c, err)
			return
		}
	}
	tools, err := h.Service.ListBuiltinTools(c.Request.Context(), p)
	if err != nil {
		response.Fail(c, err)
		return
	}
	start := sort.Search(len(tools), func(i int) bool { return tools[i].ToolKey > after })
	if after != "" && (start == 0 || tools[start-1].ToolKey != after) {
		response.Fail(c, d.ErrCursorInvalid())
		return
	}
	end := start + q.Limit
	if end > len(tools) {
		end = len(tools)
	}
	page := &a.BuiltinToolPage{Items: tools[start:end], HasMore: end < len(tools)}
	if page.HasMore {
		next, err := h.Cursor.Encode(binding, tools[end-1].ToolKey)
		if err != nil {
			response.Fail(c, err)
			return
		}
		page.NextCursor = &next
	}
	response.OK(c, 200, page)
}

func (h *Handler) listTools(c *gin.Context) {
	p, ok := h.authenticate(c, "admin", false)
	if !ok {
		return
	}
	q, err := parsePagination(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	page, err := h.Service.ListTools(c.Request.Context(), p, q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, 200, page)
}

func (h *Handler) getTool(c *gin.Context) {
	p, ok := h.authenticate(c, "admin", false)
	if !ok {
		return
	}
	id := c.Param("tool_id")
	view, err := h.Service.GetTool(c.Request.Context(), p, id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, 200, view)
}
