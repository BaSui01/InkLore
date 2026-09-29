package application

import (
	d "InkLore/internal/content/domain"
	"InkLore/internal/platform/cursor"
	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/scope"
	"context"
	"encoding/json"
	"sort"
)

func (s *Service) GetNovel(ctx context.Context, p identity.Principal, id string) (out d.Novel, err error) {
	err = s.read(ctx, p, "novel", id, true, func(tx context.Context, o Owner) error {
		var e error
		out, e = s.o.Repo.Novel(tx, o, id, false)
		if e == nil && out.Status == "deleted" && !d.Restorable(out.DeletedAt, s.now()) {
			e = d.Missing()
		}
		return e
	})
	return
}
func (s *Service) GetVolume(ctx context.Context, p identity.Principal, id string) (out d.Volume, err error) {
	err = s.read(ctx, p, "volume", id, false, func(tx context.Context, o Owner) error {
		var e error
		out, e = s.o.Repo.Volume(tx, o, id, false)
		return e
	})
	return
}
func (s *Service) GetChapter(ctx context.Context, p identity.Principal, id string) (out d.Chapter, err error) {
	err = s.read(ctx, p, "chapter", id, false, func(tx context.Context, o Owner) error {
		ch, e := s.o.Repo.ChapterMetadata(tx, o, id)
		out = ch
		return e
	})
	return
}
func (s *Service) GetBody(ctx context.Context, p identity.Principal, id string) (out d.Body, err error) {
	err = s.read(ctx, p, "chapter", id, false, func(tx context.Context, o Owner) error {
		ch, e := s.o.Repo.Chapter(tx, o, id, false)
		out = d.Body{Chapter: ch.Chapter, Body: ch.Body}
		return e
	})
	return
}
func (s *Service) GetVersion(ctx context.Context, p identity.Principal, chapter, id string) (out d.Version, err error) {
	if !scope.ValidID(id) {
		return out, invalid()
	}
	err = s.read(ctx, p, "chapter", chapter, false, func(tx context.Context, o Owner) error {
		var e error
		out, e = s.o.Repo.Version(tx, o, chapter, id)
		return e
	})
	return
}
func (s *Service) DiffVersions(ctx context.Context, p identity.Principal, chapter, from, to string) (out d.Diff, err error) {
	if !scope.ValidID(from) || !scope.ValidID(to) {
		return out, invalid()
	}
	var a, b d.Version
	err = s.read(ctx, p, "chapter", chapter, false, func(tx context.Context, o Owner) error {
		var e error
		a, e = s.o.Repo.Version(tx, o, chapter, from)
		if e != nil {
			return e
		}
		b, e = s.o.Repo.Version(tx, o, chapter, to)
		return e
	})
	if err == nil {
		out = d.DiffBodies(a.Body, b.Body)
		out.FromVersionID = from
		out.ToVersionID = to
	}
	return
}
func (s *Service) GetOrganization(ctx context.Context, p identity.Principal, novel string) (out d.Organization, err error) {
	err = s.read(ctx, p, "novel", novel, false, func(tx context.Context, o Owner) error {
		n, e := s.o.Repo.Novel(tx, o, novel, false)
		if e != nil {
			return e
		}
		out, e = s.o.Repo.Organization(tx, o)
		out.NovelRevision = n.Revision
		orderOrganization(&out)
		return e
	})
	return
}
func orderOrganization(org *d.Organization) {
	positions := map[string]int64{}
	for _, v := range org.Volumes {
		positions[v.ID] = v.Position
	}
	sort.Slice(org.Chapters, func(i, j int) bool {
		a, b := org.Chapters[i], org.Chapters[j]
		if a.VolumeID != b.VolumeID {
			return positions[a.VolumeID] < positions[b.VolumeID]
		}
		return a.Position < b.Position
	})
}
func (s *Service) pagination(p identity.Principal, operation, resource string, q ListQuery, clock string) (Query, cursor.Binding, error) {
	if q.Limit == 0 {
		q.Limit = 20
	}
	if q.Limit < 1 || q.Limit > 100 {
		return Query{}, cursor.Binding{}, invalid()
	}
	filter, _ := json.Marshal(struct {
		Resource string `json:"resource"`
		Status   string `json:"status"`
		Deleted  bool   `json:"deleted"`
	}{resource, q.Status, q.Deleted})
	binding := cursor.Binding{TenantID: p.TenantID, ActorID: p.UserID, Audience: p.Audience, PermissionRevision: p.MembershipRevision, Operation: operation, Filters: filter, Sort: clock + " DESC,id DESC"}
	params := Query{Limit: q.Limit, Status: q.Status, Deleted: q.Deleted}
	if q.Cursor != "" {
		key := Key{}
		if e := s.o.Cursor.Decode(q.Cursor, binding, &key); e != nil {
			return params, binding, e
		}
		if key.Time.IsZero() || !scope.ValidID(key.ID) {
			return params, binding, pe.Contract(400, "platform.cursor_invalid", nil)
		}
		params.After = &key
	}
	return params, binding, nil
}
func finishPage[T any](s *Service, items []T, params Query, b cursor.Binding, key func(T) Key) ([]T, *string, bool, error) {
	if len(items) <= params.Limit {
		return items, nil, false, nil
	}
	items = items[:params.Limit]
	token, e := s.o.Cursor.Encode(b, key(items[len(items)-1]))
	if e != nil {
		return nil, nil, false, e
	}
	return items, &token, true, nil
}
func (s *Service) ListNovels(ctx context.Context, p identity.Principal, q ListQuery) (out d.NovelPage, err error) {
	if q.Status == "" {
		q.Status = "active"
	}
	if q.Status != "active" && q.Status != "archived" && q.Status != "deleted" {
		return out, invalid()
	}
	params, b, e := s.pagination(p, "content_listNovels", "", q, "updated_at")
	if e != nil {
		return out, e
	}
	err = s.read(ctx, p, "", "", false, func(tx context.Context, o Owner) error {
		items, e := s.o.Repo.ListNovels(tx, o, params)
		if e != nil {
			return e
		}
		out.Items, out.NextCursor, out.HasMore, e = finishPage(s, items, params, b, func(v d.Novel) Key { return Key{Time: v.UpdatedAt, ID: v.ID} })
		return e
	})
	return
}
func (s *Service) ListVolumes(ctx context.Context, p identity.Principal, novel string, q ListQuery) (out d.VolumePage, err error) {
	params, b, e := s.pagination(p, "content_listvolumes", novel, q, "updated_at")
	if e != nil {
		return out, e
	}
	err = s.read(ctx, p, "novel", novel, false, func(tx context.Context, o Owner) error {
		items, e := s.o.Repo.ListVolumes(tx, o, params)
		if e != nil {
			return e
		}
		out.Items, out.NextCursor, out.HasMore, e = finishPage(s, items, params, b, func(v d.Volume) Key { return Key{Time: v.UpdatedAt, ID: v.ID} })
		return e
	})
	return
}
func (s *Service) ListChapters(ctx context.Context, p identity.Principal, novel string, q ListQuery) (out d.ChapterPage, err error) {
	params, b, e := s.pagination(p, "content_listchapters", novel, q, "updated_at")
	if e != nil {
		return out, e
	}
	err = s.read(ctx, p, "novel", novel, false, func(tx context.Context, o Owner) error {
		items, e := s.o.Repo.ListChapters(tx, o, params)
		if e != nil {
			return e
		}
		out.Items, out.NextCursor, out.HasMore, e = finishPage(s, items, params, b, func(v d.Chapter) Key { return Key{Time: v.UpdatedAt, ID: v.ID} })
		return e
	})
	return
}
func (s *Service) ListVersions(ctx context.Context, p identity.Principal, chapter string, q ListQuery) (out d.VersionPage, err error) {
	params, b, e := s.pagination(p, "content_listVersions", chapter, q, "created_at")
	if e != nil {
		return out, e
	}
	err = s.read(ctx, p, "chapter", chapter, false, func(tx context.Context, o Owner) error {
		items, e := s.o.Repo.ListVersions(tx, o, chapter, params)
		if e != nil {
			return e
		}
		out.Items, out.NextCursor, out.HasMore, e = finishPage(s, items, params, b, func(v d.VersionSummary) Key { return Key{Time: v.CreatedAt, ID: v.ID} })
		return e
	})
	return
}

// ReadChapterVolume is an authorized read port for outline's scope binding.
func (s *Service) ReadChapterVolume(ctx context.Context, p scope.Principal, novel, chapter string) (string, error) {
	if !scope.ValidID(novel) {
		return "", invalid()
	}
	var volume string
	ip := identity.Principal{TenantID: p.TenantID, UserID: p.ActorID, SessionID: p.SessionID, Audience: p.Audience}
	e := s.read(ctx, ip, "chapter", chapter, false, func(tx context.Context, o Owner) error {
		if o.NovelID != novel {
			return d.Missing()
		}
		ch, e := s.o.Repo.ChapterMetadata(tx, o, chapter)
		if e != nil {
			return e
		}
		volume = ch.VolumeID
		return nil
	})
	return volume, e
}
