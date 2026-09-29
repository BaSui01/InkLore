package transport

import (
	a "InkLore/internal/content/application"
	d "InkLore/internal/content/domain"
	"InkLore/internal/platform/idempotency"
	"InkLore/internal/platform/identity"

	"github.com/gin-gonic/gin"
)

func (h *Handler) RegisterRoutes(r gin.IRoutes) {
	r.GET("/v1/content/novels", readRoute(h, []string{"limit", "cursor", "status"}, func(c *gin.Context, p identity.Principal) (any, error) {
		q, e := listQuery(c)
		if e != nil {
			return nil, e
		}
		return h.s.ListNovels(c.Request.Context(), p, q)
	}))
	r.GET("/v1/content/novels/:novel_id", readRoute(h, []string{}, func(c *gin.Context, p identity.Principal) (any, error) {
		return h.s.GetNovel(c.Request.Context(), p, c.Param("novel_id"))
	}))
	r.GET("/v1/content/novels/:novel_id/volumes", readRoute(h, []string{"limit", "cursor", "deleted"}, func(c *gin.Context, p identity.Principal) (any, error) {
		q, e := listQuery(c)
		if e != nil {
			return nil, e
		}
		return h.s.ListVolumes(c.Request.Context(), p, c.Param("novel_id"), q)
	}))
	r.GET("/v1/content/volumes/:volume_id", readRoute(h, []string{}, func(c *gin.Context, p identity.Principal) (any, error) {
		return h.s.GetVolume(c.Request.Context(), p, c.Param("volume_id"))
	}))
	r.GET("/v1/content/novels/:novel_id/chapters", readRoute(h, []string{"limit", "cursor", "deleted"}, func(c *gin.Context, p identity.Principal) (any, error) {
		q, e := listQuery(c)
		if e != nil {
			return nil, e
		}
		return h.s.ListChapters(c.Request.Context(), p, c.Param("novel_id"), q)
	}))
	r.GET("/v1/content/chapters/:chapter_id", readRoute(h, []string{}, func(c *gin.Context, p identity.Principal) (any, error) {
		return h.s.GetChapter(c.Request.Context(), p, c.Param("chapter_id"))
	}))
	r.GET("/v1/content/novels/:novel_id/organization", readRoute(h, []string{}, func(c *gin.Context, p identity.Principal) (any, error) {
		return h.s.GetOrganization(c.Request.Context(), p, c.Param("novel_id"))
	}))
	r.GET("/v1/content/chapters/:chapter_id/body", readRoute(h, []string{}, func(c *gin.Context, p identity.Principal) (any, error) {
		return h.s.GetBody(c.Request.Context(), p, c.Param("chapter_id"))
	}))
	r.GET("/v1/content/chapters/:chapter_id/versions", readRoute(h, []string{"limit", "cursor"}, func(c *gin.Context, p identity.Principal) (any, error) {
		q, e := listQuery(c)
		if e != nil {
			return nil, e
		}
		return h.s.ListVersions(c.Request.Context(), p, c.Param("chapter_id"), q)
	}))
	r.GET("/v1/content/chapters/:chapter_id/versions/:version_id", readRoute(h, []string{}, func(c *gin.Context, p identity.Principal) (any, error) {
		return h.s.GetVersion(c.Request.Context(), p, c.Param("chapter_id"), c.Param("version_id"))
	}))
	r.GET("/v1/content/chapters/:chapter_id/diff", readRoute(h, []string{"from_version_id", "to_version_id"}, func(c *gin.Context, p identity.Principal) (any, error) {
		return h.s.DiffVersions(c.Request.Context(), p, c.Param("chapter_id"), c.Query("from_version_id"), c.Query("to_version_id"))
	}))
	r.POST("/v1/content/novels", writeRoute[a.CreateNovel](h, func(c *gin.Context, cmd a.Command, r a.CreateNovel) (idempotency.Result, error) {
		return h.s.CreateNovel(c.Request.Context(), cmd, r)
	}))
	r.PUT("/v1/content/novels/:novel_id", writeRoute[a.UpdateNovel](h, func(c *gin.Context, cmd a.Command, r a.UpdateNovel) (idempotency.Result, error) {
		return h.s.UpdateNovel(c.Request.Context(), cmd, c.Param("novel_id"), r)
	}))
	r.DELETE("/v1/content/novels/:novel_id", deleteRoute(h, func(c *gin.Context, cmd a.Command, r a.RevisionCommand) (idempotency.Result, error) {
		return h.s.DeleteNovel(c.Request.Context(), cmd, c.Param("novel_id"), r)
	}))
	r.POST("/v1/content/novels/:novel_id/archive", writeRoute[a.RevisionCommand](h, func(c *gin.Context, cmd a.Command, r a.RevisionCommand) (idempotency.Result, error) {
		return h.s.ArchiveNovel(c.Request.Context(), cmd, c.Param("novel_id"), r)
	}))
	r.POST("/v1/content/novels/:novel_id/restore", writeRoute[a.RevisionCommand](h, func(c *gin.Context, cmd a.Command, r a.RevisionCommand) (idempotency.Result, error) {
		return h.s.RestoreNovel(c.Request.Context(), cmd, c.Param("novel_id"), r)
	}))
	r.POST("/v1/content/novels/:novel_id/volumes", writeRoute[a.CreateVolume](h, func(c *gin.Context, cmd a.Command, r a.CreateVolume) (idempotency.Result, error) {
		return h.s.CreateVolume(c.Request.Context(), cmd, c.Param("novel_id"), r)
	}))
	r.PUT("/v1/content/volumes/:volume_id", writeRoute[a.UpdateVolume](h, func(c *gin.Context, cmd a.Command, r a.UpdateVolume) (idempotency.Result, error) {
		return h.s.UpdateVolume(c.Request.Context(), cmd, c.Param("volume_id"), r)
	}))
	r.DELETE("/v1/content/volumes/:volume_id", deleteRoute(h, func(c *gin.Context, cmd a.Command, r a.RevisionCommand) (idempotency.Result, error) {
		return h.s.DeleteVolume(c.Request.Context(), cmd, c.Param("volume_id"), r)
	}))
	r.POST("/v1/content/volumes/:volume_id/restore", writeRoute[a.RevisionCommand](h, func(c *gin.Context, cmd a.Command, r a.RevisionCommand) (idempotency.Result, error) {
		return h.s.RestoreVolume(c.Request.Context(), cmd, c.Param("volume_id"), r)
	}))
	r.POST("/v1/content/novels/:novel_id/chapters", writeRoute[a.CreateChapter](h, func(c *gin.Context, cmd a.Command, r a.CreateChapter) (idempotency.Result, error) {
		return h.s.CreateChapter(c.Request.Context(), cmd, c.Param("novel_id"), r)
	}))
	r.PUT("/v1/content/chapters/:chapter_id", writeRoute[a.UpdateChapter](h, func(c *gin.Context, cmd a.Command, r a.UpdateChapter) (idempotency.Result, error) {
		return h.s.UpdateChapter(c.Request.Context(), cmd, c.Param("chapter_id"), r)
	}))
	r.DELETE("/v1/content/chapters/:chapter_id", deleteRoute(h, func(c *gin.Context, cmd a.Command, r a.RevisionCommand) (idempotency.Result, error) {
		return h.s.DeleteChapter(c.Request.Context(), cmd, c.Param("chapter_id"), r)
	}))
	r.POST("/v1/content/chapters/:chapter_id/restore", writeRoute[a.RevisionCommand](h, func(c *gin.Context, cmd a.Command, r a.RevisionCommand) (idempotency.Result, error) {
		return h.s.RestoreChapter(c.Request.Context(), cmd, c.Param("chapter_id"), r)
	}))
	r.POST("/v1/content/chapters/:chapter_id/confirm", writeRoute[a.Confirm](h, func(c *gin.Context, cmd a.Command, r a.Confirm) (idempotency.Result, error) {
		return h.s.ConfirmChapter(c.Request.Context(), cmd, c.Param("chapter_id"), r)
	}))
	r.PUT("/v1/content/chapters/:chapter_id/body", writeRoute[a.SaveBody](h, func(c *gin.Context, cmd a.Command, r a.SaveBody) (idempotency.Result, error) {
		return h.s.SaveBody(c.Request.Context(), cmd, c.Param("chapter_id"), r)
	}))
	r.POST("/v1/content/chapters/:chapter_id/rollback", writeRoute[a.Rollback](h, func(c *gin.Context, cmd a.Command, r a.Rollback) (idempotency.Result, error) {
		return h.s.RollbackChapter(c.Request.Context(), cmd, c.Param("chapter_id"), r)
	}))
	r.POST("/v1/content/novels/:novel_id/organization/reorder", writeRoute[a.Reorder](h, func(c *gin.Context, cmd a.Command, r a.Reorder) (idempotency.Result, error) {
		return h.s.ReorderOrganization(c.Request.Context(), cmd, c.Param("novel_id"), r)
	}))
	r.POST("/v1/content/novels/:novel_id/imports", writeRoute[d.CreateImport](h, func(c *gin.Context, cmd a.Command, r d.CreateImport) (idempotency.Result, error) {
		return h.s.CreateImport(c.Request.Context(), cmd, c.Param("novel_id"), r)
	}))
	r.GET("/v1/content/imports/:job_id/preview", readRoute(h, []string{"limit", "cursor"}, func(c *gin.Context, p identity.Principal) (any, error) {
		q, e := listQuery(c)
		if e != nil {
			return nil, e
		}
		return h.s.PreviewImport(c.Request.Context(), p, c.Param("job_id"), q)
	}))
	r.POST("/v1/content/imports/:job_id/commit", writeRoute[d.CommitImport](h, func(c *gin.Context, cmd a.Command, r d.CommitImport) (idempotency.Result, error) {
		return h.s.CommitImport(c.Request.Context(), cmd, c.Param("job_id"), r)
	}))
	r.POST("/v1/content/novels/:novel_id/exports", writeRoute[d.CreateExport](h, func(c *gin.Context, cmd a.Command, r d.CreateExport) (idempotency.Result, error) {
		return h.s.CreateExport(c.Request.Context(), cmd, c.Param("novel_id"), r)
	}))
	r.POST("/v1/content/novels/:novel_id/copy", writeRoute[d.CopyNovel](h, func(c *gin.Context, cmd a.Command, r d.CopyNovel) (idempotency.Result, error) {
		return h.s.CopyNovel(c.Request.Context(), cmd, c.Param("novel_id"), r)
	}))
	r.GET("/v1/content/novels/:novel_id/jobs", readRoute(h, []string{"limit", "cursor"}, func(c *gin.Context, p identity.Principal) (any, error) {
		q, e := listQuery(c)
		if e != nil {
			return nil, e
		}
		return h.s.ListJobs(c.Request.Context(), p, c.Param("novel_id"), q)
	}))
	r.GET("/v1/content/jobs/:job_id", readRoute(h, []string{}, func(c *gin.Context, p identity.Principal) (any, error) {
		return h.s.GetJob(c.Request.Context(), p, c.Param("job_id"))
	}))
	r.POST("/v1/content/jobs/:job_id/cancel", writeRoute[a.RevisionCommand](h, func(c *gin.Context, cmd a.Command, r a.RevisionCommand) (idempotency.Result, error) {
		return h.s.CancelJob(c.Request.Context(), cmd, c.Param("job_id"), r)
	}))
	r.GET("/v1/content/novels/:novel_id/search", readRoute(h, []string{"q", "limit", "cursor"}, func(c *gin.Context, p identity.Principal) (any, error) {
		qStr := c.Query("q")
		if qStr == "" {
			return nil, invalid()
		}
		q, e := listQuery(c)
		if e != nil {
			return nil, e
		}
		return h.s.Search(c.Request.Context(), p, c.Param("novel_id"), qStr, q)
	}))
	r.GET("/v1/content/novels/:novel_id/statistics", readRoute(h, []string{"from", "to"}, func(c *gin.Context, p identity.Principal) (any, error) {
		fromStr := c.Query("from")
		toStr := c.Query("to")
		if fromStr == "" || toStr == "" {
			return nil, invalid()
		}
		return h.s.Statistics(c.Request.Context(), p, c.Param("novel_id"), fromStr, toStr)
	}))
}
