package application

import (
	"InkLore/internal/platform/scope"
	d "InkLore/internal/worldview/domain"
	"context"
	"time"

	"github.com/google/uuid"
)

func relationValid(state *d.State, v d.Relation) error {
	if v.SubjectID == v.ObjectID {
		return d.Invalid()
	}
	for _, id := range []string{v.SubjectID, v.ObjectID} {
		e := state.Entity(id)
		if e == nil || e.DeletedAt != nil {
			return d.Missing()
		}
	}
	if v.Status == "active" && v.DeletedAt == nil {
		for _, r := range state.Relations {
			if r.ID != v.ID && r.DeletedAt == nil && r.Status == "active" && r.SubjectID == v.SubjectID && r.ObjectID == v.ObjectID && r.Predicate == v.Predicate {
				return d.StateError()
			}
		}
	}
	return nil
}
func (s *Service) CreateRelation(ctx context.Context, p scope.Principal, n string, in d.CreateRelation) (d.Relation, error) {
	var out d.Relation
	if err := in.Validate(); err != nil {
		return out, err
	}
	err := s.change(ctx, p, "novel", n, "worldview.relation_created", func(state *d.State, c *d.Changes) error {
		out = d.Relation{ID: uuid.NewString(), TenantID: p.TenantID, NovelID: n, CreatedAt: c.World.UpdatedAt, UpdatedAt: c.World.UpdatedAt, Revision: 1, SubjectID: in.SubjectID, Predicate: in.Predicate, ObjectID: in.ObjectID, Description: in.Description, Weight: in.Weight, Status: "active"}
		if err := relationValid(state, out); err != nil {
			return err
		}
		c.Relations = append(c.Relations, out)
		return nil
	})
	return out, err
}
func (s *Service) UpdateRelation(ctx context.Context, p scope.Principal, id string, in d.UpdateRelation) (d.Relation, error) {
	var out d.Relation
	if err := in.Validate(); err != nil {
		return out, err
	}
	err := s.change(ctx, p, "relation", id, "worldview.relation_updated", func(state *d.State, c *d.Changes) error {
		for _, v := range state.Relations {
			if v.ID != id {
				continue
			}
			if v.DeletedAt != nil {
				return d.Missing()
			}
			if err := expected(in.ExpectedRevision, v.Revision); err != nil {
				return err
			}
			v.SubjectID = in.SubjectID
			v.Predicate = in.Predicate
			v.ObjectID = in.ObjectID
			v.Description = in.Description
			v.Weight = in.Weight
			v.Status = in.Status
			if err := relationValid(state, v); err != nil {
				return err
			}
			v.Revision++
			v.UpdatedAt = c.World.UpdatedAt
			out = v
			c.Relations = append(c.Relations, v)
			return nil
		}
		return d.Missing()
	})
	return out, err
}
func (s *Service) DeleteRelation(ctx context.Context, p scope.Principal, id string, rev int64) (d.Relation, error) {
	return s.relationLifecycle(ctx, p, id, rev, false)
}
func (s *Service) RestoreRelation(ctx context.Context, p scope.Principal, id string, rev int64) (d.Relation, error) {
	return s.relationLifecycle(ctx, p, id, rev, true)
}
func (s *Service) relationLifecycle(ctx context.Context, p scope.Principal, id string, rev int64, restore bool) (d.Relation, error) {
	var out d.Relation
	action := "worldview.relation_deleted"
	if restore {
		action = "worldview.relation_restored"
	}
	err := s.change(ctx, p, "relation", id, action, func(state *d.State, c *d.Changes) error {
		for _, v := range state.Relations {
			if v.ID != id {
				continue
			}
			if err := expected(rev, v.Revision); err != nil {
				return err
			}
			if restore {
				if v.DeletedAt == nil || !v.DeletedAt.Add(30*24*time.Hour).After(c.World.UpdatedAt) {
					return d.StateError()
				}
				v.DeletedAt = nil
				if err := relationValid(state, v); err != nil {
					return err
				}
			} else {
				if v.DeletedAt != nil {
					return d.StateError()
				}
				v.DeletedAt = &c.World.UpdatedAt
			}
			v.Revision++
			v.UpdatedAt = c.World.UpdatedAt
			out = v
			c.Relations = append(c.Relations, v)
			return nil
		}
		return d.Missing()
	})
	return out, err
}
