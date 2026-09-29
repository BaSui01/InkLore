package application

import (
	d "InkLore/internal/outline/domain"
	"InkLore/internal/platform/scope"
	"context"
)

func (s *Service) Reorder(ctx context.Context, p scope.Principal, id string, in d.Reorder) (d.OrderResult, error) {
	var parent *d.Node
	err := s.mutate(ctx, p, "outline_node", id, "reorder", func(tx context.Context, e *editState) error {
		n := e.node(id)
		if n == nil || n.DeletedAt != nil {
			return missing()
		}
		if n.Status == "archived" {
			return stateError()
		}
		if err := cas(n.Revision, in.ExpectedRevision); err != nil {
			return err
		}
		if len(in.OrderedIDs) > 2000 {
			return invalid("ordered_ids")
		}
		expected := map[string]bool{}
		switch in.Resource {
		case "nodes":
			if n.Level == "chapter" {
				return invalid("resource")
			}
			for _, v := range e.Nodes {
				if v.DeletedAt == nil && same(v.ParentID, id) {
					expected[v.ID] = true
				}
			}
		case "beats":
			if _, err := e.chapter(id); err != nil {
				return err
			}
			for _, v := range e.Beats {
				if v.DeletedAt == nil && v.NodeID == id {
					expected[v.ID] = true
				}
			}
		case "scenes":
			if _, err := e.chapter(id); err != nil {
				return err
			}
			for _, v := range e.Scenes {
				if v.DeletedAt == nil && v.NodeID == id {
					expected[v.ID] = true
				}
			}
		default:
			return invalid("resource")
		}
		if len(expected) != len(in.OrderedIDs) {
			return invalid("ordered_ids")
		}
		for i, key := range in.OrderedIDs {
			if !scope.ValidID(key) || !expected[key] {
				return invalid("ordered_ids")
			}
			delete(expected, key)
			switch in.Resource {
			case "nodes":
				v := e.node(key)
				if err := e.touch(v); err != nil {
					return err
				}
				v.Position = int64(i)
			case "beats":
				v := e.beat(key)
				if err := bump(v.ID, &v.Revision, &v.UpdatedAt, e.beats, e.now); err != nil {
					return err
				}
				v.Position = int64(i)
			case "scenes":
				v := e.scene(key)
				if err := bump(v.ID, &v.Revision, &v.UpdatedAt, e.scenes, e.now); err != nil {
					return err
				}
				v.Position = int64(i)
			}
		}
		if err := e.touch(n); err != nil {
			return err
		}
		parent = n
		return nil
	})
	if err != nil {
		return d.OrderResult{}, err
	}
	return d.OrderResult{Parent: *parent, OrderedIDs: in.OrderedIDs}, nil
}
