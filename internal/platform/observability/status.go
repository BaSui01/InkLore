// Package observability exposes fixed, safe R1 operational projections.
package observability

import (
	"InkLore/internal/platform/cursor"
	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/uow"
	"context"
	"errors"
	"time"
	"unicode/utf8"
)

type IdentityPort interface {
	Authorize(context.Context, identity.Principal, ...string) (identity.Principal, error)
}
type Pinger interface{ Ping(context.Context) error }
type Options struct {
	Identity         IdentityPort
	Redis            Pinger
	Version          string
	Cursor           *cursor.Codec
	SecretReferences []SecretReference
	Diagnostics      DiagnosticPort
}
type Service struct {
	manager       *uow.Manager
	opts          Options
	references    []SecretReference
	referenceHash string
}

func New(m *uow.Manager, o Options) (*Service, error) {
	if m == nil || o.Identity == nil || o.Cursor == nil || utf8.RuneCountInString(o.Version) > 80 || o.Version == "" {
		return nil, errors.New("observability dependencies required")
	}
	s := &Service{manager: m, opts: o}
	if e := s.configureReferences(o.SecretReferences); e != nil {
		return nil, e
	}
	return s, nil
}

type Dependency struct {
	Name      string    `json:"name"`
	State     string    `json:"state"`
	CheckedAt time.Time `json:"checked_at"`
}
type Queue struct {
	Queued            int64  `json:"queued"`
	Leased            int64  `json:"leased"`
	Failed            int64  `json:"failed"`
	OldestWaitSeconds *int64 `json:"oldest_wait_seconds"`
}
type Outbox struct {
	Pending    int64 `json:"pending"`
	DeadLetter int64 `json:"dead_letter"`
}
type Status struct {
	ObservedAt     time.Time    `json:"observed_at"`
	ServiceVersion string       `json:"service_version"`
	State          string       `json:"state"`
	Dependencies   []Dependency `json:"dependencies"`
	Queue          Queue        `json:"queue"`
	Outbox         Outbox       `json:"outbox"`
}

func (s *Service) admin(ctx context.Context, p identity.Principal, roles ...string) (identity.Principal, error) {
	p, e := s.opts.Identity.Authorize(ctx, p, roles...)
	if e != nil {
		return p, e
	}
	if p.Audience != "admin" {
		return identity.Principal{}, pe.Contract(403, "auth.forbidden", nil)
	}
	return p, nil
}
func (s *Service) Status(ctx context.Context, p identity.Principal) (Status, error) {
	var out Status
	current, e := s.admin(ctx, p, "operator", "auditor")
	if e != nil {
		return out, e
	}
	out = Status{ObservedAt: time.Now().UTC(), ServiceVersion: s.opts.Version, State: "ready", Dependencies: []Dependency{}}
	probe, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	db, e := s.manager.DB(probe)
	if e != nil {
		return Status{}, e
	}
	var one int
	pgErr := db.Raw("SELECT 1").Scan(&one).Error
	pgState := "healthy"
	if pgErr != nil {
		pgState = "unavailable"
		out.State = "not_ready"
	}
	out.Dependencies = append(out.Dependencies, Dependency{"postgres", pgState, time.Now().UTC()})
	redisState := "healthy"
	redisCtx, redisCancel := context.WithTimeout(ctx, 2*time.Second)
	if s.opts.Redis == nil || s.opts.Redis.Ping(redisCtx) != nil {
		redisState = "unavailable"
		if out.State == "ready" {
			out.State = "degraded"
		}
	}
	redisCancel()
	out.Dependencies = append(out.Dependencies, Dependency{"redis", redisState, time.Now().UTC()})
	if pgErr != nil {
		return out, nil
	}
	db, e = s.manager.DB(ctx)
	if e != nil {
		return Status{}, e
	}
	e = db.Raw("SELECT count(*) FILTER (WHERE state IN ('queued','retry_wait')) AS queued,count(*) FILTER (WHERE state='leased') AS leased,count(*) FILTER (WHERE state='failed') AS failed,CASE WHEN count(*) FILTER (WHERE state IN ('queued','retry_wait') AND available_at<=clock_timestamp())=0 THEN NULL ELSE GREATEST(0,FLOOR(EXTRACT(EPOCH FROM (clock_timestamp()-min(available_at) FILTER (WHERE state IN ('queued','retry_wait') AND available_at<=clock_timestamp())))))::bigint END AS oldest_wait_seconds FROM platform_jobs WHERE tenant_id=?", current.TenantID).Scan(&out.Queue).Error
	if e != nil {
		return Status{}, e
	}
	e = db.Raw("SELECT count(*) FILTER (WHERE state='pending') AS pending,count(*) FILTER (WHERE state='dead_letter') AS dead_letter FROM platform_outbox WHERE tenant_id=?", current.TenantID).Scan(&out.Outbox).Error
	if e != nil {
		return Status{}, e
	}
	return out, nil
}
