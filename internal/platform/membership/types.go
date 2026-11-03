// Package membership owns tenant member administration using the shared PostgreSQL UoW.
package membership

import (
	"context"
	"time"

	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/uow"
)

// Authorizer rechecks current session and membership permissions. Composition may
// inject identity.Service directly. JWT roles alone are never sufficient.
type Authorizer interface {
	Authorize(context.Context, identity.Principal, ...string) (identity.Principal, error)
}

// PolicyReferenceReleaser removes LLM-owned policy-version pins for guide
// sessions that are being deleted, keeping frozen-version deletion refusal
// accurate without platform code reading LLM tables.
type PolicyReferenceReleaser interface {
	ReleaseGuideSessionReferences(ctx context.Context, tenantID string, sessionIDs []string) error
}

type Service struct {
	manager    *uow.Manager
	authorizer Authorizer
	policyRefs PolicyReferenceReleaser
}

func New(manager *uow.Manager, authorizer Authorizer) *Service {
	return &Service{manager: manager, authorizer: authorizer}
}

// SetPolicyReferenceReleaser wires the composition-root adapter after the
// LLM domain is built.
func (s *Service) SetPolicyReferenceReleaser(r PolicyReferenceReleaser) {
	s.policyRefs = r
}

type Member struct {
	ID          string    `json:"id"`
	Revision    int64     `json:"revision"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	UserID      string    `json:"user_id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Roles       []string  `json:"roles" gorm:"-"`
	Status      string    `json:"status"`
}

type CreateMember struct {
	Email       string   `json:"email"`
	DisplayName string   `json:"display_name"`
	Password    string   `json:"password"`
	Roles       []string `json:"roles"`
}
type UpdateMember struct {
	ExpectedRevision int64    `json:"expected_revision"`
	Roles            []string `json:"roles"`
	Status           string   `json:"status"`
}
type ResetPassword struct {
	ExpectedRevision int64  `json:"expected_revision"`
	NewPassword      string `json:"new_password"`
}
type RevisionCommand struct {
	ExpectedRevision int64 `json:"expected_revision"`
}
type RevokeSessions struct {
	RevokedCount int64 `json:"revoked_count"`
	Revision     int64 `json:"revision"`
}

// ListQuery is a decoded keyset, not an HTTP cursor. The transport must verify
// cursor signature, expiry, tenant, actor, filters and sort before calling List.
// The next anchor is the final returned Member's UpdatedAt and ID.
type ListQuery struct {
	Limit           int
	BeforeUpdatedAt time.Time
	BeforeID        string
}

// Page is internal; the transport supplies the frozen next_cursor field.
type Page struct {
	Items   []Member
	HasMore bool
}
