package application

import (
	"InkLore/internal/platform/scope"
	"InkLore/internal/platform/uow"
	d "InkLore/internal/worldview/domain"
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

func (s *Service) maintenance(ctx context.Context, t, n, action string) error {
	if s.o.AuthorizeMaintenance == nil {
		return errors.New("worldview: maintenance capability required")
	}
	if !scope.ValidID(t) || !scope.ValidID(n) {
		return d.Malformed()
	}
	return s.o.AuthorizeMaintenance(ctx, t, n, action)
}
func (s *Service) ReplayHeads(ctx context.Context, p scope.Principal, n string) error {
	var state d.State
	var events []d.FactEvent
	err := s.o.UoW.WithinReadSnapshot(ctx, func(tx context.Context) error {
		if err := s.maintenance(tx, p.TenantID, n, "replay_heads"); err != nil {
			return err
		}
		var err error
		state, err = s.o.Store.Load(tx, p.TenantID, n, false)
		if err != nil {
			return err
		}
		events, err = s.o.Store.Events(tx, p.TenantID, n)
		return err
	})
	if err != nil {
		return err
	}
	heads := map[string]d.FactHead{}
	var seq int64
	var lastWorld int64
	for _, e := range events {
		if e.Seq != seq+1 || e.WorldRevision < lastWorld || e.WorldRevision > state.World.Revision || e.PayloadErased {
			return d.Corrupt()
		}
		seq = e.Seq
		lastWorld = e.WorldRevision
		key := e.SubjectID + "/" + e.Predicate + "/" + e.FactKey
		h, exists := heads[key]
		if e.Action == "assert" {
			if e.TargetEventID != nil || e.Resolution != "none" || e.Value == nil || (exists && h.Status == "active") {
				return d.Corrupt()
			}
		} else if d.One(e.Action, "supersede", "retract") {
			if !exists || h.Status != "active" || e.TargetEventID == nil || *e.TargetEventID != h.EventID || !d.One(e.Resolution, "correction", "retcon") || e.Reason == nil || (e.Action == "retract") != (e.Value == nil) {
				return d.Corrupt()
			}
		} else {
			return d.Corrupt()
		}
		proposal := d.FactProposal{SubjectID: e.SubjectID, Predicate: e.Predicate, FactKey: e.FactKey, Value: e.Value, Action: e.Action, TargetEventID: e.TargetEventID, Resolution: e.Resolution, Reason: e.Reason}
		if proposal.Validate() != nil {
			return d.Corrupt()
		}
		if !exists {
			h = d.FactHead{TenantID: p.TenantID, NovelID: n, SubjectID: e.SubjectID, Predicate: e.Predicate, FactKey: e.FactKey, CreatedAt: e.CreatedAt}
		}
		h.EventID = e.ID
		h.Value = e.Value
		h.Status = "active"
		if e.Action == "retract" {
			h.Status = "retracted"
		}
		heads[key] = h
	}
	if seq != state.LastSeq {
		return d.Corrupt()
	}
	rebuilt := []d.FactHead{}
	for _, h := range heads {
		rebuilt = append(rebuilt, h)
	}
	if err := d.ValidateProjection(state, rebuilt); err != nil {
		return err
	}
	return s.o.UoW.Within(ctx, func(tx context.Context) error {
		if err := s.maintenance(tx, p.TenantID, n, "replay_heads"); err != nil {
			return err
		}
		current, err := s.o.Store.Load(tx, p.TenantID, n, true)
		if err != nil {
			return err
		}
		if current.World.Revision != state.World.Revision || current.World.Revision >= d.MaxRevision {
			return d.Version(current.World.Revision)
		}
		c := d.Changes{World: current.World, OldWorldRevision: current.World.Revision, LastSeq: seq, ReplaceHeads: true}
		c.World.Revision++
		c.World.UpdatedAt = time.Now().UTC()
		for _, h := range heads {
			old := current.Head(h.SubjectID, h.Predicate, h.FactKey)
			if old != nil {
				if old.Revision >= d.MaxRevision {
					return d.Version(old.Revision)
				}
				h.ID = old.ID
				h.CreatedAt = old.CreatedAt
				h.Revision = old.Revision + 1
			} else {
				h.ID = uuid.NewString()
				h.Revision = 1
			}
			h.UpdatedAt = c.World.UpdatedAt
			c.Heads = append(c.Heads, h)
		}
		for _, e := range current.Entities {
			if e.Kind != "character" {
				continue
			}
			if e.Revision >= d.MaxRevision {
				return d.Version(e.Revision)
			}
			e.LifeStatus = "alive"
			e.RealmRank = 0
			for _, h := range c.Heads {
				if h.SubjectID != e.ID || h.Status != "active" || h.Value == nil {
					continue
				}
				if h.Predicate == "life_status" && h.Value.TextValue != nil {
					e.LifeStatus = *h.Value.TextValue
				}
				if h.Predicate == "realm_rank" && h.Value.NumberValue != nil {
					e.RealmRank = *h.Value.NumberValue
				}
			}
			e.Revision++
			e.UpdatedAt = c.World.UpdatedAt
			c.Entities = append(c.Entities, e)
		}
		return s.save(tx, p, c, n, "worldview.heads_rebuilt")
	})
}

// PurgeDeletedEntities removes expired static text without breaking foreign keys
// from retired relations. The opaque identity and ledger proofs remain intact.
func (s *Service) PurgeDeletedEntities(ctx context.Context, p scope.Principal, novel string) error {
	return s.o.UoW.Within(ctx, func(tx context.Context) error {
		if err := s.maintenance(tx, p.TenantID, novel, "purge_entities"); err != nil {
			return err
		}
		state, err := s.o.Store.Load(tx, p.TenantID, novel, true)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		c := d.Changes{World: state.World, OldWorldRevision: state.World.Revision, LastSeq: state.LastSeq}
		c.World.Revision++
		c.World.UpdatedAt = now
		for _, e := range state.Entities {
			if e.DeletedAt == nil || e.DeletedAt.Add(30*24*time.Hour).After(now) {
				continue
			}
			if e.Name == "erased" && e.Description == "" && len(e.Aliases) == 0 {
				continue
			}
			if e.Revision >= d.MaxRevision {
				return d.Version(e.Revision)
			}
			e.Name = "erased"
			e.Description = ""
			e.Aliases = []string{}
			e.Revision++
			e.UpdatedAt = now
			c.Entities = append(c.Entities, e)
		}
		if len(c.Entities) == 0 {
			return nil
		}
		return s.save(tx, p, c, novel, "worldview.entities_purged")
	})
}
func (s *Service) PurgeNovel(ctx context.Context, tenant, novel string) error {
	return s.o.UoW.Within(ctx, func(tx context.Context) error {
		if err := s.maintenance(tx, tenant, novel, "purge_novel"); err != nil {
			return err
		}
		state, err := s.o.Store.Load(tx, tenant, novel, true)
		if err != nil {
			return err
		}
		if err = s.o.Store.Purge(tx, tenant, novel); err != nil {
			return err
		}
		if s.o.OnEvent == nil {
			return errors.New("worldview: transactional event sink required")
		}
		return s.o.OnEvent(tx, d.ChangeEvent{TenantID: tenant, NovelID: novel, ResourceID: novel, Action: "worldview.novel_purged", Revision: state.World.Revision})
	})
}
func (s *Service) CaptureSnapshot(ctx context.Context, p scope.Principal, n string) (d.Snapshot, error) {
	out := d.Snapshot{Entities: []d.Entity{}, Relations: []d.Relation{}}
	err := s.o.UoW.WithinReadSnapshot(ctx, func(tx context.Context) error {
		if _, err := s.Authorize(tx, p, "novel", n, false); err != nil {
			return err
		}
		state, err := s.o.Store.Load(tx, p.TenantID, n, false)
		if err != nil {
			return err
		}
		out.WorldRevision = state.World.Revision
		ids := map[string]bool{}
		for _, e := range state.Entities {
			if e.DeletedAt != nil {
				continue
			}
			out.Entities = append(out.Entities, e)
			ids[e.ID] = true
		}
		for _, r := range state.Relations {
			if r.DeletedAt == nil && ids[r.SubjectID] && ids[r.ObjectID] {
				out.Relations = append(out.Relations, r)
			}
		}
		return nil
	})
	return out, err
}
func (s *Service) CloneSnapshot(ctx context.Context, p scope.Principal, n string, idMap map[string]string, snapshot d.Snapshot) (map[string]string, error) {
	if !uow.InTransaction(ctx) {
		return nil, uow.ErrTransactionRequired
	}
	out := map[string]string{}
	for k, v := range idMap {
		out[k] = v
	}
	err := s.o.UoW.Within(ctx, func(tx context.Context) error {
		if err := s.EnsureWorld(tx, p, n); err != nil {
			return err
		}
		return s.change(tx, p, "novel", n, "worldview.snapshot_cloned", func(state *d.State, c *d.Changes) error {
			if len(state.Entities) > 0 || len(state.Relations) > 0 || state.LastSeq != 0 {
				return d.StateError()
			}
			used := map[string]bool{}
			for _, old := range snapshot.Entities {
				if !scope.ValidID(old.ID) || old.DeletedAt != nil || !d.One(old.Kind, "character", "lore") {
					return d.Invalid()
				}
				id := out[old.ID]
				if id == "" {
					id = uuid.NewString()
					out[old.ID] = id
				}
				if !scope.ValidID(id) || used[id] {
					return d.Invalid()
				}
				used[id] = true
				e := newEntity(p, n, old.Kind, c.World.UpdatedAt)
				e.ID = id
				e.Name = old.Name
				e.Description = old.Description
				e.Status = old.Status
				e.Category = old.Category
				e.RoleType = old.RoleType
				e.Aliases = uniqueAliases(old.Aliases)
				if e.Kind == "character" {
					e.LifeStatus = "alive"
					if err := (d.CreateCharacter{Name: e.Name, Description: e.Description, Aliases: e.Aliases, RoleType: e.RoleType}).Validate(); err != nil {
						return err
					}
				} else {
					e.Aliases = []string{}
					if err := (d.CreateLore{Title: e.Name, Description: e.Description, Category: e.Category}).Validate(); err != nil {
						return err
					}
				}
				if !d.One(e.Status, "active", "archived") {
					return d.Invalid()
				}
				c.Entities = append(c.Entities, e)
			}
			clone := d.State{Entities: c.Entities}
			for _, old := range snapshot.Relations {
				subject, a := out[old.SubjectID]
				object, b := out[old.ObjectID]
				if !a || !b || old.DeletedAt != nil {
					return d.Invalid()
				}
				v := d.Relation{ID: uuid.NewString(), TenantID: p.TenantID, NovelID: n, CreatedAt: c.World.UpdatedAt, UpdatedAt: c.World.UpdatedAt, Revision: 1, SubjectID: subject, ObjectID: object, Predicate: old.Predicate, Description: old.Description, Weight: old.Weight, Status: old.Status}
				if err := (d.CreateRelation{SubjectID: subject, ObjectID: object, Predicate: v.Predicate, Description: v.Description, Weight: v.Weight}).Validate(); err != nil {
					return err
				}
				if !d.One(v.Status, "active", "retired") {
					return d.Invalid()
				}
				clone.Relations = c.Relations
				if err := relationValid(&clone, v); err != nil {
					return err
				}
				c.Relations = append(c.Relations, v)
			}
			return nil
		})
	})
	return out, err
}
