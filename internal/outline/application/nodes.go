package application

import (
	d "InkLore/internal/outline/domain"
	"InkLore/internal/platform/scope"
	"context"
	"strings"

	"github.com/google/uuid"
)

func (s *Service) binding(ctx context.Context, e *editState, n *d.Node) error {
	if !scope.ValidID(n.TargetID) {
		return invalid("target_id")
	}
	novel := e.NovelID
	if n.Level == "master" {
		if n.ParentID != nil || n.TargetID != novel {
			return invalid("parent_id")
		}
		return nil
	}
	if n.ParentID == nil || !scope.ValidID(*n.ParentID) {
		return invalid("parent_id")
	}
	parent := e.node(*n.ParentID)
	if parent == nil || parent.DeletedAt != nil {
		return invalid("parent_id")
	}
	if parent.Status == "archived" {
		return stateError()
	}
	want := "master"
	kind := "volume"
	if n.Level == "chapter" {
		want = "volume"
		kind = "chapter"
	} else if n.Level != "volume" {
		return invalid("level")
	}
	if parent.Level != want {
		return invalid("parent_id")
	}
	if err := s.assert(ctx, e.p, novel, kind, n.TargetID, scope.Write); err != nil {
		return err
	}
	if n.Level == "chapter" {
		v, err := s.ChapterContext.ChapterVolume(ctx, e.p, novel, n.TargetID)
		if err != nil {
			return err
		}
		if v != parent.TargetID {
			return invalid("parent_id")
		}
	}
	for _, other := range e.Nodes {
		if other.ID != n.ID && other.DeletedAt == nil && other.Level == n.Level && other.TargetID == n.TargetID {
			return stateError()
		}
	}
	return nil
}
func (e *editState) nodeCapacity(level string) error {
	maxNodes := 100
	if level == "chapter" {
		maxNodes = 2000
	}
	count := 0
	for _, n := range e.Nodes {
		if n.DeletedAt == nil && n.Level == level {
			count++
		}
	}
	if count >= maxNodes {
		return invalid("level")
	}
	return nil
}
func nodeText(title, summary string) error {
	if !validText(title, 1, 120) {
		return invalid("title")
	}
	if !validText(summary, 0, 20000) {
		return invalid("summary")
	}
	return nil
}
func (s *Service) CreateNode(ctx context.Context, p scope.Principal, novel string, in d.CreateNode) (d.Node, error) {
	var out *d.Node
	err := s.mutate(ctx, p, "novel", novel, "createNode", func(tx context.Context, e *editState) error {
		in.Summary = strings.ReplaceAll(in.Summary, "\r\n", "\n")
		if err := nodeText(in.Title, in.Summary); err != nil {
			return err
		}
		if !oneOf(in.Level, "master", "volume", "chapter") {
			return invalid("level")
		}
		if in.Level == "master" && e.master() != nil {
			return stateError()
		}
		if in.Level != "master" {
			if e.master() == nil || e.master().Status == "archived" {
				return stateError()
			}
			if err := e.nodeCapacity(in.Level); err != nil {
				return err
			}
		}
		n := d.Node{ID: uuid.NewString(), TenantID: p.TenantID, NovelID: novel, ParentID: in.ParentID, Level: in.Level, TargetID: in.TargetID, Title: in.Title, Summary: in.Summary, Status: "draft", Revision: 1, CreatedAt: e.now, UpdatedAt: e.now}
		if err := s.binding(tx, e, &n); err != nil {
			return err
		}
		if n.ParentID != nil {
			n.Position = e.count(*n.ParentID)
			if err := e.touch(e.node(*n.ParentID)); err != nil {
				return err
			}
		}
		e.Nodes = append(e.Nodes, n)
		e.nodes[n.ID] = 0
		out = &e.Nodes[len(e.Nodes)-1]
		return nil
	})
	if err != nil {
		return d.Node{}, err
	}
	return *out, nil
}
func (s *Service) UpdateNode(ctx context.Context, p scope.Principal, id string, in d.UpdateNode) (d.Node, error) {
	var out *d.Node
	err := s.mutate(ctx, p, "outline_node", id, "updateNode", func(tx context.Context, e *editState) error {
		n := e.node(id)
		if n == nil || n.DeletedAt != nil {
			return missing()
		}
		if err := cas(n.Revision, in.ExpectedRevision); err != nil {
			return err
		}
		if !oneOf(in.Status, "draft", "ready", "archived") {
			return invalid("status")
		}
		if n.Status == "archived" && in.Status != "draft" {
			return stateError()
		}
		in.Summary = strings.ReplaceAll(in.Summary, "\r\n", "\n")
		if err := nodeText(in.Title, in.Summary); err != nil {
			return err
		}
		if in.Status == "ready" && strings.TrimSpace(in.Summary) == "" {
			return invalid("summary")
		}
		if err := e.touch(n); err != nil {
			return err
		}
		n.Title = in.Title
		n.Summary = in.Summary
		n.Status = in.Status
		out = n
		return nil
	})
	if err != nil {
		return d.Node{}, err
	}
	return *out, nil
}
func (e *editState) dependent(id string) bool {
	for _, n := range e.Nodes {
		if n.DeletedAt == nil && same(n.ParentID, id) {
			return true
		}
	}
	for _, b := range e.Beats {
		if b.DeletedAt == nil && b.NodeID == id {
			return true
		}
	}
	for _, v := range e.Scenes {
		if v.DeletedAt == nil && v.NodeID == id {
			return true
		}
	}
	for _, c := range e.Clues {
		if c.DeletedAt == nil && (same(c.PlantedNodeID, id) || same(c.PlannedRevealNodeID, id) || same(c.LastNodeID, id)) {
			return true
		}
	}
	return false
}
func (s *Service) DeleteNode(ctx context.Context, p scope.Principal, id string, in d.RevisionCommand) (d.Node, error) {
	var out *d.Node
	err := s.mutate(ctx, p, "outline_node", id, "deleteNode", func(tx context.Context, e *editState) error {
		n := e.node(id)
		if n == nil || n.DeletedAt != nil {
			return missing()
		}
		if err := cas(n.Revision, in.ExpectedRevision); err != nil {
			return err
		}
		if e.dependent(id) {
			return stateError()
		}
		if n.Level == "master" {
			for _, c := range e.Clues {
				if c.DeletedAt == nil {
					return stateError()
				}
			}
		}
		if err := e.touch(n); err != nil {
			return err
		}
		n.DeletedAt = &e.now
		if n.ParentID != nil {
			if err := e.touch(e.node(*n.ParentID)); err != nil {
				return err
			}
			if err := e.compact(*n.ParentID); err != nil {
				return err
			}
		}
		out = n
		return nil
	})
	if err != nil {
		return d.Node{}, err
	}
	return *out, nil
}
func (s *Service) RestoreNode(ctx context.Context, p scope.Principal, id string, in d.RevisionCommand) (d.Node, error) {
	var out *d.Node
	err := s.mutate(ctx, p, "outline_node", id, "restoreNode", func(tx context.Context, e *editState) error {
		n := e.node(id)
		if n == nil || !retained(n.DeletedAt, e.now) {
			return missing()
		}
		if err := cas(n.Revision, in.ExpectedRevision); err != nil {
			return err
		}
		if n.Level == "master" {
			if e.master() != nil {
				return stateError()
			}
		} else {
			if e.master() == nil || e.master().Status == "archived" {
				return stateError()
			}
			if err := e.nodeCapacity(n.Level); err != nil {
				return err
			}
		}
		if err := s.binding(tx, e, n); err != nil {
			return err
		}
		if err := e.touch(n); err != nil {
			return err
		}
		n.DeletedAt = nil
		if n.ParentID != nil {
			n.Position = e.count(*n.ParentID) - 1
			if err := e.touch(e.node(*n.ParentID)); err != nil {
				return err
			}
		}
		out = n
		return nil
	})
	if err != nil {
		return d.Node{}, err
	}
	return *out, nil
}
func (s *Service) RebindNode(ctx context.Context, p scope.Principal, id string, in d.RebindNode) (d.Node, error) {
	var out *d.Node
	err := s.mutate(ctx, p, "outline_node", id, "rebindNode", func(tx context.Context, e *editState) error {
		n, err := e.chapter(id)
		if err != nil {
			return err
		}
		if err = cas(n.Revision, in.ExpectedRevision); err != nil {
			return err
		}
		if !scope.ValidID(in.ParentID) {
			return invalid("parent_id")
		}
		old := *n.ParentID
		n.ParentID = &in.ParentID
		if err = s.binding(tx, e, n); err != nil {
			return err
		}
		if old != in.ParentID {
			n.Position = e.count(in.ParentID) - 1
			if err = e.compact(old); err != nil {
				return err
			}
		}
		if err = e.touch(e.node(old)); err != nil {
			return err
		}
		if err = e.touch(e.node(in.ParentID)); err != nil {
			return err
		}
		if err = e.touch(n); err != nil {
			return err
		}
		out = n
		return nil
	})
	if err != nil {
		return d.Node{}, err
	}
	return *out, nil
}
