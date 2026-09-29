package http

import (
	d "InkLore/internal/compliance/domain"
	"InkLore/internal/platform/response"
	"context"

	"github.com/gin-gonic/gin"
)

func (h *Handler) RegisterRoutes(r gin.IRoutes) {
	// Rule sets
	r.POST("/v1/compliance/rule-sets", func(c *gin.Context) {
		command(h, c, "admin", "operator", 201, h.Service.CreateRuleSet)
	})
	r.GET("/v1/compliance/rule-sets", h.listRuleSets)
	r.GET("/v1/compliance/rule-sets/:rule_set_id", func(c *gin.Context) {
		get(h, c, "admin", "operator", "rule_set_id", h.Service.GetRuleSet)
	})
	r.POST("/v1/compliance/rule-sets/:rule_set_id/publish", func(c *gin.Context) {
		p, ok := h.authenticate(c, "admin", "operator")
		if !ok {
			return
		}
		input, err := decode[d.Expected](c)
		if err != nil {
			response.Fail(c, err)
			return
		}
		runCommand(h, c, p, "operator", input, 200, func(ctx context.Context) (any, error) {
			return h.Service.PublishRuleSet(ctx, principal(p), c.Param("rule_set_id"), input)
		})
	})

	// Rules
	r.POST("/v1/compliance/rule-sets/:rule_set_id/rules", func(c *gin.Context) {
		p, ok := h.authenticate(c, "admin", "operator")
		if !ok {
			return
		}
		input, err := decode[d.RuleCreate](c)
		if err != nil {
			response.Fail(c, err)
			return
		}
		runCommand(h, c, p, "operator", input, 201, func(ctx context.Context) (any, error) {
			return h.Service.CreateRule(ctx, principal(p), c.Param("rule_set_id"), input)
		})
	})
	r.GET("/v1/compliance/rule-sets/:rule_set_id/rules", h.listRules)
	r.PUT("/v1/compliance/rule-sets/:rule_set_id/rules/:rule_id", func(c *gin.Context) {
		p, ok := h.authenticate(c, "admin", "operator")
		if !ok {
			return
		}
		input, err := decode[d.RuleUpdate](c)
		if err != nil {
			response.Fail(c, err)
			return
		}
		runCommand(h, c, p, "operator", input, 200, func(ctx context.Context) (any, error) {
			return h.Service.UpdateRule(ctx, principal(p), c.Param("rule_set_id"), c.Param("rule_id"), input)
		})
	})
	r.DELETE("/v1/compliance/rule-sets/:rule_set_id/rules/:rule_id", func(c *gin.Context) {
		remove(h, c, "admin", "operator", h.Service.DeleteRule)
	})

	// Fast scan
	r.POST("/v1/compliance/scans", func(c *gin.Context) {
		command(h, c, "creator", "creator", 200, h.Service.Scan)
	})

	// Reports
	r.POST("/v1/compliance/reports", func(c *gin.Context) {
		command(h, c, "creator", "creator", 201, h.Service.CreateReport)
	})
	r.GET("/v1/compliance/reports", h.listReports)
	r.GET("/v1/compliance/reports/:report_id", func(c *gin.Context) {
		get(h, c, "creator", "creator", "report_id", h.Service.GetReport)
	})

	// Issues & Decisions
	r.GET("/v1/compliance/reports/:report_id/issues", h.listIssues)
	r.POST("/v1/compliance/reports/:report_id/issues/:issue_id/ignore", func(c *gin.Context) {
		p, ok := h.authenticate(c, "creator", "creator")
		if !ok {
			return
		}
		input, err := decode[d.ReasonCommand](c)
		if err != nil {
			response.Fail(c, err)
			return
		}
		runCommand(h, c, p, "creator", input, 200, func(ctx context.Context) (any, error) {
			return h.Service.IgnoreIssue(ctx, principal(p), c.Param("report_id"), c.Param("issue_id"), input)
		})
	})
	r.POST("/v1/compliance/reports/:report_id/issues/:issue_id/reopen", func(c *gin.Context) {
		p, ok := h.authenticate(c, "creator", "creator")
		if !ok {
			return
		}
		input, err := decode[d.ReasonCommand](c)
		if err != nil {
			response.Fail(c, err)
			return
		}
		runCommand(h, c, p, "creator", input, 200, func(ctx context.Context) (any, error) {
			return h.Service.ReopenIssue(ctx, principal(p), c.Param("report_id"), c.Param("issue_id"), input)
		})
	})
	r.GET("/v1/compliance/reports/:report_id/issues/:issue_id/decisions", h.listDecisions)

	// Reviews
	r.POST("/v1/compliance/reports/:report_id/reviews", func(c *gin.Context) {
		p, ok := h.authenticate(c, "creator", "creator")
		if !ok {
			return
		}
		input, err := decode[d.ReviewCreate](c)
		if err != nil {
			response.Fail(c, err)
			return
		}
		runCommand(h, c, p, "creator", input, 201, func(ctx context.Context) (any, error) {
			return h.Service.CreateReview(ctx, principal(p), c.Param("report_id"), input)
		})
	})
	r.GET("/v1/compliance/reports/:report_id/reviews", h.listReviews)

	// Rewrites
	r.POST("/v1/compliance/rewrites", func(c *gin.Context) {
		command(h, c, "creator", "creator", 201, h.Service.CreateRewrite)
	})
	r.GET("/v1/compliance/rewrites", h.listRewrites)
	r.GET("/v1/compliance/rewrites/:rewrite_id", func(c *gin.Context) {
		get(h, c, "creator", "creator", "rewrite_id", h.Service.GetRewrite)
	})
}
