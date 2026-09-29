package http

import (
	d "InkLore/internal/outline/domain"
	"InkLore/internal/platform/httpguard"
	"InkLore/internal/platform/response"
	"InkLore/internal/platform/scope"
	"context"

	"github.com/gin-gonic/gin"
)

func (h *Handler) RegisterRoutes(r gin.IRoutes) {
	r.POST("/v1/outline/novels/:novel_id/nodes", func(c *gin.Context) { command(h, c, "novel", "novel_id", 201, h.Service.CreateNode) })
	r.PUT("/v1/outline/nodes/:node_id", func(c *gin.Context) { command(h, c, "outline_node", "node_id", 200, h.Service.UpdateNode) })
	r.GET("/v1/outline/nodes/:node_id", func(c *gin.Context) { get(h, c, "node_id", h.Service.GetNode) })
	r.DELETE("/v1/outline/nodes/:node_id", func(c *gin.Context) { remove(h, c, "outline_node", "node_id", h.Service.DeleteNode) })
	r.POST("/v1/outline/nodes/:node_id/restore", func(c *gin.Context) { command(h, c, "outline_node", "node_id", 200, h.Service.RestoreNode) })
	r.GET("/v1/outline/novels/:novel_id/nodes", func(c *gin.Context) { h.listNodes(c) })
	r.POST("/v1/outline/novels/:novel_id/beats", func(c *gin.Context) { command(h, c, "novel", "novel_id", 201, h.Service.CreateBeat) })
	r.PUT("/v1/outline/beats/:beat_id", func(c *gin.Context) { command(h, c, "outline_beat", "beat_id", 200, h.Service.UpdateBeat) })
	r.GET("/v1/outline/beats/:beat_id", func(c *gin.Context) { get(h, c, "beat_id", h.Service.GetBeat) })
	r.DELETE("/v1/outline/beats/:beat_id", func(c *gin.Context) { remove(h, c, "outline_beat", "beat_id", h.Service.DeleteBeat) })
	r.POST("/v1/outline/beats/:beat_id/restore", func(c *gin.Context) { command(h, c, "outline_beat", "beat_id", 200, h.Service.RestoreBeat) })
	r.GET("/v1/outline/novels/:novel_id/beats", func(c *gin.Context) { h.listBeats(c) })
	r.POST("/v1/outline/novels/:novel_id/scenes", func(c *gin.Context) { command(h, c, "novel", "novel_id", 201, h.Service.CreateScene) })
	r.PUT("/v1/outline/scenes/:scene_id", func(c *gin.Context) { command(h, c, "outline_scene", "scene_id", 200, h.Service.UpdateScene) })
	r.GET("/v1/outline/scenes/:scene_id", func(c *gin.Context) { get(h, c, "scene_id", h.Service.GetScene) })
	r.DELETE("/v1/outline/scenes/:scene_id", func(c *gin.Context) { remove(h, c, "outline_scene", "scene_id", h.Service.DeleteScene) })
	r.POST("/v1/outline/scenes/:scene_id/restore", func(c *gin.Context) { command(h, c, "outline_scene", "scene_id", 200, h.Service.RestoreScene) })
	r.GET("/v1/outline/novels/:novel_id/scenes", func(c *gin.Context) { h.listScenes(c) })
	r.POST("/v1/outline/novels/:novel_id/clues", func(c *gin.Context) { command(h, c, "novel", "novel_id", 201, h.Service.CreateClue) })
	r.PUT("/v1/outline/clues/:clue_id", func(c *gin.Context) { command(h, c, "outline_clue", "clue_id", 200, h.Service.UpdateClue) })
	r.GET("/v1/outline/clues/:clue_id", func(c *gin.Context) { get(h, c, "clue_id", h.Service.GetClue) })
	r.DELETE("/v1/outline/clues/:clue_id", func(c *gin.Context) { remove(h, c, "outline_clue", "clue_id", h.Service.DeleteClue) })
	r.POST("/v1/outline/clues/:clue_id/restore", func(c *gin.Context) { command(h, c, "outline_clue", "clue_id", 200, h.Service.RestoreClue) })
	r.GET("/v1/outline/novels/:novel_id/clues", func(c *gin.Context) { h.listClues(c) })
	r.POST("/v1/outline/nodes/:node_id/reorder", func(c *gin.Context) { command(h, c, "outline_node", "node_id", 200, h.Service.Reorder) })
	r.POST("/v1/outline/nodes/:node_id/rebind", func(c *gin.Context) { command(h, c, "outline_node", "node_id", 200, h.Service.RebindNode) })
	r.POST("/v1/outline/clues/:clue_id/transition", func(c *gin.Context) { command(h, c, "outline_clue", "clue_id", 200, h.Service.TransitionClue) })
	r.GET("/v1/outline/clues/:clue_id/events", h.listClueEvents)
	r.GET("/v1/outline/novels/:novel_id/pacing", func(c *gin.Context) { get(h, c, "novel_id", h.Service.Pacing) })
}
func remove[O any](h *Handler, c *gin.Context, kind, param string, fn func(context.Context, scope.Principal, string, d.RevisionCommand) (O, error)) {
	p, ok := h.authenticate(c)
	if !ok {
		return
	}
	q := c.Request.URL.Query()
	if len(q) != 1 {
		response.Fail(c, bad())
		return
	}
	revision, err := httpguard.QueryRevision(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	input := d.RevisionCommand{ExpectedRevision: revision}
	runCommand(h, c, p, kind, c.Param(param), input, 200, func(ctx context.Context) (any, error) { return fn(ctx, principal(p), c.Param(param), input) })
}
