// Package files owns R1 PostgreSQL bytea files and safe file validation.
package files

import (
	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/scope"
	"InkLore/internal/platform/uow"
	"context"
	"io"
	"time"
)

const InputLimit int64 = 20 << 20
const OutputLimit int64 = 100 << 20
const MaxRevision int64 = 9007199254740991
const DOCX = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"

type Descriptor struct {
	ID        string     `json:"id"`
	Revision  int64      `json:"revision"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	Filename  string     `json:"filename"`
	MediaType string     `json:"media_type"`
	Purpose   string     `json:"purpose"`
	SizeBytes int64      `json:"size_bytes"`
	SHA256    string     `json:"sha256"`
	ExpiresAt *time.Time `json:"expires_at"`
	DeletedAt *time.Time `json:"deleted_at"`
}
type Prepared struct {
	descriptor Descriptor
	payload    []byte
	novelID    *string
	output     bool
}

func (p Prepared) Descriptor() Descriptor { return p.descriptor }

// DigestMetadata is the typed canonical request used by the composition root's idempotency executor.
type DigestMetadata struct {
	Filename  string  `json:"filename"`
	MediaType string  `json:"media_type"`
	Purpose   string  `json:"purpose"`
	NovelID   *string `json:"novel_id"`
	SHA256    string  `json:"sha256"`
	SizeBytes int64   `json:"size_bytes"`
}

func (p Prepared) DigestMetadata() DigestMetadata {
	d := p.descriptor
	return DigestMetadata{d.Filename, d.MediaType, d.Purpose, p.novelID, d.SHA256, d.SizeBytes}
}

type IdentityPort interface {
	Authorize(context.Context, identity.Principal, ...string) (identity.Principal, error)
}
type ScopePort interface {
	Assert(context.Context, scope.Principal, scope.Resource, scope.Action) (scope.Scope, error)
}

// ReferencesPort is adapted at app; callers must lock the file before creating a domain reference.
type ReferencesPort interface {
	InUse(context.Context, string, string) (bool, error)
}
type Event struct {
	Principal identity.Principal
	Action    string
	File      Descriptor
}
type Options struct {
	Identity   IdentityPort
	Scope      ScopePort
	References ReferencesPort
	OnEvent    func(context.Context, Event) error
}
type Service struct {
	manager *uow.Manager
	opts    Options
}

func New(m *uow.Manager, o Options) (*Service, error) {
	if m == nil || o.Identity == nil || o.Scope == nil {
		return nil, pe.Contract(500, "platform.internal_error", nil)
	}
	return &Service{m, o}, nil
}
func invalid() error  { return pe.Contract(400, "platform.validation_failed", nil) }
func missing() error  { return pe.Contract(404, "platform.resource_not_found", nil) }
func badType() error  { return pe.Contract(422, "platform.file_type_invalid", nil) }
func tooLarge() error { return pe.Contract(413, "platform.file_too_large", nil) }

type Opened struct {
	Descriptor Descriptor
	Reader     io.ReadSeeker
}
