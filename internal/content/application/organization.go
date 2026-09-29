package application

import (
	d "InkLore/internal/content/domain"
	"InkLore/internal/platform/idempotency"
	"context"
	"sort"
)

func (s *Service) ReorderOrganization(ctx context.Context, c Command, novel string, r d.Reorder) (idempotency.Result, error) {
	bind(&c, "POST", "/v1/content/novels/"+novel+"/organization/reorder", r)
	return s.execute(ctx, c, "novel", novel, false, true, 200, func(tx context.Context, o Owner) (any, error) {
		n, e := s.active(tx, o)
		if e != nil {
			return nil, e
		}
		if e = d.CAS(n.Revision, r.ExpectedRevision); e != nil {
			return nil, e
		}
		org, e := s.o.Repo.Organization(tx, o)
		if e != nil {
			return nil, e
		}
		if e = d.ValidateOrder(org.Volumes, org.Chapters, r); e != nil {
			return nil, e
		}
		vpos := map[string]int64{}
		for i, id := range r.VolumeIds {
			vpos[id] = int64(i)
		}
		placements := map[string]d.Placement{}
		for _, p := range r.Chapters {
			placements[p.ChapterID] = p
		}
		sort.Slice(org.Volumes, func(i, j int) bool { return org.Volumes[i].ID < org.Volumes[j].ID })
		for i := range org.Volumes {
			v := &org.Volumes[i]
			if v.Position != vpos[v.ID] {
				v.Position = vpos[v.ID]
				if e = s.saveVolume(tx, o, v); e != nil {
					return nil, e
				}
			}
		}
		sort.Slice(org.Chapters, func(i, j int) bool { return org.Chapters[i].ID < org.Chapters[j].ID })
		for i := range org.Chapters {
			meta := &org.Chapters[i]
			p := placements[meta.ID]
			if meta.VolumeID == p.VolumeID && meta.Position == p.Position {
				continue
			}
			ch, e := s.o.Repo.Chapter(tx, o, meta.ID, true)
			if e != nil {
				return nil, e
			}
			ch.VolumeID, ch.Position = p.VolumeID, p.Position
			if e = s.saveChapter(tx, o, &ch); e != nil {
				return nil, e
			}
			*meta = ch.Chapter
		}
		if e = s.bumpNovel(tx, o, &n); e != nil {
			return nil, e
		}
		org.NovelRevision = n.Revision
		sort.Slice(org.Volumes, func(i, j int) bool { return org.Volumes[i].Position < org.Volumes[j].Position })
		orderOrganization(&org)
		return org, s.event(tx, c.Principal, "organization.reordered", "novel", novel, novel, n.Revision)
	})
}
