package application

import (
	d "InkLore/internal/content/domain"
	"InkLore/internal/platform/idempotency"
	"context"

	"github.com/google/uuid"
)

func (s *Service) CreateVolume(ctx context.Context, c Command, novel string, r d.CreateVolume) (idempotency.Result, error) {
	bind(&c, "POST", "/v1/content/novels/"+novel+"/volumes", r)
	return s.execute(ctx, c, "novel", novel, false, false, 201, func(tx context.Context, o Owner) (any, error) {
		if e := d.Metadata(r.Title, r.Summary, "", 0); e != nil {
			return nil, e
		}
		n, e := s.active(tx, o)
		if e != nil {
			return nil, e
		}
		org, e := s.o.Repo.Organization(tx, o)
		if e != nil {
			return nil, e
		}
		if len(org.Volumes) >= 100 {
			return nil, d.Invalid()
		}
		var pos int64
		for _, v := range org.Volumes {
			if v.Position >= pos {
				pos = v.Position + 1
			}
		}
		if pos > 99 {
			return nil, d.Invalid()
		}
		now := s.now()
		v := d.Volume{ID: uuid.NewString(), TenantID: o.TenantID, NovelID: novel, Title: r.Title, Summary: r.Summary, Position: pos, Revision: 1, CreatedAt: now, UpdatedAt: now}
		if e = s.o.Repo.InsertVolume(tx, o, v); e != nil {
			return nil, e
		}
		if e = s.projectVolume(tx, o, v, 0); e != nil {
			return nil, e
		}
		if e = s.bumpNovel(tx, o, &n); e != nil {
			return nil, e
		}
		return v, s.event(tx, c.Principal, "volume.created", "volume", v.ID, novel, v.Revision)
	})
}
func (s *Service) UpdateVolume(ctx context.Context, c Command, id string, r d.UpdateVolume) (idempotency.Result, error) {
	bind(&c, "PUT", "/v1/content/volumes/"+id, r)
	return s.execute(ctx, c, "volume", id, false, false, 200, func(tx context.Context, o Owner) (any, error) {
		if e := d.Metadata(r.Title, r.Summary, "", 0); e != nil {
			return nil, e
		}
		n, e := s.active(tx, o)
		if e != nil {
			return nil, e
		}
		v, e := s.o.Repo.Volume(tx, o, id, true)
		if e != nil {
			return nil, e
		}
		if v.DeletedAt != nil {
			return nil, d.Missing()
		}
		if e = d.CAS(v.Revision, r.ExpectedRevision); e != nil {
			return nil, e
		}
		v.Title, v.Summary = r.Title, r.Summary
		if e = s.saveVolume(tx, o, &v); e != nil {
			return nil, e
		}
		if e = s.bumpNovel(tx, o, &n); e != nil {
			return nil, e
		}
		return v, s.event(tx, c.Principal, "volume.updated", "volume", id, o.NovelID, v.Revision)
	})
}
func (s *Service) saveVolume(ctx context.Context, o Owner, v *d.Volume) error {
	old := v.Revision
	if e := d.CAS(old, old); e != nil {
		return e
	}
	v.Revision++
	v.UpdatedAt = s.now()
	if e := s.o.Repo.PutVolume(ctx, o, *v, old); e != nil {
		return e
	}
	return s.projectVolume(ctx, o, *v, old)
}
func (s *Service) DeleteVolume(ctx context.Context, c Command, id string, r d.RevisionCommand) (idempotency.Result, error) {
	bindDelete(&c, "/v1/content/volumes/"+id, r)
	return s.volumeState(ctx, c, id, r, false)
}
func (s *Service) RestoreVolume(ctx context.Context, c Command, id string, r d.RevisionCommand) (idempotency.Result, error) {
	bind(&c, "POST", "/v1/content/volumes/"+id+"/restore", r)
	return s.volumeState(ctx, c, id, r, true)
}
func (s *Service) volumeState(ctx context.Context, c Command, id string, r d.RevisionCommand, restore bool) (idempotency.Result, error) {
	return s.execute(ctx, c, "volume", id, true, true, 200, func(tx context.Context, o Owner) (any, error) {
		n, e := s.active(tx, o)
		if e != nil {
			return nil, e
		}
		v, e := s.o.Repo.Volume(tx, o, id, true)
		if e != nil {
			return nil, e
		}
		if e = d.CAS(v.Revision, r.ExpectedRevision); e != nil {
			return nil, e
		}
		org, e := s.o.Repo.Organization(tx, o)
		if e != nil {
			return nil, e
		}
		action := "volume.deleted"
		if restore {
			if !d.Restorable(v.DeletedAt, s.now()) {
				return nil, d.State()
			}
			if len(org.Volumes) >= 100 {
				return nil, d.Invalid()
			}
			v.DeletedAt = nil
			v.Position = int64(len(org.Volumes))
			action = "volume.restored"
		} else {
			if v.DeletedAt != nil {
				return nil, d.State()
			}
			for _, ch := range org.Chapters {
				if ch.VolumeID == id {
					return nil, d.State()
				}
			}
			now := s.now()
			v.DeletedAt = &now
		}
		if e = s.saveVolume(tx, o, &v); e != nil {
			return nil, e
		}
		if !restore {
			for _, sibling := range org.Volumes {
				if sibling.ID != id && sibling.Position > v.Position {
					sibling.Position--
					if e = s.saveVolume(tx, o, &sibling); e != nil {
						return nil, e
					}
				}
			}
		}
		if e = s.bumpNovel(tx, o, &n); e != nil {
			return nil, e
		}
		return v, s.event(tx, c.Principal, action, "volume", id, o.NovelID, v.Revision)
	})
}
