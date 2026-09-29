package http

import (
	"InkLore/internal/platform/response"
	"InkLore/internal/platform/scope"
	"strconv"

	"github.com/gin-gonic/gin"
)

func parseCommonQuery(c *gin.Context, allowed map[string]bool) (int, string, bool, error) {
	limit := 20
	cursor := ""
	includeDeleted := false

	values := c.Request.URL.Query()
	for k, v := range values {
		if !allowed[k] || len(v) != 1 {
			return 0, "", false, bad()
		}
	}
	if v := values.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 100 {
			return 0, "", false, bad()
		}
		limit = n
	}
	if v := values.Get("cursor"); v != "" {
		if len(v) > 2048 {
			return 0, "", false, bad()
		}
		cursor = v
	}
	if v := values.Get("include_deleted"); v != "" {
		if v != "true" && v != "false" {
			return 0, "", false, bad()
		}
		includeDeleted = (v == "true")
	}
	return limit, cursor, includeDeleted, nil
}

func (h *Handler) listAssets(c *gin.Context) {
	p, ok := h.authenticate(c)
	if !ok {
		return
	}
	limit, cursor, includeDeleted, err := parseCommonQuery(c, map[string]bool{"limit": true, "cursor": true, "include_deleted": true})
	if err != nil {
		response.Fail(c, err)
		return
	}
	res, err := h.Service.ListAssets(c.Request.Context(), principal(p), limit, cursor, includeDeleted)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, 200, res)
}

func (h *Handler) getAsset(c *gin.Context) {
	p, ok := h.authenticate(c)
	if !ok {
		return
	}
	_, _, includeDeleted, err := parseCommonQuery(c, map[string]bool{"include_deleted": true})
	if err != nil {
		response.Fail(c, err)
		return
	}
	res, err := h.Service.GetAsset(c.Request.Context(), principal(p), c.Param("asset_id"), includeDeleted)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, 200, res)
}

func (h *Handler) listSegments(c *gin.Context) {
	p, ok := h.authenticate(c)
	if !ok {
		return
	}
	limit, cursor, _, err := parseCommonQuery(c, map[string]bool{"limit": true, "cursor": true})
	if err != nil {
		response.Fail(c, err)
		return
	}
	res, err := h.Service.ListSegments(c.Request.Context(), principal(p), c.Param("asset_id"), limit, cursor)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, 200, res)
}

func (h *Handler) listAnalyses(c *gin.Context) {
	p, ok := h.authenticate(c)
	if !ok {
		return
	}
	limit, cursor, _, err := parseCommonQuery(c, map[string]bool{"limit": true, "cursor": true, "novel_id": true, "state": true})
	if err != nil {
		response.Fail(c, err)
		return
	}
	var novelID *string
	if v := c.Query("novel_id"); v != "" {
		if !scope.ValidID(v) {
			response.Fail(c, bad())
			return
		}
		novelID = &v
	}
	var state *string
	if v := c.Query("state"); v != "" {
		state = &v
	}
	res, err := h.Service.ListAnalyses(c.Request.Context(), principal(p), novelID, state, limit, cursor)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, 200, res)
}

func (h *Handler) listStyles(c *gin.Context) {
	p, ok := h.authenticate(c)
	if !ok {
		return
	}
	limit, cursor, includeDeleted, err := parseCommonQuery(c, map[string]bool{"limit": true, "cursor": true, "include_deleted": true})
	if err != nil {
		response.Fail(c, err)
		return
	}
	res, err := h.Service.ListStyles(c.Request.Context(), principal(p), limit, cursor, includeDeleted)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, 200, res)
}

func (h *Handler) getStyle(c *gin.Context) {
	p, ok := h.authenticate(c)
	if !ok {
		return
	}
	_, _, includeDeleted, err := parseCommonQuery(c, map[string]bool{"include_deleted": true})
	if err != nil {
		response.Fail(c, err)
		return
	}
	res, err := h.Service.GetStyle(c.Request.Context(), principal(p), c.Param("style_id"), includeDeleted)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, 200, res)
}

func (h *Handler) listStyleVersions(c *gin.Context) {
	p, ok := h.authenticate(c)
	if !ok {
		return
	}
	limit, cursor, _, err := parseCommonQuery(c, map[string]bool{"limit": true, "cursor": true})
	if err != nil {
		response.Fail(c, err)
		return
	}
	res, err := h.Service.ListStyleVersions(c.Request.Context(), principal(p), c.Param("style_id"), limit, cursor)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, 200, res)
}

func (h *Handler) getStyleVersion(c *gin.Context) {
	p, ok := h.authenticate(c)
	if !ok {
		return
	}
	if len(c.Request.URL.Query()) != 0 {
		response.Fail(c, bad())
		return
	}
	res, err := h.Service.GetStyleVersion(c.Request.Context(), principal(p), c.Param("style_id"), c.Param("version_id"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, 200, res)
}

func (h *Handler) listCards(c *gin.Context) {
	p, ok := h.authenticate(c)
	if !ok {
		return
	}
	limit, cursor, includeDeleted, err := parseCommonQuery(c, map[string]bool{"limit": true, "cursor": true, "novel_id": true, "state": true, "include_deleted": true})
	if err != nil {
		response.Fail(c, err)
		return
	}
	var novelID *string
	if v := c.Query("novel_id"); v != "" {
		if !scope.ValidID(v) {
			response.Fail(c, bad())
			return
		}
		novelID = &v
	}
	var state *string
	if v := c.Query("state"); v != "" {
		state = &v
	}
	res, err := h.Service.ListCards(c.Request.Context(), principal(p), novelID, nil, state, limit, cursor, includeDeleted)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, 200, res)
}

func (h *Handler) getCard(c *gin.Context) {
	p, ok := h.authenticate(c)
	if !ok {
		return
	}
	_, _, includeDeleted, err := parseCommonQuery(c, map[string]bool{"include_deleted": true})
	if err != nil {
		response.Fail(c, err)
		return
	}
	res, err := h.Service.GetCard(c.Request.Context(), principal(p), c.Param("card_id"), includeDeleted)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, 200, res)
}

func (h *Handler) listAdoptions(c *gin.Context) {
	p, ok := h.authenticate(c)
	if !ok {
		return
	}
	limit, cursor, _, err := parseCommonQuery(c, map[string]bool{"limit": true, "cursor": true})
	if err != nil {
		response.Fail(c, err)
		return
	}
	res, err := h.Service.ListAdoptions(c.Request.Context(), principal(p), c.Param("card_id"), limit, cursor)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, 200, res)
}
