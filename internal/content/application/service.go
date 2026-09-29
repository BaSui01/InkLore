package application

import (
	d "InkLore/internal/content/domain"
	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/idempotency"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/scope"
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"
	"unicode/utf8"
)

func New(o Options) (*Service, error) {
	if o.Repo == nil || o.UOW == nil || o.Scope == nil || o.Identity == nil || o.Idempotency == nil || o.Cursor == nil || o.Events == nil || o.World == nil {
		return nil, errors.New("content: all application ports are required")
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	return &Service{o: o}, nil
}
func (s *Service) Authenticate(ctx context.Context, token string) (identity.Principal, error) {
	return s.o.Identity.Authenticate(ctx, token, "creator")
}
func public(err error) error {
	var e *d.RuleError
	if errors.As(err, &e) {
		status := 422
		switch e.Code {
		case "content.not_found":
			status = 404
		case "content.invalid_state", "content.version_conflict", "content.review_required":
			status = 409
		case "content.job_capacity_exceeded":
			status = 429
		case "content.export_too_large":
			status = 413
		case "content.unsupported_encoding", "content.unsupported_document":
			status = 422
		}
		details := map[string]any{}
		if e.CurrentRevision > 0 {
			details["current_revision"] = e.CurrentRevision
		}
		return pe.Contract(status, e.Code, details)
	}
	var p *pe.AppError
	if errors.As(err, &p) && p.Code == "platform.resource_not_found" {
		return pe.Contract(404, "content.not_found", nil)
	}
	return err
}
func invalid() error              { return pe.Contract(400, "platform.validation_failed", nil) }
func (s *Service) now() time.Time { return s.o.Now().UTC().Truncate(time.Microsecond) }
func (s *Service) owner(ctx context.Context, p identity.Principal, kind, id string, trash bool, action scope.Action) (Owner, error) {
	current, e := s.o.Identity.Authorize(ctx, p, "creator")
	if e != nil {
		return Owner{}, e
	}
	if current.Audience != "creator" {
		return Owner{}, pe.Contract(403, "auth.forbidden", nil)
	}
	o := Owner{TenantID: p.TenantID, ActorID: p.UserID}
	if id == "" {
		return o, nil
	}
	if !scope.ValidID(id) {
		return o, invalid()
	}
	var sc scope.Scope
	if trash {
		sc, e = s.o.Scope.AssertTrashOwner(ctx, Principal(current), scope.Resource{Kind: kind, ID: id}, action)
	} else {
		sc, e = s.o.Scope.Assert(ctx, Principal(current), scope.Resource{Kind: kind, ID: id}, action)
	}
	if e != nil {
		return o, e
	}
	if kind == "novel" {
		o.NovelID = id
	} else if sc.NovelID != nil {
		o.NovelID = *sc.NovelID
	} else {
		return o, d.Missing()
	}
	return o, nil
}

// lockChildren runs before any business aggregate lock. The novel scope is already
// exclusively locked, so its membership cannot change while we order child locks.
func (s *Service) lockChildren(ctx context.Context, p identity.Principal, o Owner) error {
	org, e := s.o.Repo.Organization(ctx, o)
	if e != nil {
		return e
	}
	resources := []scope.Resource{}
	for _, v := range org.Volumes {
		resources = append(resources, scope.Resource{Kind: "volume", ID: v.ID})
	}
	for _, c := range org.Chapters {
		resources = append(resources, scope.Resource{Kind: "chapter", ID: c.ID})
	}
	sort.Slice(resources, func(i, j int) bool { return resources[i].ID < resources[j].ID })
	for _, r := range resources {
		if _, e = s.o.Scope.Assert(ctx, Principal(p), r, scope.Write); e != nil {
			return e
		}
	}
	return nil
}
func (s *Service) execute(ctx context.Context, c Command, kind, id string, trash, allChildren bool, status int, fn commandFn) (idempotency.Result, error) {
	if c.Request.TenantID != c.Principal.TenantID || c.Request.ActorID != c.Principal.UserID {
		return idempotency.Result{}, invalid()
	}
	var owner Owner
	result, e := s.o.Idempotency.Execute(ctx, c.Request, func(tx context.Context) error {
		var e error
		owner, e = s.owner(tx, c.Principal, kind, id, trash, scope.Write)
		if e != nil {
			return public(e)
		}
		if allChildren && owner.NovelID != "" {
			e = s.lockChildren(tx, c.Principal, owner)
		}
		return public(e)
	}, func(tx context.Context) (idempotency.CommandOutcome, error) {
		value, e := fn(tx, owner)
		if e != nil {
			return idempotency.CommandOutcome{}, public(e)
		}
		raw, e := json.Marshal(value)
		if e != nil {
			return idempotency.CommandOutcome{}, e
		}
		return idempotency.CommandOutcome{Commit: true, Result: idempotency.Result{Status: status, Body: raw}}, nil
	})
	return result, public(e)
}
func (s *Service) read(ctx context.Context, p identity.Principal, kind, id string, trash bool, fn func(context.Context, Owner) error) error {
	return public(s.o.UOW.Within(ctx, func(tx context.Context) error {
		o, e := s.owner(tx, p, kind, id, trash, scope.Read)
		if e != nil {
			return e
		}
		return fn(tx, o)
	}))
}
func (s *Service) active(ctx context.Context, o Owner) (d.Novel, error) {
	n, e := s.o.Repo.Novel(ctx, o, o.NovelID, true)
	if e != nil {
		return n, e
	}
	if n.Status != "active" {
		return n, d.State()
	}
	if n.Revision >= d.MaxRevision {
		return n, &d.RuleError{Code: "content.version_conflict", CurrentRevision: n.Revision}
	}
	return n, nil
}
func (s *Service) projectNovel(ctx context.Context, n d.Novel, expected int64) error {
	return s.o.Scope.UpsertScope(ctx, scope.Projection{TenantID: n.TenantID, ResourceID: n.ID, ResourceKind: "novel", OwnerID: n.OwnerID, CurrentRevision: n.Revision, DeletedAt: n.DeletedAt}, expected)
}
func (s *Service) projectVolume(ctx context.Context, o Owner, v d.Volume, expected int64) error {
	return s.o.Scope.UpsertScope(ctx, scope.Projection{TenantID: o.TenantID, ResourceID: v.ID, ResourceKind: "volume", OwnerID: o.ActorID, NovelID: &o.NovelID, CurrentRevision: v.Revision, DeletedAt: v.DeletedAt}, expected)
}
func (s *Service) projectChapter(ctx context.Context, o Owner, c d.ChapterRecord, expected int64) error {
	if e := s.o.Scope.UpsertScope(ctx, scope.Projection{TenantID: o.TenantID, ResourceID: c.ID, ResourceKind: "chapter", OwnerID: o.ActorID, NovelID: &o.NovelID, CurrentRevision: c.Revision, ContentHash: &c.ContentHash, DeletedAt: c.DeletedAt}, expected); e != nil {
		return e
	}
	return s.o.Scope.AppendRevision(ctx, scope.Revision{TenantID: o.TenantID, ResourceID: c.ID, Revision: c.Revision, ContentHash: c.ContentHash, ContentLength: int64(utf8.RuneCountInString(c.Body))})
}

// Every immutable version also has a revision-1 authorization proof. Chapter
// history proves editable revision coordinates; this proof authenticates the
// version identifier and hash supplied by the confirmation adapter.
func (s *Service) projectVersion(ctx context.Context, o Owner, v d.Version) error {
	if e := s.o.Scope.UpsertScope(ctx, scope.Projection{TenantID: o.TenantID, ResourceID: v.ID, ResourceKind: "chapter_version", OwnerID: o.ActorID, NovelID: &o.NovelID, CurrentRevision: 1, ContentHash: &v.ContentHash}, 0); e != nil {
		return e
	}
	return s.o.Scope.AppendRevision(ctx, scope.Revision{TenantID: o.TenantID, ResourceID: v.ID, Revision: 1, ContentHash: v.ContentHash, ContentLength: int64(utf8.RuneCountInString(v.Body))})
}
func (s *Service) bumpNovel(ctx context.Context, o Owner, n *d.Novel) error {
	expected := n.Revision
	if expected >= d.MaxRevision {
		return &d.RuleError{Code: "content.version_conflict", CurrentRevision: expected}
	}
	n.Revision++
	n.UpdatedAt = s.now()
	if n.WordCount < 0 || n.WordCount > 400000000 {
		return d.Invalid()
	}
	if e := s.o.Repo.PutNovel(ctx, o, *n, expected); e != nil {
		return e
	}
	return s.projectNovel(ctx, *n, expected)
}
func (s *Service) event(ctx context.Context, p identity.Principal, action, kind, id, novel string, revision int64) error {
	return s.o.Events.Record(ctx, Event{Principal: Principal(p), Action: action, ResourceKind: kind, ResourceID: id, NovelID: novel, Revision: revision})
}
