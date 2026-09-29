package application

import (
	d "InkLore/internal/outline/domain"
	"InkLore/internal/platform/scope"
	"context"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

type editState struct {
	*d.State
	p         scope.Principal
	now       time.Time
	nodes     map[string]int64
	beats     map[string]int64
	scenes    map[string]int64
	clues     map[string]int64
	newEvents []d.ClueEvent
}

func newEdit(st *d.State, p scope.Principal, now time.Time) *editState {
	return &editState{State: st, p: p, now: now, nodes: map[string]int64{}, beats: map[string]int64{}, scenes: map[string]int64{}, clues: map[string]int64{}}
}
func (e *editState) node(id string) *d.Node {
	for i := range e.Nodes {
		if e.Nodes[i].ID == id {
			return &e.Nodes[i]
		}
	}
	return nil
}
func (e *editState) master() *d.Node {
	for i := range e.Nodes {
		if e.Nodes[i].Level == "master" && e.Nodes[i].DeletedAt == nil {
			return &e.Nodes[i]
		}
	}
	return nil
}
func bump(id string, rev *int64, updated *time.Time, changes map[string]int64, now time.Time) error {
	if _, ok := changes[id]; ok {
		return nil
	}
	if *rev >= scope.MaxRevision {
		return conflict(*rev)
	}
	changes[id] = *rev
	*rev++
	*updated = now
	return nil
}
func (e *editState) touch(n *d.Node) error {
	return bump(n.ID, &n.Revision, &n.UpdatedAt, e.nodes, e.now)
}
func (e *editState) touchMaster() error {
	if n := e.master(); n != nil {
		return e.touch(n)
	}
	for i := range e.Nodes {
		n := &e.Nodes[i]
		if n.Level == "master" {
			if _, ok := e.nodes[n.ID]; ok {
				return nil
			}
		}
	}
	return stateError()
}
func (e *editState) chapter(id string) (*d.Node, error) {
	n := e.node(id)
	if n == nil || n.DeletedAt != nil {
		return nil, missing()
	}
	if n.Level != "chapter" {
		return nil, invalid("node_id")
	}
	if n.Status == "archived" {
		return nil, stateError()
	}
	for p := n.ParentID; p != nil; {
		a := e.node(*p)
		if a == nil || a.DeletedAt != nil || a.Status == "archived" {
			return nil, stateError()
		}
		p = a.ParentID
	}
	return n, nil
}
func hashNode(n *d.Node) error { n.ContentHash = d.NodeHash(*n); return nil }
func (e *editState) changes() (*d.Changes, error) {
	c := &d.Changes{Events: e.newEvents}
	for i := range e.Nodes {
		n := &e.Nodes[i]
		if prev, ok := e.nodes[n.ID]; ok {
			if err := hashNode(n); err != nil {
				return nil, err
			}
			c.Nodes = append(c.Nodes, d.NodeChange{Value: *n, Previous: prev})
		}
	}
	for _, v := range e.Beats {
		if prev, ok := e.beats[v.ID]; ok {
			c.Beats = append(c.Beats, d.BeatChange{Value: v, Previous: prev})
		}
	}
	for _, v := range e.Scenes {
		if prev, ok := e.scenes[v.ID]; ok {
			c.Scenes = append(c.Scenes, d.SceneChange{Value: v, Previous: prev})
		}
	}
	for _, v := range e.Clues {
		if prev, ok := e.clues[v.ID]; ok {
			c.Clues = append(c.Clues, d.ClueChange{Value: v, Previous: prev})
		}
	}
	return c, nil
}
func (s *Service) project(ctx context.Context, p scope.Principal, novel string, c *d.Changes) error {
	for _, v := range c.Nodes {
		n := v.Value
		if err := s.Scope.UpsertScope(ctx, scope.Projection{TenantID: p.TenantID, ResourceID: n.ID, ResourceKind: "outline_node", OwnerID: p.ActorID, NovelID: &novel, CurrentRevision: n.Revision, ContentHash: &n.ContentHash, DeletedAt: n.DeletedAt}, v.Previous); err != nil {
			return err
		}
		if err := s.Scope.AppendRevision(ctx, scope.Revision{TenantID: p.TenantID, ResourceID: n.ID, Revision: n.Revision, ContentHash: n.ContentHash, ContentLength: int64(utf8.RuneCountInString(n.Summary))}); err != nil {
			return err
		}
	}
	for _, v := range c.Beats {
		n := v.Value
		if err := s.Scope.UpsertScope(ctx, scope.Projection{TenantID: p.TenantID, ResourceID: n.ID, ResourceKind: "outline_beat", OwnerID: p.ActorID, NovelID: &novel, CurrentRevision: n.Revision, DeletedAt: n.DeletedAt}, v.Previous); err != nil {
			return err
		}
	}
	for _, v := range c.Scenes {
		n := v.Value
		if err := s.Scope.UpsertScope(ctx, scope.Projection{TenantID: p.TenantID, ResourceID: n.ID, ResourceKind: "outline_scene", OwnerID: p.ActorID, NovelID: &novel, CurrentRevision: n.Revision, DeletedAt: n.DeletedAt}, v.Previous); err != nil {
			return err
		}
	}
	for _, v := range c.Clues {
		n := v.Value
		if err := s.Scope.UpsertScope(ctx, scope.Projection{TenantID: p.TenantID, ResourceID: n.ID, ResourceKind: "outline_clue", OwnerID: p.ActorID, NovelID: &novel, CurrentRevision: n.Revision, DeletedAt: n.DeletedAt}, v.Previous); err != nil {
			return err
		}
	}
	return nil
}
func same(a *string, b string) bool { return a != nil && *a == b }
func (e *editState) compact(parent string) error {
	ids := []string{}
	for _, n := range e.Nodes {
		if n.DeletedAt == nil && same(n.ParentID, parent) {
			ids = append(ids, n.ID)
		}
	}
	sort.Slice(ids, func(i, j int) bool {
		a, b := e.node(ids[i]), e.node(ids[j])
		if a.Position == b.Position {
			return a.ID < b.ID
		}
		return a.Position < b.Position
	})
	for i, id := range ids {
		n := e.node(id)
		if n == nil {
			return missing()
		}
		if n.Position != int64(i) {
			if err := e.touch(n); err != nil {
				return err
			}
			n.Position = int64(i)
		}
	}
	return nil
}
func (e *editState) count(parent string) int64 {
	var n int64
	for _, v := range e.Nodes {
		if v.DeletedAt == nil && same(v.ParentID, parent) {
			n++
		}
	}
	return n
}
func validText(s string, min, max int) bool {
	n := utf8.RuneCountInString(s)
	return utf8.ValidString(s) && !strings.ContainsRune(s, 0) && n >= min && n <= max
}
func oneOf(s string, values ...string) bool {
	for _, v := range values {
		if s == v {
			return true
		}
	}
	return false
}
