package application

import (
	d "InkLore/internal/content/domain"
	"InkLore/internal/platform/idempotency"
	"InkLore/internal/platform/scope"
	"context"

	"github.com/google/uuid"
)

func (s *Service) CreateChapter(ctx context.Context, c Command, novel string, r d.CreateChapter) (idempotency.Result, error) {
	bind(&c, "POST", "/v1/content/novels/"+novel+"/chapters", r)
	return s.execute(ctx, c, "novel", novel, false, true, 201, func(tx context.Context, o Owner) (any, error) {
		if !scope.ValidID(r.VolumeID) {
			return nil, invalid()
		}
		if e := d.Metadata(r.Title, "", "", 0); e != nil {
			return nil, e
		}
		n, e := s.active(tx, o)
		if e != nil {
			return nil, e
		}
		v, e := s.o.Repo.Volume(tx, o, r.VolumeID, true)
		if e != nil {
			return nil, e
		}
		if v.DeletedAt != nil {
			return nil, d.Missing()
		}
		org, e := s.o.Repo.Organization(tx, o)
		if e != nil {
			return nil, e
		}
		if len(org.Chapters) >= 2000 {
			return nil, d.Invalid()
		}
		var pos int64
		for _, ch := range org.Chapters {
			if ch.VolumeID == v.ID && ch.Position >= pos {
				pos = ch.Position + 1
			}
		}
		if pos > 1999 {
			return nil, d.Invalid()
		}
		now := s.now()
		_, hash, _, _ := d.Normalize("")
		ch := d.ChapterRecord{Chapter: d.Chapter{ID: uuid.NewString(), TenantID: o.TenantID, NovelID: novel, VolumeID: v.ID, Title: r.Title, Position: pos, Status: "draft", ContentHash: hash, CurrentVersionID: uuid.NewString(), Revision: 1, CreatedAt: now, UpdatedAt: now}}
		if e = s.o.Repo.InsertChapter(tx, o, ch); e != nil {
			return nil, e
		}
		ver := s.version(c, o, ch, "create", nil)
		if e = s.o.Repo.InsertVersion(tx, o, ver); e != nil {
			return nil, e
		}
		if e = s.projectVersion(tx, o, ver); e != nil {
			return nil, e
		}
		if e = s.projectChapter(tx, o, ch, 0); e != nil {
			return nil, e
		}
		if e = s.bumpNovel(tx, o, &n); e != nil {
			return nil, e
		}
		return ch.Chapter, s.event(tx, c.Principal, "chapter.created", "chapter", ch.ID, novel, ch.Revision)
	})
}
func (s *Service) version(c Command, o Owner, ch d.ChapterRecord, source string, restored *string) d.Version {
	now := s.now()
	actor := o.ActorID
	return d.Version{ID: ch.CurrentVersionID, TenantID: o.TenantID, CreatedAt: now, UpdatedAt: now, ChapterID: ch.ID, NovelID: o.NovelID, ChapterRevision: ch.Revision, Title: ch.Title, Body: ch.Body, ContentHash: ch.ContentHash, WordCount: ch.WordCount, Source: source, ActorID: &actor, RestoredFromVersionID: restored}
}
func (s *Service) saveChapter(ctx context.Context, o Owner, ch *d.ChapterRecord) error {
	old := ch.Revision
	if e := d.CAS(old, old); e != nil {
		return e
	}
	ch.Revision++
	ch.UpdatedAt = s.now()
	if e := s.o.Repo.PutChapter(ctx, o, *ch, old); e != nil {
		return e
	}
	return s.projectChapter(ctx, o, *ch, old)
}
func (s *Service) UpdateChapter(ctx context.Context, c Command, id string, r d.UpdateChapter) (idempotency.Result, error) {
	bind(&c, "PUT", "/v1/content/chapters/"+id, r)
	return s.writeBody(ctx, c, id, r.ExpectedRevision, "metadata", r.Title, nil)
}
func (s *Service) SaveBody(ctx context.Context, c Command, id string, r d.SaveBody) (idempotency.Result, error) {
	bind(&c, "PUT", "/v1/content/chapters/"+id+"/body", r)
	return s.writeBody(ctx, c, id, r.ExpectedRevision, "save", r.Body, nil)
}
func (s *Service) RollbackChapter(ctx context.Context, c Command, id string, r d.Rollback) (idempotency.Result, error) {
	bind(&c, "POST", "/v1/content/chapters/"+id+"/rollback", r)
	if !scope.ValidID(r.VersionID) {
		return idempotency.Result{}, invalid()
	}
	return s.writeBody(ctx, c, id, r.ExpectedRevision, "rollback", "", &r.VersionID)
}
func (s *Service) writeBody(ctx context.Context, c Command, id string, revision int64, mode, value string, restored *string) (idempotency.Result, error) {
	return s.execute(ctx, c, "chapter", id, false, false, 200, func(tx context.Context, o Owner) (any, error) {
		n, e := s.active(tx, o)
		if e != nil {
			return nil, e
		}
		ch, e := s.o.Repo.Chapter(tx, o, id, true)
		if e != nil {
			return nil, e
		}
		if ch.DeletedAt != nil {
			return nil, d.Missing()
		}
		if e = d.CAS(ch.Revision, revision); e != nil {
			return nil, e
		}
		source := mode
		switch mode {
		case "metadata":
			if e = d.Metadata(value, "", "", 0); e != nil {
				return nil, e
			}
			ch.Title = value
			value = ch.Body
			source = "save"
		case "rollback":
			ver, e := s.o.Repo.Version(tx, o, id, *restored)
			if e != nil {
				return nil, e
			}
			value = ver.Body
			ch.Title = ver.Title
		}
		ch.Body, ch.ContentHash, ch.WordCount, e = d.Normalize(value)
		if e != nil {
			return nil, e
		}
		ch.Status = "draft"
		ch.CurrentVersionID = uuid.NewString()
		old := ch.Revision
		ch.Revision++
		ch.UpdatedAt = s.now()
		// The business update intentionally precedes the snapshot insert: injecting a
		// failing InsertVersion proves the entire visible body update rolls back.
		if e = s.o.Repo.PutChapter(tx, o, ch, old); e != nil {
			return nil, e
		}
		ver := s.version(c, o, ch, source, restored)
		if e = s.o.Repo.InsertVersion(tx, o, ver); e != nil {
			return nil, e
		}
		if e = s.projectVersion(tx, o, ver); e != nil {
			return nil, e
		}
		if e = s.projectChapter(tx, o, ch, old); e != nil {
			return nil, e
		}
		if e = s.bumpNovel(tx, o, &n); e != nil {
			return nil, e
		}
		if e = s.event(tx, c.Principal, "chapter."+source, "chapter", id, o.NovelID, ch.Revision); e != nil {
			return nil, e
		}
		if mode == "metadata" {
			return ch.Chapter, nil
		}
		return d.Body{Chapter: ch.Chapter, Body: ch.Body}, nil
	})
}
func (s *Service) DeleteChapter(ctx context.Context, c Command, id string, r d.RevisionCommand) (idempotency.Result, error) {
	bindDelete(&c, "/v1/content/chapters/"+id, r)
	return s.chapterState(ctx, c, id, r, false)
}
func (s *Service) RestoreChapter(ctx context.Context, c Command, id string, r d.RevisionCommand) (idempotency.Result, error) {
	bind(&c, "POST", "/v1/content/chapters/"+id+"/restore", r)
	return s.chapterState(ctx, c, id, r, true)
}
func (s *Service) chapterState(ctx context.Context, c Command, id string, r d.RevisionCommand, restore bool) (idempotency.Result, error) {
	return s.execute(ctx, c, "chapter", id, true, true, 200, func(tx context.Context, o Owner) (any, error) {
		n, e := s.active(tx, o)
		if e != nil {
			return nil, e
		}
		ch, e := s.o.Repo.Chapter(tx, o, id, true)
		if e != nil {
			return nil, e
		}
		if e = d.CAS(ch.Revision, r.ExpectedRevision); e != nil {
			return nil, e
		}
		org, e := s.o.Repo.Organization(tx, o)
		if e != nil {
			return nil, e
		}
		action := "chapter.deleted"
		if restore {
			if !d.Restorable(ch.DeletedAt, s.now()) {
				return nil, d.State()
			}
			v, e := s.o.Repo.Volume(tx, o, ch.VolumeID, true)
			if e != nil {
				return nil, e
			}
			if v.DeletedAt != nil {
				return nil, d.State()
			}
			if len(org.Chapters) >= 2000 {
				return nil, d.Invalid()
			}
			ch.DeletedAt = nil
			ch.Position = 0
			for _, other := range org.Chapters {
				if other.VolumeID == ch.VolumeID {
					ch.Position++
				}
			}
			action = "chapter.restored"
		} else {
			if ch.DeletedAt != nil {
				return nil, d.State()
			}
			now := s.now()
			ch.DeletedAt = &now
		}
		if e = s.saveChapter(tx, o, &ch); e != nil {
			return nil, e
		}
		confirmed, e := s.confirmedBody(tx, o, ch)
		if e != nil {
			return nil, e
		}
		if ch.ConfirmedVersionID != nil {
			before, after, source := confirmed, "", "delete"
			if restore {
				before, after, source = "", confirmed, "restore"
			}
			if e = s.recordConfirmedWriting(tx, o, &n, ch, *ch.ConfirmedVersionID, source, before, after); e != nil {
				return nil, e
			}
		}
		if !restore {
			for _, other := range org.Chapters {
				if other.VolumeID == ch.VolumeID && other.Position > ch.Position {
					sibling, e := s.o.Repo.Chapter(tx, o, other.ID, true)
					if e != nil {
						return nil, e
					}
					sibling.Position--
					if e = s.saveChapter(tx, o, &sibling); e != nil {
						return nil, e
					}
				}
			}
		}
		if e = s.bumpNovel(tx, o, &n); e != nil {
			return nil, e
		}
		return ch.Chapter, s.event(tx, c.Principal, action, "chapter", id, o.NovelID, ch.Revision)
	})
}
