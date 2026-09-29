package infrastructure

import (
	"InkLore/internal/platform/uow"
	d "InkLore/internal/worldview/domain"
	"context"
	"sort"
	"time"
)

type nodeRow struct {
	ID, Kind, Name, Status string
	UpdatedAt              time.Time
}

func (r *Repository) Graph(ctx context.Context, t, n, center string, depth int) (d.Graph, error) {
	out := d.Graph{Nodes: []d.GraphNode{}, Edges: []d.Relation{}}
	db, err := r.db(ctx, false)
	if err != nil {
		return out, err
	}
	suffix := " FOR SHARE"
	if uow.ReadSnapshot(ctx) {
		suffix = ""
	}
	q := db.Raw("SELECT revision FROM worldview_revisions WHERE tenant_id=? AND novel_id=?"+suffix, t, n).Scan(&out.WorldRevision)
	if q.Error != nil {
		return out, q.Error
	}
	if q.RowsAffected != 1 {
		return out, d.Missing()
	}
	included := map[string]nodeRow{}
	var ordered []nodeRow
	if center == "" {
		err = db.Raw("SELECT id,kind,name,status,updated_at FROM worldview_entities WHERE tenant_id=? AND novel_id=? AND deleted_at IS NULL ORDER BY updated_at DESC,id DESC LIMIT 501", t, n).Scan(&ordered).Error
		if err != nil {
			return out, err
		}
		if len(ordered) > 500 {
			out.Truncated = true
			ordered = ordered[:500]
		}
		for _, v := range ordered {
			included[v.ID] = v
		}
	} else {
		var start nodeRow
		q = db.Raw("SELECT id,kind,name,status,updated_at FROM worldview_entities WHERE tenant_id=? AND novel_id=? AND id=? AND deleted_at IS NULL", t, n, center).Scan(&start)
		if q.Error != nil {
			return out, q.Error
		}
		if q.RowsAffected != 1 {
			return out, d.Missing()
		}
		included[center] = start
		ordered = append(ordered, start)
		front := []string{center}
		for level := 0; level < depth && len(front) > 0; level++ {
			candidates := map[string]nodeRow{}
			// Independent outgoing/incoming predicates use the frozen endpoint indexes.
			for _, direction := range []struct{ from, to string }{{"subject_id", "object_id"}, {"object_id", "subject_id"}} {
				var rows []nodeRow
				query := "SELECT DISTINCT e.id,e.kind,e.name,e.status,e.updated_at FROM worldview_relations r JOIN worldview_entities e ON e.tenant_id=r.tenant_id AND e.novel_id=r.novel_id AND e.id=r." + direction.to + " WHERE r.tenant_id=? AND r.novel_id=? AND r." + direction.from + " IN ? AND r.deleted_at IS NULL AND r.status='active' AND e.deleted_at IS NULL ORDER BY e.updated_at DESC,e.id DESC LIMIT 501"
				if err = db.Raw(query, t, n, front).Scan(&rows).Error; err != nil {
					return out, err
				}
				if len(rows) > 500 {
					out.Truncated = true
				}
				for _, v := range rows {
					if _, ok := included[v.ID]; !ok {
						candidates[v.ID] = v
					}
				}
			}
			next := []nodeRow{}
			for _, v := range candidates {
				next = append(next, v)
			}
			sort.Slice(next, func(i, j int) bool {
				if next[i].UpdatedAt.Equal(next[j].UpdatedAt) {
					return next[i].ID > next[j].ID
				}
				return next[i].UpdatedAt.After(next[j].UpdatedAt)
			})
			front = []string{}
			for _, v := range next {
				if len(included) >= 500 {
					out.Truncated = true
					continue
				}
				included[v.ID] = v
				ordered = append(ordered, v)
				front = append(front, v.ID)
			}
		}
	}
	ids := []string{}
	for _, v := range ordered {
		ids = append(ids, v.ID)
		out.Nodes = append(out.Nodes, d.GraphNode{ID: v.ID, Kind: v.Kind, Name: v.Name, Status: v.Status})
	}
	if len(ids) == 0 {
		return out, nil
	}
	if err = db.Raw("SELECT * FROM worldview_relations WHERE tenant_id=? AND novel_id=? AND subject_id IN ? AND object_id IN ? AND deleted_at IS NULL AND status='active' ORDER BY updated_at DESC,id DESC LIMIT 1001", t, n, ids, ids).Scan(&out.Edges).Error; err != nil {
		return out, err
	}
	if len(out.Edges) > 1000 {
		out.Truncated = true
		out.Edges = out.Edges[:1000]
	}
	for i := range out.Edges {
		utcRelation(&out.Edges[i])
	}
	return out, nil
}
