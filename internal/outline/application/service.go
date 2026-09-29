// package application implements the frozen outline use cases through owned ports.
package application

import (
	d "InkLore/internal/outline/domain"
	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/scope"
	"context"
	"errors"
	"strings"
	"time"
)

type UnitOfWork interface {
	Within(context.Context, func(context.Context) error) error
	WithinReadSnapshot(context.Context, func(context.Context) error) error
}
type ScopePort interface {
	Assert(context.Context, scope.Principal, scope.Resource, scope.Action) (scope.Scope, error)
	AssertTrashOwner(context.Context, scope.Principal, scope.Resource, scope.Action) (scope.Scope, error)
	UpsertScope(context.Context, scope.Projection, int64) error
	AppendRevision(context.Context, scope.Revision) error
	AssertChapterSnapshot(context.Context, scope.Principal, string, int64, string) (scope.SnapshotProof, error)
}
type ChapterContextPort interface {
	ChapterVolume(context.Context, scope.Principal, string, string) (string, error)
}
type Mutation struct {
	Principal                                 scope.Principal
	NovelID, ResourceKind, ResourceID, Action string
	Revision                                  int64
}
type Options struct {
	Repository     d.Repository
	UoW            UnitOfWork
	Scope          ScopePort
	ChapterContext ChapterContextPort
	OnMutation     func(context.Context, Mutation) error
}
type Service struct{ Options }

func NewService(o Options) (*Service, error) {
	if o.Repository == nil || o.UoW == nil || o.Scope == nil || o.ChapterContext == nil || o.OnMutation == nil {
		return nil, errors.New("outline: repository, UoW, scope, chapter context and mutation hook required")
	}
	return &Service{Options: o}, nil
}
func invalid(field string) error {
	return pe.Contract(422, "outline.validation_failed", map[string]any{"field_errors": []map[string]string{{"field": field, "reason": "invalid"}}})
}
func missing() error    { return pe.Contract(404, "outline.not_found", nil) }
func stateError() error { return pe.Contract(409, "outline.invalid_state", nil) }
func conflict(rev int64) error {
	return pe.Contract(409, "outline.version_conflict", map[string]any{"current_revision": rev})
}
func cas(actual, want int64) error {
	if want < 1 || want > scope.MaxRevision {
		return pe.Contract(400, "platform.validation_failed", nil)
	}
	if actual != want || actual >= scope.MaxRevision {
		return conflict(actual)
	}
	return nil
}
func mapScope(err error) error {
	var e *pe.AppError
	if errors.As(err, &e) && e.Code == "platform.resource_not_found" {
		return missing()
	}
	return err
}
func (s *Service) assert(ctx context.Context, p scope.Principal, novel, kind, id string, action scope.Action) error {
	v, err := s.Scope.Assert(ctx, p, scope.Resource{Kind: kind, ID: id}, action)
	if err != nil {
		return mapScope(err)
	}
	if kind == "novel" {
		if v.ResourceID != novel {
			return missing()
		}
	} else if v.NovelID == nil || *v.NovelID != novel {
		return invalid("target_id")
	}
	return nil
}

// Authorize is also the idempotency replay gate. Deleted children are authorized
// through their still-live novel; visibility of tombstones is checked per use case.
func (s *Service) Authorize(ctx context.Context, p scope.Principal, kind, id string, write bool) (string, error) {
	return s.authorize(ctx, p, kind, id, write, false)
}
func (s *Service) AuthorizeCommand(ctx context.Context, p scope.Principal, kind, id string, trash bool) (string, error) {
	return s.authorize(ctx, p, kind, id, true, trash)
}
func (s *Service) authorize(ctx context.Context, p scope.Principal, kind, id string, write, trash bool) (string, error) {
	if !scope.ValidID(id) {
		return "", pe.Contract(400, "platform.validation_failed", nil)
	}
	novel := id
	var err error
	if kind != "novel" {
		novel, err = s.Repository.ResolveNovel(ctx, p.TenantID, kind, id)
		if err != nil {
			return "", err
		}
	}
	action := scope.Read
	if write {
		action = scope.Write
	}
	if err = s.assert(ctx, p, novel, "novel", novel, action); err != nil {
		return "", err
	}
	if kind != "novel" {
		var v scope.Scope
		var err error
		if trash {
			v, err = s.Scope.AssertTrashOwner(ctx, p, scope.Resource{Kind: kind, ID: id}, action)
		} else {
			v, err = s.Scope.Assert(ctx, p, scope.Resource{Kind: kind, ID: id}, action)
		}
		if err != nil {
			return "", mapScope(err)
		}
		if v.NovelID == nil || *v.NovelID != novel {
			return "", missing()
		}
	}
	return novel, nil
}
func (s *Service) read(ctx context.Context, p scope.Principal, kind, id string) (*d.State, error) {
	var st *d.State
	err := s.UoW.Within(ctx, func(tx context.Context) error {
		novel, err := s.Authorize(tx, p, kind, id, false)
		if err != nil {
			return err
		}
		st, err = s.Repository.Load(tx, d.Scope{TenantID: p.TenantID, NovelID: novel}, false)
		return err
	})
	return st, err
}
func (s *Service) mutate(ctx context.Context, p scope.Principal, kind, id, action string, fn func(context.Context, *editState) error) error {
	return s.UoW.Within(ctx, func(tx context.Context) error {
		novel, err := s.authorize(tx, p, kind, id, true, strings.HasPrefix(action, "restore"))
		if err != nil {
			return err
		}
		st, err := s.Repository.Load(tx, d.Scope{TenantID: p.TenantID, NovelID: novel}, true)
		if err != nil {
			return err
		}
		e := newEdit(st, p, time.Now().UTC())
		if err = fn(tx, e); err != nil {
			return err
		}
		if err = e.touchMaster(); err != nil {
			return err
		}
		changes, err := e.changes()
		if err != nil {
			return err
		}
		if err = s.Repository.Save(tx, d.Scope{TenantID: p.TenantID, NovelID: novel}, changes); err != nil {
			return err
		}
		if err = s.project(tx, p, novel, changes); err != nil {
			return err
		}
		rev := int64(1)
		if n := e.master(); n != nil {
			rev = n.Revision
		}
		return s.OnMutation(tx, Mutation{Principal: p, NovelID: novel, ResourceKind: kind, ResourceID: id, Action: action, Revision: rev})
	})
}
func retained(t *time.Time, now time.Time) bool {
	return t != nil && t.After(now.Add(-30*24*time.Hour))
}
func ptr[T any](v T) *T { return &v }
