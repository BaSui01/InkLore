package application

import (
	"InkLore/internal/platform/scope"
	d "InkLore/internal/worldview/domain"
	"context"
	"time"
)

func (s *Service) CreateCharacter(ctx context.Context, p scope.Principal, n string, in d.CreateCharacter) (d.Character, error) {
	var out d.Character
	if err := in.Validate(); err != nil {
		return out, err
	}
	err := s.change(ctx, p, "novel", n, "worldview.character_created", func(state *d.State, c *d.Changes) error {
		v := newEntity(p, n, "character", c.World.UpdatedAt)
		v.Name = in.Name
		v.Description = in.Description
		v.Aliases = uniqueAliases(in.Aliases)
		v.RoleType = in.RoleType
		v.LifeStatus = "alive"
		c.Entities = append(c.Entities, v)
		out = v.Character
		return nil
	})
	return out, err
}
func (s *Service) CreateLore(ctx context.Context, p scope.Principal, n string, in d.CreateLore) (d.Lore, error) {
	var out d.Lore
	if err := in.Validate(); err != nil {
		return out, err
	}
	err := s.change(ctx, p, "novel", n, "worldview.lore_created", func(state *d.State, c *d.Changes) error {
		v := newEntity(p, n, "lore", c.World.UpdatedAt)
		v.Name = in.Title
		v.Description = in.Description
		v.Category = in.Category
		c.Entities = append(c.Entities, v)
		out = v.Lore()
		return nil
	})
	return out, err
}
func (s *Service) UpdateCharacter(ctx context.Context, p scope.Principal, id string, in d.UpdateCharacter) (d.Character, error) {
	var out d.Character
	if err := in.Validate(); err != nil {
		return out, err
	}
	err := s.change(ctx, p, "character", id, "worldview.character_updated", func(state *d.State, c *d.Changes) error {
		v := state.Entity(id)
		if v == nil || v.Kind != "character" || v.DeletedAt != nil {
			return d.Missing()
		}
		if err := expected(in.ExpectedRevision, v.Revision); err != nil {
			return err
		}
		v.Revision++
		v.UpdatedAt = c.World.UpdatedAt
		v.Name = in.Name
		v.Description = in.Description
		v.Aliases = uniqueAliases(in.Aliases)
		v.RoleType = in.RoleType
		v.Status = in.Status
		c.Entities = append(c.Entities, *v)
		out = v.Character
		return nil
	})
	return out, err
}
func (s *Service) UpdateLore(ctx context.Context, p scope.Principal, id string, in d.UpdateLore) (d.Lore, error) {
	var out d.Lore
	if err := in.Validate(); err != nil {
		return out, err
	}
	err := s.change(ctx, p, "lore", id, "worldview.lore_updated", func(state *d.State, c *d.Changes) error {
		v := state.Entity(id)
		if v == nil || v.Kind != "lore" || v.DeletedAt != nil {
			return d.Missing()
		}
		if err := expected(in.ExpectedRevision, v.Revision); err != nil {
			return err
		}
		if v.Category != in.Category {
			for _, h := range state.Heads {
				if h.Status == "active" && ((h.SubjectID == id && h.Predicate == "item_status") || (h.Predicate == "location" && h.Value != nil && h.Value.EntityID != nil && *h.Value.EntityID == id)) {
					return d.StateError()
				}
			}
		}
		v.Revision++
		v.UpdatedAt = c.World.UpdatedAt
		v.Name = in.Title
		v.Description = in.Description
		v.Category = in.Category
		v.Status = in.Status
		c.Entities = append(c.Entities, *v)
		out = v.Lore()
		return nil
	})
	return out, err
}
func (s *Service) DeleteEntity(ctx context.Context, p scope.Principal, kind, id string, revision int64) (d.Entity, error) {
	return s.entityLifecycle(ctx, p, kind, id, revision, false)
}
func (s *Service) RestoreEntity(ctx context.Context, p scope.Principal, kind, id string, revision int64) (d.Entity, error) {
	return s.entityLifecycle(ctx, p, kind, id, revision, true)
}
func (s *Service) entityLifecycle(ctx context.Context, p scope.Principal, kind, id string, revision int64, restore bool) (d.Entity, error) {
	var out d.Entity
	action := "worldview.entity_deleted"
	if restore {
		action = "worldview.entity_restored"
	}
	err := s.change(ctx, p, kind, id, action, func(state *d.State, c *d.Changes) error {
		v := state.Entity(id)
		if v == nil || v.Kind != kind {
			return d.Missing()
		}
		if err := expected(revision, v.Revision); err != nil {
			return err
		}
		if restore {
			if v.DeletedAt == nil || !v.DeletedAt.Add(30*24*time.Hour).After(c.World.UpdatedAt) {
				return d.StateError()
			}
			v.DeletedAt = nil
		} else {
			if v.DeletedAt != nil {
				return d.StateError()
			}
			for _, r := range state.Relations {
				if r.DeletedAt == nil && r.Status == "active" && (r.SubjectID == id || r.ObjectID == id) {
					return d.StateError()
				}
			}
			for _, h := range state.Heads {
				if h.Status == "active" && (h.SubjectID == id || (h.Value != nil && h.Value.EntityID != nil && *h.Value.EntityID == id)) {
					return d.StateError()
				}
			}
			v.DeletedAt = &c.World.UpdatedAt
		}
		v.Revision++
		v.UpdatedAt = c.World.UpdatedAt
		out = *v
		c.Entities = append(c.Entities, *v)
		return nil
	})
	return out, err
}
