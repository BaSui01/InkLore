package application

import (
	d "InkLore/internal/content/domain"
	"InkLore/internal/platform/idempotency"
	"context"
	"strconv"

	"github.com/google/uuid"
)

func bind(c *Command, method, path string, body any) {
	c.Request.Method = method
	c.Request.Path = path
	c.Request.Query = nil
	if body != nil {
		c.Request.Body = requestBody(body)
	} else {
		c.Request.Body = nil
	}
}
func bindDelete(c *Command, path string, r d.RevisionCommand) {
	bind(c, "DELETE", path, nil)
	c.Request.Query = map[string][]string{"expected_revision": {strconv.FormatInt(r.ExpectedRevision, 10)}}
}
func (s *Service) CreateNovel(ctx context.Context, c Command, r d.CreateNovel) (idempotency.Result, error) {
	bind(&c, "POST", "/v1/content/novels", r)
	return s.execute(ctx, c, "", "", false, false, 201, func(tx context.Context, o Owner) (any, error) {
		if e := d.Metadata(r.Title, r.Summary, r.Category, r.TargetWords); e != nil {
			return nil, e
		}
		if e := d.CompletionDate(r.TargetCompletionDate); e != nil {
			return nil, e
		}
		now := s.now()
		n := d.Novel{ID: uuid.NewString(), TenantID: o.TenantID, OwnerID: o.ActorID, CreatedAt: now, UpdatedAt: now, Revision: 1, Title: r.Title, Summary: r.Summary, Category: r.Category, TargetWords: r.TargetWords, TargetCompletionDate: r.TargetCompletionDate, Status: "active"}
		o.NovelID = n.ID
		if e := s.o.Repo.InsertNovel(tx, o, n); e != nil {
			return nil, e
		}
		if e := s.projectNovel(tx, n, 0); e != nil {
			return nil, e
		}
		if e := s.o.World.EnsureWorld(tx, Principal(c.Principal), n.ID); e != nil {
			return nil, e
		}
		return n, s.event(tx, c.Principal, "novel.created", "novel", n.ID, n.ID, n.Revision)
	})
}
func (s *Service) UpdateNovel(ctx context.Context, c Command, id string, r d.UpdateNovel) (idempotency.Result, error) {
	bind(&c, "PUT", "/v1/content/novels/"+id, r)
	return s.execute(ctx, c, "novel", id, false, false, 200, func(tx context.Context, o Owner) (any, error) {
		if e := d.Metadata(r.Title, r.Summary, r.Category, r.TargetWords); e != nil {
			return nil, e
		}
		if e := d.CompletionDate(r.TargetCompletionDate); e != nil {
			return nil, e
		}
		n, e := s.active(tx, o)
		if e != nil {
			return nil, e
		}
		if e = d.CAS(n.Revision, r.ExpectedRevision); e != nil {
			return nil, e
		}
		n.Title, n.Summary, n.Category, n.TargetWords = r.Title, r.Summary, r.Category, r.TargetWords
		n.TargetCompletionDate = r.TargetCompletionDate
		if e = s.bumpNovel(tx, o, &n); e != nil {
			return nil, e
		}
		return n, s.event(tx, c.Principal, "novel.updated", "novel", id, id, n.Revision)
	})
}
func (s *Service) ArchiveNovel(ctx context.Context, c Command, id string, r d.RevisionCommand) (idempotency.Result, error) {
	bind(&c, "POST", "/v1/content/novels/"+id+"/archive", r)
	return s.novelState(ctx, c, id, r, "archive")
}
func (s *Service) RestoreNovel(ctx context.Context, c Command, id string, r d.RevisionCommand) (idempotency.Result, error) {
	bind(&c, "POST", "/v1/content/novels/"+id+"/restore", r)
	return s.novelState(ctx, c, id, r, "restore")
}
func (s *Service) DeleteNovel(ctx context.Context, c Command, id string, r d.RevisionCommand) (idempotency.Result, error) {
	bindDelete(&c, "/v1/content/novels/"+id, r)
	return s.novelState(ctx, c, id, r, "delete")
}
func (s *Service) novelState(ctx context.Context, c Command, id string, r d.RevisionCommand, action string) (idempotency.Result, error) {
	return s.execute(ctx, c, "novel", id, action != "archive", false, 200, func(tx context.Context, o Owner) (any, error) {
		n, e := s.o.Repo.Novel(tx, o, id, true)
		if e != nil {
			return nil, e
		}
		if e = d.CAS(n.Revision, r.ExpectedRevision); e != nil {
			return nil, e
		}
		switch action {
		case "archive":
			if n.Status != "active" {
				return nil, d.State()
			}
			busy, e := s.o.Repo.HasActiveImport(tx, o)
			if e != nil {
				return nil, e
			}
			if busy {
				return nil, d.State()
			}
			n.Status = "archived"
		case "delete":
			if n.Status == "deleted" {
				return nil, d.State()
			}
			previous := n.Status
			now := s.now()
			n.PreviousStatus = &previous
			n.Status = "deleted"
			n.DeletedAt = &now
		case "restore":
			switch n.Status {
			case "archived":
				n.Status = "active"
			case "deleted":
				if !d.Restorable(n.DeletedAt, s.now()) || n.PreviousStatus == nil {
					return nil, d.Missing()
				}
				n.Status = *n.PreviousStatus
				n.PreviousStatus = nil
				n.DeletedAt = nil
			default:
				return nil, d.State()
			}
		}
		if e = s.bumpNovel(tx, o, &n); e != nil {
			return nil, e
		}
		return n, s.event(tx, c.Principal, "novel."+action, "novel", id, id, n.Revision)
	})
}
