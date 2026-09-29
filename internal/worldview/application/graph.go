package application

import (
	"InkLore/internal/platform/scope"
	d "InkLore/internal/worldview/domain"
	"context"
	"encoding/json"
	"sort"
)

func orderedEntities(state d.State) []d.Entity {
	items := append([]d.Entity(nil), state.Entities...)
	sort.Slice(items, func(i, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	return items
}
func orderedRelations(state d.State) []d.Relation {
	items := append([]d.Relation(nil), state.Relations...)
	sort.Slice(items, func(i, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	return items
}
func (s *Service) Graph(ctx context.Context, p scope.Principal, n, center string, depth int) (d.Graph, error) {
	out := d.Graph{Nodes: []d.GraphNode{}, Edges: []d.Relation{}}
	if depth < 1 || depth > 2 || (center != "" && !scope.ValidID(center)) {
		return out, d.Malformed()
	}
	err := s.o.UoW.Within(ctx, func(tx context.Context) error {
		if _, err := s.Authorize(tx, p, "novel", n, false); err != nil {
			return err
		}
		var err error
		out, err = s.o.Store.Graph(tx, p.TenantID, n, center, depth)
		return err
	})
	return out, err
}

type ContextRequest struct {
	EntityIDs                 []string
	RecordBudget, TokenBudget int
}
type ContextResult struct {
	Entities      []d.Entity
	Relations     []d.Relation
	Heads         []d.FactHead
	WorldRevision int64
	Truncated     bool
}

func (s *Service) ReadContext(ctx context.Context, p scope.Principal, n string, in ContextRequest) (ContextResult, error) {
	out := ContextResult{Entities: []d.Entity{}, Relations: []d.Relation{}, Heads: []d.FactHead{}}
	if in.RecordBudget < 1 || in.RecordBudget > 500 || in.TokenBudget < 1 || in.TokenBudget > 60000 || len(in.EntityIDs) > 500 {
		return out, d.Invalid()
	}
	wanted := map[string]bool{}
	for _, id := range in.EntityIDs {
		if !scope.ValidID(id) {
			return out, d.Malformed()
		}
		wanted[id] = true
	}
	err := s.read(ctx, p, "novel", n, func(_ context.Context, state d.State) error {
		out.WorldRevision = state.World.Revision
		used, count := 0, 0
		add := func(v any) bool {
			b, e := json.Marshal(v)
			if e != nil {
				return false
			}
			cost := len(b)
			if count >= in.RecordBudget || used+cost > in.TokenBudget {
				out.Truncated = true
				return false
			}
			used += cost
			count++
			return true
		}
		included := map[string]bool{}
		for _, e := range orderedEntities(state) {
			if e.DeletedAt != nil || e.Status != "active" || (len(wanted) > 0 && !wanted[e.ID]) {
				continue
			}
			if add(e) {
				out.Entities = append(out.Entities, e)
				included[e.ID] = true
			}
		}
		for _, r := range orderedRelations(state) {
			if r.DeletedAt == nil && r.Status == "active" && included[r.SubjectID] && included[r.ObjectID] && add(r) {
				out.Relations = append(out.Relations, r)
			}
		}
		for _, h := range state.Heads {
			if h.Status == "active" && included[h.SubjectID] && add(h) {
				out.Heads = append(out.Heads, h)
			}
		}
		return nil
	})
	return out, err
}
