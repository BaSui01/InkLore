package application

import (
	d "InkLore/internal/outline/domain"
	"InkLore/internal/platform/scope"
	"context"
	"sort"
	"time"
)

type ListQuery struct {
	ParentID, Level, NodeID, Status, PlannedRevealNodeID string
	Deleted                                              bool
	Limit                                                int
	BeforeAt                                             time.Time
	BeforeID                                             string
}
type ListResult[T any] struct {
	Items   []T
	HasMore bool
}

func page[T any](items []T, q ListQuery, key func(T) (time.Time, string)) ListResult[T] {
	sort.Slice(items, func(i, j int) bool {
		a, ai := key(items[i])
		b, bi := key(items[j])
		if a.Equal(b) {
			return ai > bi
		}
		return a.After(b)
	})
	out := make([]T, 0)
	for _, v := range items {
		at, id := key(v)
		if q.BeforeID != "" && (!at.Before(q.BeforeAt) && !(at.Equal(q.BeforeAt) && id < q.BeforeID)) {
			continue
		}
		out = append(out, v)
	}
	limit := q.Limit
	if limit < 1 || limit > 100 {
		limit = 20
	}
	more := len(out) > limit
	if more {
		out = out[:limit]
	}
	return ListResult[T]{Items: out, HasMore: more}
}
func visible(t *time.Time, deleted bool) bool {
	if !deleted {
		return t == nil
	}
	return retained(t, time.Now().UTC())
}
func (s *Service) ListNodes(ctx context.Context, p scope.Principal, novel string, q ListQuery) (ListResult[d.Node], error) {
	st, err := s.read(ctx, p, "novel", novel)
	if err != nil {
		return ListResult[d.Node]{}, err
	}
	if q.ParentID != "" {
		e := newEdit(st, p, time.Now().UTC())
		n := e.node(q.ParentID)
		if n == nil || n.DeletedAt != nil {
			return ListResult[d.Node]{}, missing()
		}
	}
	items := []d.Node{}
	for _, v := range st.Nodes {
		if visible(v.DeletedAt, q.Deleted) && (q.ParentID == "" || same(v.ParentID, q.ParentID)) && (q.Level == "" || v.Level == q.Level) {
			items = append(items, v)
		}
	}
	return page(items, q, func(v d.Node) (time.Time, string) { return v.UpdatedAt, v.ID }), nil
}
func (s *Service) ListBeats(ctx context.Context, p scope.Principal, novel string, q ListQuery) (ListResult[d.Beat], error) {
	st, err := s.read(ctx, p, "novel", novel)
	if err != nil {
		return ListResult[d.Beat]{}, err
	}
	e := newEdit(st, p, time.Now().UTC())
	n := e.node(q.NodeID)
	if n == nil || n.DeletedAt != nil || n.Level != "chapter" {
		return ListResult[d.Beat]{}, missing()
	}
	items := []d.Beat{}
	for _, v := range st.Beats {
		if visible(v.DeletedAt, q.Deleted) && v.NodeID == q.NodeID {
			items = append(items, v)
		}
	}
	return page(items, q, func(v d.Beat) (time.Time, string) { return v.UpdatedAt, v.ID }), nil
}
func (s *Service) ListScenes(ctx context.Context, p scope.Principal, novel string, q ListQuery) (ListResult[d.Scene], error) {
	st, err := s.read(ctx, p, "novel", novel)
	if err != nil {
		return ListResult[d.Scene]{}, err
	}
	e := newEdit(st, p, time.Now().UTC())
	n := e.node(q.NodeID)
	if n == nil || n.DeletedAt != nil || n.Level != "chapter" {
		return ListResult[d.Scene]{}, missing()
	}
	items := []d.Scene{}
	for _, v := range st.Scenes {
		if visible(v.DeletedAt, q.Deleted) && v.NodeID == q.NodeID {
			items = append(items, v)
		}
	}
	return page(items, q, func(v d.Scene) (time.Time, string) { return v.UpdatedAt, v.ID }), nil
}
func (s *Service) ListClues(ctx context.Context, p scope.Principal, novel string, q ListQuery) (ListResult[d.Clue], error) {
	st, err := s.read(ctx, p, "novel", novel)
	if err != nil {
		return ListResult[d.Clue]{}, err
	}
	items := []d.Clue{}
	for _, v := range st.Clues {
		if visible(v.DeletedAt, q.Deleted) && (q.Status == "" || v.Status == q.Status) && (q.PlannedRevealNodeID == "" || same(v.PlannedRevealNodeID, q.PlannedRevealNodeID)) {
			items = append(items, v)
		}
	}
	return page(items, q, func(v d.Clue) (time.Time, string) { return v.UpdatedAt, v.ID }), nil
}
func (s *Service) ListClueEvents(ctx context.Context, p scope.Principal, id string, q ListQuery) (ListResult[d.ClueEvent], error) {
	st, err := s.read(ctx, p, "outline_clue", id)
	if err != nil {
		return ListResult[d.ClueEvent]{}, err
	}
	e := newEdit(st, p, time.Now().UTC())
	v := e.clue(id)
	if v == nil || v.DeletedAt != nil {
		return ListResult[d.ClueEvent]{}, missing()
	}
	items := []d.ClueEvent{}
	for _, v := range st.Events {
		if v.ClueID == id {
			items = append(items, v)
		}
	}
	return page(items, q, func(v d.ClueEvent) (time.Time, string) { return v.CreatedAt, v.ID }), nil
}
