package jobs

import (
	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/scope"
	"InkLore/internal/platform/uow"
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"

	"github.com/google/uuid"
)

type Service struct {
	manager  *uow.Manager
	opts     Options
	draining atomic.Bool
}

func New(m *uow.Manager, o Options) (*Service, error) {
	if m == nil || o.Identity == nil {
		return nil, pe.Contract(500, "platform.internal_error", nil)
	}
	return &Service{manager: m, opts: o}, nil
}
func invalid() error { return pe.Contract(400, "platform.validation_failed", nil) }
func missing() error { return pe.Contract(404, "platform.resource_not_found", nil) }
func terminal(state string) bool {
	return state == "succeeded" || state == "failed" || state == "cancelled"
}
func validDomain(s string) bool {
	switch s {
	case "platform", "content", "outline", "worldview", "inspiration", "compliance", "agent", "llm":
		return true
	}
	return false
}
func (s *Service) SetDraining(v bool) { s.draining.Store(v) }
func (s *Service) Enqueue(ctx context.Context, in EnqueueInput) (Job, error) {
	var out Job
	db, e := s.manager.Require(ctx)
	if e != nil {
		return out, e
	}
	if !validDomain(in.Domain) || !machine.MatchString(in.Kind) || !scope.ValidID(in.ResourceID) || len(in.ExecutionKey) < 1 || len(in.ExecutionKey) > 200 || strings.ContainsAny(in.ExecutionKey, "\r\n\t") {
		return out, invalid()
	}
	if in.Metadata.RequestID != "" && !scope.ValidID(in.Metadata.RequestID) {
		return out, invalid()
	}
	if in.Metadata.TraceID != "" && !tracePattern.MatchString(in.Metadata.TraceID) {
		return out, invalid()
	}
	if !validTraceParent(in.Metadata.TraceParent) {
		return out, invalid()
	}
	if _, e = s.opts.Identity.Authorize(ctx, in.Principal); e != nil {
		return out, e
	}
	if s.opts.Resources == nil {
		return out, pe.Contract(403, "auth.forbidden", nil)
	}
	if e = s.opts.Resources.AuthorizeJob(ctx, in.Principal, in.Domain, in.Kind, in.ResourceID, true); e != nil {
		return out, e
	}
	payload, e := json.Marshal(in.Metadata)
	if e != nil {
		return out, e
	}
	q := db.Raw("INSERT INTO platform_jobs (id,tenant_id,owner_id,domain,job_kind,resource_id,execution_key,payload) VALUES (?,?,?,?,?,?,?,?::jsonb) ON CONFLICT (tenant_id,domain,execution_key) DO NOTHING RETURNING *", uuid.NewString(), in.Principal.TenantID, in.Principal.UserID, in.Domain, in.Kind, in.ResourceID, in.ExecutionKey, string(payload)).Scan(&out)
	if q.Error != nil {
		return out, q.Error
	}
	if q.RowsAffected == 0 {
		q = db.Raw("SELECT * FROM platform_jobs WHERE tenant_id=? AND domain=? AND execution_key=?", in.Principal.TenantID, in.Domain, in.ExecutionKey).Scan(&out)
		if q.Error != nil {
			return Job{}, q.Error
		}
		if out.OwnerID != in.Principal.UserID || out.ResourceID != in.ResourceID || out.JobKind != in.Kind {
			return Job{}, pe.Contract(409, "platform.idempotency_conflict", nil)
		}
		return out, nil
	}
	if e = s.event(ctx, "job.enqueued", out); e != nil {
		return Job{}, e
	}
	return out, nil
}
func validTraceParent(v string) bool {
	if v == "" {
		return true
	}
	if len(v) != 55 || v[:3] != "00-" || v[35] != '-' || v[52] != '-' {
		return false
	}
	parts := strings.Split(v, "-")
	if len(parts) != 4 || !tracePattern.MatchString(parts[1]) || parts[1] == strings.Repeat("0", 32) || parts[2] == strings.Repeat("0", 16) {
		return false
	}
	for _, p := range parts[2:] {
		for _, c := range p {
			if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
				return false
			}
		}
	}
	return true
}
func (s *Service) event(ctx context.Context, action string, j Job) error {
	if s.opts.OnEvent != nil {
		return s.opts.OnEvent(ctx, Event{j, action})
	}
	return nil
}
func (s *Service) Get(ctx context.Context, p identity.Principal, id string) (Job, error) {
	var out Job
	if !scope.ValidID(id) {
		return out, invalid()
	}
	if _, e := s.opts.Identity.Authorize(ctx, p); e != nil {
		return out, e
	}
	db, e := s.manager.DB(ctx)
	if e != nil {
		return out, e
	}
	q := db.Raw("SELECT * FROM platform_jobs WHERE tenant_id=? AND owner_id=? AND id=?", p.TenantID, p.UserID, id).Scan(&out)
	if q.Error != nil {
		return out, q.Error
	}
	if q.RowsAffected != 1 {
		return out, missing()
	}
	if s.opts.Resources == nil {
		return Job{}, missing()
	}
	if e = s.opts.Resources.AuthorizeJob(ctx, p, out.Domain, out.JobKind, out.ResourceID, false); e != nil {
		return Job{}, e
	}
	return out, nil
}
func (s *Service) AuthorizeCommand(ctx context.Context, p identity.Principal, id string, revision int64) (AuthorizedCommand, error) {
	var proof AuthorizedCommand
	if revision < 1 || revision >= MaxRevision {
		return proof, invalid()
	}
	db, e := s.manager.Require(ctx)
	if e != nil {
		return proof, e
	}
	j, e := s.Get(ctx, p, id)
	if e != nil {
		return proof, e
	}
	if e = s.opts.Resources.AuthorizeJob(ctx, p, j.Domain, j.JobKind, j.ResourceID, true); e != nil {
		return proof, e
	}
	q := db.Raw("SELECT * FROM platform_jobs WHERE tenant_id=? AND owner_id=? AND id=? FOR UPDATE", p.TenantID, p.UserID, id).Scan(&j)
	if q.Error != nil {
		return proof, q.Error
	}
	if q.RowsAffected != 1 {
		return proof, missing()
	}
	if j.Revision != revision {
		return proof, pe.Contract(409, "platform.version_conflict", map[string]any{"current_revision": j.Revision})
	}
	return AuthorizedCommand{s, ctx, j}, nil
}
func (s *Service) Cancel(ctx context.Context, p identity.Principal, id string, revision int64) (Job, error) {
	var out Job
	e := s.manager.Within(ctx, func(tx context.Context) error {
		proof, e := s.AuthorizeCommand(tx, p, id, revision)
		if e != nil {
			return e
		}
		if terminal(proof.job.State) {
			out = proof.job
			return nil
		}
		if e = s.opts.Resources.CancelJob(tx, p, proof.job); e != nil {
			return e
		}
		out, e = s.CompleteCommand(tx, proof, "cancelled", "")
		return e
	})
	return out, e
}
