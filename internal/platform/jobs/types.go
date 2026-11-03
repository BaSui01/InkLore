// Package jobs implements durable scheduling; domain state remains domain-owned.
package jobs

import (
	"InkLore/internal/platform/identity"
	"context"
	"errors"
	"regexp"
	"time"
)

const LeaseDuration = 30 * time.Second
const HeartbeatInterval = 10 * time.Second
const MaxRevision int64 = 9007199254740991

var ErrLeaseLost = errors.New("job lease lost")
var ErrNoJob = errors.New("no eligible job")
var ErrTerminal = errors.New("job is terminal")
var ErrDraining = errors.New("job scheduler draining")
var machine = regexp.MustCompile("^[a-z][a-z0-9_]{0,99}$")
var tracePattern = regexp.MustCompile("^[0-9a-f]{32}$")

type Metadata struct {
	RequestID   string `json:"request_id,omitempty"`
	TraceID     string `json:"trace_id,omitempty"`
	TraceParent string `json:"traceparent,omitempty"`
}
type Job struct {
	ID                string     `json:"id"`
	TenantID          string     `json:"-"`
	OwnerID           string     `json:"-"`
	Domain            string     `json:"domain"`
	JobKind           string     `json:"job_kind"`
	ResourceID        string     `json:"resource_id"`
	ExecutionKey      string     `json:"-"`
	State             string     `json:"state"`
	Attempt           int        `json:"attempt"`
	AvailableAt       time.Time  `json:"available_at"`
	LeaseOwner        *string    `json:"-"`
	LeaseEpoch        int64      `json:"-"`
	LeaseUntil        *time.Time `json:"-"`
	CancelRequestedAt *time.Time `json:"cancel_requested_at"`
	LastErrorCode     *string    `json:"last_error_code"`
	Revision          int64      `json:"revision"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	Payload           string     `json:"-"`
}
type EnqueueInput struct {
	Principal                              identity.Principal
	Domain, Kind, ResourceID, ExecutionKey string
	Metadata                               Metadata
}

// Lease cannot be constructed from HTTP JSON: all authority fields are private.
type Lease struct {
	service           *Service
	tenant, id, owner string
	epoch             int64
	job               Job
	recovered         bool
}

func (l Lease) Job() Job        { return l.job }
func (l Lease) Epoch() int64    { return l.epoch }
func (l Lease) Recovered() bool { return l.recovered }

type IdentityPort interface {
	Authorize(context.Context, identity.Principal, ...string) (identity.Principal, error)
}

// ResourcePort verifies domain resources via app adapters, never cross-domain SQL.
type ResourcePort interface {
	AuthorizeJob(context.Context, identity.Principal, string, string, string, bool) error
	CancelJob(context.Context, identity.Principal, Job) error
}
type Event struct {
	Job    Job
	Action string
}
type Options struct {
	Identity  IdentityPort
	Resources ResourcePort
	OnEvent   func(context.Context, Event) error
}

// AuthorizedCommand is minted only after live identity+domain authorization and
// the job row lock. It is tied to that exact transaction context.
type AuthorizedCommand struct {
	service *Service
	ctx     context.Context
	job     Job
}
