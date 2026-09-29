package http

import (
	d "InkLore/internal/inspiration/domain"
	"InkLore/internal/platform/scope"
	"context"

	"github.com/gin-gonic/gin"
)

func (h *Handler) RegisterRoutes(r gin.IRoutes) {
	// Assets commands and queries
	r.POST("/v1/inspiration/assets", func(c *gin.Context) {
		command(h, c, "inspiration_asset", "", 201, func(ctx context.Context, p scope.Principal, _ string, in d.AssetCreate) (d.Asset, error) {
			return h.Service.CreateAsset(ctx, p, in)
		})
	})
	r.PUT("/v1/inspiration/assets/:asset_id", func(c *gin.Context) {
		command(h, c, "inspiration_asset", "asset_id", 200, h.Service.UpdateAsset)
	})
	r.DELETE("/v1/inspiration/assets/:asset_id", func(c *gin.Context) {
		remove(h, c, "inspiration_asset", "asset_id", h.Service.DeleteAsset)
	})
	r.POST("/v1/inspiration/assets/:asset_id/restore", func(c *gin.Context) {
		command(h, c, "inspiration_asset", "asset_id", 200, h.Service.RestoreAsset)
	})
	r.GET("/v1/inspiration/assets", h.listAssets)
	r.GET("/v1/inspiration/assets/:asset_id", h.getAsset)
	r.GET("/v1/inspiration/assets/:asset_id/segments", h.listSegments)

	// Analyses commands and queries
	r.POST("/v1/inspiration/analyses", func(c *gin.Context) {
		command(h, c, "inspiration_analysis", "", 201, func(ctx context.Context, p scope.Principal, _ string, in d.AnalysisCreate) (d.Analysis, error) {
			return h.Service.CreateAnalysis(ctx, p, in)
		})
	})
	r.GET("/v1/inspiration/analyses", h.listAnalyses)
	r.GET("/v1/inspiration/analyses/:analysis_id", func(c *gin.Context) {
		get(h, c, "analysis_id", h.Service.GetAnalysis)
	})
	r.GET("/v1/inspiration/analyses/:analysis_id/report", func(c *gin.Context) {
		get(h, c, "analysis_id", h.Service.GetReport)
	})

	// Styles commands and queries
	r.POST("/v1/inspiration/styles", func(c *gin.Context) {
		command(h, c, "inspiration_style", "", 201, func(ctx context.Context, p scope.Principal, _ string, in d.StyleCreate) (d.Style, error) {
			return h.Service.CreateStyle(ctx, p, in)
		})
	})
	r.PUT("/v1/inspiration/styles/:style_id", func(c *gin.Context) {
		command(h, c, "inspiration_style", "style_id", 200, h.Service.UpdateStyle)
	})
	r.DELETE("/v1/inspiration/styles/:style_id", func(c *gin.Context) {
		remove(h, c, "inspiration_style", "style_id", h.Service.DeleteStyle)
	})
	r.POST("/v1/inspiration/styles/:style_id/restore", func(c *gin.Context) {
		command(h, c, "inspiration_style", "style_id", 200, h.Service.RestoreStyle)
	})
	r.GET("/v1/inspiration/styles", h.listStyles)
	r.GET("/v1/inspiration/styles/:style_id", h.getStyle)
	r.GET("/v1/inspiration/styles/:style_id/versions", h.listStyleVersions)
	r.GET("/v1/inspiration/styles/:style_id/versions/:version_id", h.getStyleVersion)

	// Cards commands and queries
	r.POST("/v1/inspiration/cards", func(c *gin.Context) {
		command(h, c, "inspiration_card", "", 201, func(ctx context.Context, p scope.Principal, _ string, in d.CardCreate) (d.Card, error) {
			return h.Service.CreateCard(ctx, p, in)
		})
	})
	r.PUT("/v1/inspiration/cards/:card_id", func(c *gin.Context) {
		command(h, c, "inspiration_card", "card_id", 200, h.Service.UpdateCard)
	})
	r.DELETE("/v1/inspiration/cards/:card_id", func(c *gin.Context) {
		remove(h, c, "inspiration_card", "card_id", h.Service.DeleteCard)
	})
	r.POST("/v1/inspiration/cards/:card_id/restore", func(c *gin.Context) {
		command(h, c, "inspiration_card", "card_id", 200, h.Service.RestoreCard)
	})
	r.POST("/v1/inspiration/cards/:card_id/state", func(c *gin.Context) {
		command(h, c, "inspiration_card", "card_id", 200, h.Service.SetCardState)
	})
	r.GET("/v1/inspiration/cards", h.listCards)
	r.GET("/v1/inspiration/cards/:card_id", h.getCard)

	// Adoptions commands and queries
	r.POST("/v1/inspiration/cards/:card_id/adoptions", func(c *gin.Context) {
		command(h, c, "inspiration_card", "card_id", 201, h.Service.RecordAdoption)
	})
	r.POST("/v1/inspiration/adoptions/:adoption_id/revoke", func(c *gin.Context) {
		command(h, c, "inspiration_adoption", "adoption_id", 200, h.Service.RevokeAdoption)
	})
	r.GET("/v1/inspiration/cards/:card_id/adoptions", h.listAdoptions)
}
