package http

import (
	d "InkLore/internal/compliance/domain"
	"InkLore/internal/platform/cursor"
	pe "InkLore/internal/platform/errors"
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

func sendPage[T any](h *Handler, c *gin.Context, b cursor.Binding, page d.Page[T], key func(T) anchor) {
	out := d.Page[T]{Items: page.Items, HasMore: page.HasMore}
	if page.HasMore && len(page.Items) > 0 {
		token, err := h.Cursor.Encode(b, key(page.Items[len(page.Items)-1]))
		if err != nil {
			response.Fail(c, err)
			return
		}
		out.NextCursor = &token
	}
	response.OK(c, 200, out)
}

func (h *Handler) listRuleSets(c *gin.Context) {
	p, ok := h.authenticate(c, "admin", "operator")
	if !ok {
		return
	}

	q := d.ListQuery{Limit: 20}
	values := c.Request.URL.Query()
	allowed := map[string]bool{"limit": true, "cursor": true, "state": true}
	for k, v := range values {
		if !allowed[k] || len(v) != 1 || v[0] == "" {
			response.Fail(c, bad())
			return
		}
	}
	if v := values.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 100 {
			response.Fail(c, bad())
			return
		}
		q.Limit = n
	}
	if v := values.Get("state"); v != "" {
		if v != d.RuleSetStateDraft && v != d.RuleSetStatePublished && v != d.RuleSetStateRetired {
			response.Fail(c, bad())
			return
		}
		q.State = v
	}

	filters, _ := json.Marshal(map[string]any{"state": q.State})
	b := cursor.Binding{
		TenantID:           p.TenantID,
		ActorID:            p.UserID,
		Audience:           p.Audience,
		PermissionRevision: p.MembershipRevision,
		Operation:          "compliance.list.rule_sets",
		Filters:            filters,
		Sort:               "updated_at:desc,id:desc",
	}

	if token := values.Get("cursor"); token != "" {
		var key anchor
		if err := h.Cursor.Decode(token, b, &key); err != nil {
			response.Fail(c, err)
			return
		}
		if !scope.ValidID(key.ID) || key.At.IsZero() {
			response.Fail(c, pe.Contract(400, "platform.cursor_invalid", nil))
			return
		}
		q.BeforeAt = key.At
		q.BeforeID = key.ID
	}

	page, err := h.Service.ListRuleSets(c.Request.Context(), principal(p), q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	sendPage(h, c, b, page, func(v d.RuleSet) anchor { return anchor{At: v.UpdatedAt, ID: v.ID} })
}

func (h *Handler) listRules(c *gin.Context) {
	p, ok := h.authenticate(c, "admin", "operator")
	if !ok {
		return
	}

	ruleSetID := c.Param("rule_set_id")
	q := d.ListQuery{Limit: 20}
	values := c.Request.URL.Query()
	allowed := map[string]bool{"limit": true, "cursor": true}
	for k, v := range values {
		if !allowed[k] || len(v) != 1 || v[0] == "" {
			response.Fail(c, bad())
			return
		}
	}
	if v := values.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 100 {
			response.Fail(c, bad())
			return
		}
		q.Limit = n
	}

	filters, _ := json.Marshal(map[string]any{"rule_set_id": ruleSetID})
	b := cursor.Binding{
		TenantID:           p.TenantID,
		ActorID:            p.UserID,
		Audience:           p.Audience,
		PermissionRevision: p.MembershipRevision,
		Operation:          "compliance.list.rules",
		Filters:            filters,
		Sort:               "updated_at:desc,id:desc",
	}

	if token := values.Get("cursor"); token != "" {
		var key anchor
		if err := h.Cursor.Decode(token, b, &key); err != nil {
			response.Fail(c, err)
			return
		}
		if !scope.ValidID(key.ID) || key.At.IsZero() {
			response.Fail(c, pe.Contract(400, "platform.cursor_invalid", nil))
			return
		}
		q.BeforeAt = key.At
		q.BeforeID = key.ID
	}

	page, err := h.Service.ListRules(c.Request.Context(), principal(p), ruleSetID, q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	sendPage(h, c, b, page, func(v d.Rule) anchor { return anchor{At: v.UpdatedAt, ID: v.ID} })
}

func (h *Handler) listReports(c *gin.Context) {
	p, ok := h.authenticate(c, "creator", "creator")
	if !ok {
		return
	}

	q := d.ListQuery{Limit: 20}
	values := c.Request.URL.Query()
	allowed := map[string]bool{"limit": true, "cursor": true, "chapter_id": true, "state": true}
	for k, v := range values {
		if !allowed[k] || len(v) != 1 || v[0] == "" {
			response.Fail(c, bad())
			return
		}
	}
	if v := values.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 100 {
			response.Fail(c, bad())
			return
		}
		q.Limit = n
	}
	if v := values.Get("chapter_id"); v != "" {
		if !scope.ValidID(v) {
			response.Fail(c, bad())
			return
		}
		q.ChapterID = v
	}
	if v := values.Get("state"); v != "" {
		switch v {
		case d.ReportStatePending, d.ReportStateRunning, d.ReportStateSucceeded, d.ReportStateFailed, d.ReportStateCancelled:
		default:
			response.Fail(c, bad())
			return
		}
		q.State = v
	}

	filters, _ := json.Marshal(map[string]any{"chapter_id": q.ChapterID, "state": q.State})
	b := cursor.Binding{
		TenantID:           p.TenantID,
		ActorID:            p.UserID,
		Audience:           p.Audience,
		PermissionRevision: p.MembershipRevision,
		Operation:          "compliance.list.reports",
		Filters:            filters,
		Sort:               "updated_at:desc,id:desc",
	}

	if token := values.Get("cursor"); token != "" {
		var key anchor
		if err := h.Cursor.Decode(token, b, &key); err != nil {
			response.Fail(c, err)
			return
		}
		if !scope.ValidID(key.ID) || key.At.IsZero() {
			response.Fail(c, pe.Contract(400, "platform.cursor_invalid", nil))
			return
		}
		q.BeforeAt = key.At
		q.BeforeID = key.ID
	}

	page, err := h.Service.ListReports(c.Request.Context(), principal(p), q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	sendPage(h, c, b, page, func(v d.Report) anchor { return anchor{At: v.UpdatedAt, ID: v.ID} })
}

func (h *Handler) listIssues(c *gin.Context) {
	p, ok := h.authenticate(c, "creator", "creator")
	if !ok {
		return
	}

	reportID := c.Param("report_id")
	q := d.ListQuery{Limit: 20}
	values := c.Request.URL.Query()
	allowed := map[string]bool{"limit": true, "cursor": true, "state": true}
	for k, v := range values {
		if !allowed[k] || len(v) != 1 || v[0] == "" {
			response.Fail(c, bad())
			return
		}
	}
	if v := values.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 100 {
			response.Fail(c, bad())
			return
		}
		q.Limit = n
	}
	if v := values.Get("state"); v != "" {
		if v != d.IssueStateOpen && v != d.IssueStateIgnored {
			response.Fail(c, bad())
			return
		}
		q.State = v
	}

	filters, _ := json.Marshal(map[string]any{"report_id": reportID, "state": q.State})
	b := cursor.Binding{
		TenantID:           p.TenantID,
		ActorID:            p.UserID,
		Audience:           p.Audience,
		PermissionRevision: p.MembershipRevision,
		Operation:          "compliance.list.issues",
		Filters:            filters,
		Sort:               "updated_at:desc,id:desc",
	}

	if token := values.Get("cursor"); token != "" {
		var key anchor
		if err := h.Cursor.Decode(token, b, &key); err != nil {
			response.Fail(c, err)
			return
		}
		if !scope.ValidID(key.ID) || key.At.IsZero() {
			response.Fail(c, pe.Contract(400, "platform.cursor_invalid", nil))
			return
		}
		q.BeforeAt = key.At
		q.BeforeID = key.ID
	}

	page, err := h.Service.ListIssues(c.Request.Context(), principal(p), reportID, q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	sendPage(h, c, b, page, func(v d.Issue) anchor { return anchor{At: v.UpdatedAt, ID: v.ID} })
}

func (h *Handler) listDecisions(c *gin.Context) {
	p, ok := h.authenticate(c, "creator", "creator")
	if !ok {
		return
	}

	reportID := c.Param("report_id")
	issueID := c.Param("issue_id")
	q := d.ListQuery{Limit: 20}
	values := c.Request.URL.Query()
	allowed := map[string]bool{"limit": true, "cursor": true}
	for k, v := range values {
		if !allowed[k] || len(v) != 1 || v[0] == "" {
			response.Fail(c, bad())
			return
		}
	}
	if v := values.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 100 {
			response.Fail(c, bad())
			return
		}
		q.Limit = n
	}

	filters, _ := json.Marshal(map[string]any{"report_id": reportID, "issue_id": issueID})
	b := cursor.Binding{
		TenantID:           p.TenantID,
		ActorID:            p.UserID,
		Audience:           p.Audience,
		PermissionRevision: p.MembershipRevision,
		Operation:          "compliance.list.decisions",
		Filters:            filters,
		Sort:               "created_at:desc,id:desc",
	}

	if token := values.Get("cursor"); token != "" {
		var key anchor
		if err := h.Cursor.Decode(token, b, &key); err != nil {
			response.Fail(c, err)
			return
		}
		if !scope.ValidID(key.ID) || key.At.IsZero() {
			response.Fail(c, pe.Contract(400, "platform.cursor_invalid", nil))
			return
		}
		q.BeforeAt = key.At
		q.BeforeID = key.ID
	}

	page, err := h.Service.ListIssueDecisions(c.Request.Context(), principal(p), reportID, issueID, q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	sendPage(h, c, b, page, func(v d.IssueDecision) anchor { return anchor{At: v.CreatedAt, ID: v.ID} })
}

func (h *Handler) listReviews(c *gin.Context) {
	p, ok := h.authenticate(c, "creator", "creator")
	if !ok {
		return
	}

	reportID := c.Param("report_id")
	q := d.ListQuery{Limit: 20}
	values := c.Request.URL.Query()
	allowed := map[string]bool{"limit": true, "cursor": true}
	for k, v := range values {
		if !allowed[k] || len(v) != 1 || v[0] == "" {
			response.Fail(c, bad())
			return
		}
	}
	if v := values.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 100 {
			response.Fail(c, bad())
			return
		}
		q.Limit = n
	}

	filters, _ := json.Marshal(map[string]any{"report_id": reportID})
	b := cursor.Binding{
		TenantID:           p.TenantID,
		ActorID:            p.UserID,
		Audience:           p.Audience,
		PermissionRevision: p.MembershipRevision,
		Operation:          "compliance.list.reviews",
		Filters:            filters,
		Sort:               "created_at:desc,id:desc",
	}

	if token := values.Get("cursor"); token != "" {
		var key anchor
		if err := h.Cursor.Decode(token, b, &key); err != nil {
			response.Fail(c, err)
			return
		}
		if !scope.ValidID(key.ID) || key.At.IsZero() {
			response.Fail(c, pe.Contract(400, "platform.cursor_invalid", nil))
			return
		}
		q.BeforeAt = key.At
		q.BeforeID = key.ID
	}

	page, err := h.Service.ListReviews(c.Request.Context(), principal(p), reportID, q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	sendPage(h, c, b, page, func(v d.Review) anchor { return anchor{At: v.CreatedAt, ID: v.ID} })
}

func (h *Handler) listRewrites(c *gin.Context) {
	p, ok := h.authenticate(c, "creator", "creator")
	if !ok {
		return
	}

	q := d.ListQuery{Limit: 20}
	values := c.Request.URL.Query()
	allowed := map[string]bool{"limit": true, "cursor": true, "report_id": true}
	for k, v := range values {
		if !allowed[k] || len(v) != 1 || v[0] == "" {
			response.Fail(c, bad())
			return
		}
	}
	if v := values.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 100 {
			response.Fail(c, bad())
			return
		}
		q.Limit = n
	}
	if v := values.Get("report_id"); v != "" {
		if !scope.ValidID(v) {
			response.Fail(c, bad())
			return
		}
		q.ReportID = v
	}

	filters, _ := json.Marshal(map[string]any{"report_id": q.ReportID})
	b := cursor.Binding{
		TenantID:           p.TenantID,
		ActorID:            p.UserID,
		Audience:           p.Audience,
		PermissionRevision: p.MembershipRevision,
		Operation:          "compliance.list.rewrites",
		Filters:            filters,
		Sort:               "updated_at:desc,id:desc",
	}

	if token := values.Get("cursor"); token != "" {
		var key anchor
		if err := h.Cursor.Decode(token, b, &key); err != nil {
			response.Fail(c, err)
			return
		}
		if !scope.ValidID(key.ID) || key.At.IsZero() {
			response.Fail(c, pe.Contract(400, "platform.cursor_invalid", nil))
			return
		}
		q.BeforeAt = key.At
		q.BeforeID = key.ID
	}

	page, err := h.Service.ListRewrites(c.Request.Context(), principal(p), q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	sendPage(h, c, b, page, func(v d.Rewrite) anchor { return anchor{At: v.UpdatedAt, ID: v.ID} })
}
