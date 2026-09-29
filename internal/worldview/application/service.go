// Package application coordinates worldview use cases without database handles.
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

type Options struct {
	Store   d.Repository
	UoW     *uow.Manager
	Scope   *scope.Authorizer
	OnEvent func(context.Context, d.ChangeEvent) error
	// This trusted application callback must prove system capability and, for purge,
	// that the novel has remained deleted for at least thirty days. Never HTTP input.
	AuthorizeMaintenance func(context.Context, string, string, string) error
}
type Service struct{ o Options }

func New(o Options) *Service { return &Service{o: o} }
func (s *Service) ValidateOptions() error {
	if s == nil || s.o.Store == nil || s.o.UoW == nil || s.o.Scope == nil || s.o.OnEvent == nil {
		return errors.New("worldview: store, UoW, scope and transactional event sink required")
	}
	return nil
}
func (s *Service) Authorize(ctx context.Context, p scope.Principal, kind, id string, write bool) (string, error) {
	if !scope.ValidID(id) {
		return "", d.Malformed()
	}
	novel := id
	var err error
	if kind != "novel" {
		novel, err = s.o.Store.FindNovel(ctx, p.TenantID, kind, id)
		if err != nil {
			return "", err
		}
	}
	action := scope.Read
	if write {
		action = scope.Write
	}
	_, err = s.o.Scope.Assert(ctx, p, scope.Resource{Kind: "novel", ID: novel}, action)
	return novel, err
}
func (s *Service) AuthorizeReplay(ctx context.Context, p scope.Principal, kind, id string, write, includeDeleted bool) error {
	if _, err := s.Authorize(ctx, p, kind, id, write); err != nil {
		return err
	}
	if kind != "novel" && !includeDeleted {
		return s.o.Store.Visible(ctx, p.TenantID, kind, id)
	}
	return nil
}
func (s *Service) EnsureWorld(ctx context.Context, p scope.Principal, n string) error {
	if !uow.InTransaction(ctx) {
		return uow.ErrTransactionRequired
	}
	return s.o.UoW.Within(ctx, func(tx context.Context) error {
		if _, err := s.Authorize(tx, p, "novel", n, true); err != nil {
			return err
		}
		_, err := s.o.Store.EnsureWorld(tx, p.TenantID, n)
		return err
	})
}
func (s *Service) read(ctx context.Context, p scope.Principal, kind, id string, fn func(context.Context, d.State) error) error {
	return s.o.UoW.Within(ctx, func(tx context.Context) error {
		n, err := s.Authorize(tx, p, kind, id, false)
		if err != nil {
			return err
		}
		state, err := s.o.Store.Load(tx, p.TenantID, n, false)
		if err != nil {
			return err
		}
		return fn(tx, state)
	})
}
func (s *Service) change(ctx context.Context, p scope.Principal, kind, id, action string, fn func(*d.State, *d.Changes) error) error {
	return s.o.UoW.Within(ctx, func(tx context.Context) error {
		n, err := s.Authorize(tx, p, kind, id, true)
		if err != nil {
			return err
		}
		state, err := s.o.Store.Load(tx, p.TenantID, n, true)
		if err != nil {
			return err
		}
		if state.World.Revision >= d.MaxRevision {
			return d.Version(state.World.Revision)
		}
		c := d.Changes{World: state.World, OldWorldRevision: state.World.Revision, LastSeq: state.LastSeq}
		c.World.Revision++
		c.World.UpdatedAt = time.Now().UTC()
		if err = fn(&state, &c); err != nil {
			return err
		}
		return s.save(tx, p, c, id, action)
	})
}
func (s *Service) save(ctx context.Context, p scope.Principal, c d.Changes, id, action string) error {
	if err := s.o.Store.Save(ctx, c); err != nil {
		return err
	}
	for _, e := range c.Entities {
		n := e.NovelID
		projection := scope.Projection{TenantID: e.TenantID, ResourceID: e.ID, ResourceKind: "entity", OwnerID: p.ActorID, NovelID: &n, CurrentRevision: e.Revision, DeletedAt: e.DeletedAt}
		if err := s.o.Scope.UpsertScope(ctx, projection, e.Revision-1); err != nil {
			return err
		}
	}
	if s.o.OnEvent == nil {
		return errors.New("worldview: transactional event sink required")
	}
	if d.One(action, "worldview.character_created", "worldview.lore_created") && len(c.Entities) == 1 {
		id = c.Entities[0].ID
	}
	if action == "worldview.relation_created" && len(c.Relations) == 1 {
		id = c.Relations[0].ID
	}
	return s.o.OnEvent(ctx, d.ChangeEvent{TenantID: p.TenantID, ActorID: p.ActorID, NovelID: c.World.NovelID, ResourceID: id, Action: action, Revision: c.World.Revision})
}
func expected(got, want int64) error {
	if got < 1 || got > d.MaxRevision {
		return d.Malformed()
	}
	if got != want || want >= d.MaxRevision {
		return d.Version(want)
	}
	return nil
}
func uniqueAliases(values []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, v := range values {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}
func newEntity(p scope.Principal, n, kind string, now time.Time) d.Entity {
	return d.Entity{Character: d.Character{ID: uuid.NewString(), TenantID: p.TenantID, NovelID: n, CreatedAt: now, UpdatedAt: now, Revision: 1, Status: "active", Aliases: []string{}}, Kind: kind}
}
func (s *Service) GetWorldRevision(ctx context.Context, p scope.Principal, n string) (d.WorldRevision, error) {
	var out d.WorldRevision
	err := s.read(ctx, p, "novel", n, func(_ context.Context, state d.State) error { out = state.World; return nil })
	return out, err
}
func (s *Service) GetEntity(ctx context.Context, p scope.Principal, kind, id string) (d.Entity, error) {
	var out d.Entity
	err := s.read(ctx, p, kind, id, func(_ context.Context, state d.State) error {
		v := state.Entity(id)
		if v == nil || v.Kind != kind || v.DeletedAt != nil {
			return d.Missing()
		}
		out = *v
		return nil
	})
	return out, err
}
func (s *Service) GetRelation(ctx context.Context, p scope.Principal, id string) (d.Relation, error) {
	var out d.Relation
	err := s.read(ctx, p, "relation", id, func(_ context.Context, state d.State) error {
		for _, v := range state.Relations {
			if v.ID == id && v.DeletedAt == nil {
				out = v
				return nil
			}
		}
		return d.Missing()
	})
	return out, err
}
func (s *Service) Page(ctx context.Context, p scope.Principal, n string, q d.Query) (d.Records, error) {
	var out d.Records
	err := s.o.UoW.Within(ctx, func(tx context.Context) error {
		if _, err := s.Authorize(tx, p, "novel", n, false); err != nil {
			return err
		}
		var err error
		out, err = s.o.Store.Page(tx, p.TenantID, n, q)
		return err
	})
	return out, err
}
