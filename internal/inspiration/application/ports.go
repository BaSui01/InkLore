// package application orchestrates the inspiration and deconstruction use cases through owned ports.
package application

import (
	d "InkLore/internal/inspiration/domain"
	"InkLore/internal/platform/cursor"
	"InkLore/internal/platform/idempotency"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/scope"
	"context"
)

type UnitOfWork interface {
	Within(context.Context, func(context.Context) error) error
	WithinReadSnapshot(context.Context, func(context.Context) error) error
}

type IdentityPort interface {
	Authenticate(context.Context, string, string) (identity.Principal, error)
	Authorize(context.Context, identity.Principal, ...string) (identity.Principal, error)
}

type ScopePort interface {
	Assert(context.Context, scope.Principal, scope.Resource, scope.Action) (scope.Scope, error)
	AssertTrashOwner(context.Context, scope.Principal, scope.Resource, scope.Action) (scope.Scope, error)
	AssertResourceRevision(context.Context, scope.Principal, scope.Resource, int64, string) (scope.SnapshotProof, error)
}

type IdempotencyPort interface {
	Execute(context.Context, idempotency.Request, idempotency.Authorize, idempotency.Command) (idempotency.Result, error)
}

type CursorPort interface {
	Encode(cursor.Binding, any) (string, error)
	Decode(string, cursor.Binding, any) error
}

type FilePort interface {
	ReadAuthorized(ctx context.Context, p identity.Principal, fileID string) ([]byte, error)
	StoreText(ctx context.Context, p identity.Principal, filename, text, purpose string) (string, error)
	InUse(ctx context.Context, tenantID, fileID string) (bool, error)
}

type GenerationRequest struct {
	TaskKind        string
	ContextSegments []string
	Brief           string
	MaxOutputTokens int
	ExecutionKey    string
	RemainingBudget int
}

type GenerationResponse struct {
	Summary      string
	Findings     []d.Finding
	Patterns     []d.Pattern
	StyleMetrics *d.StyleMetrics
	Ideas        []d.Idea
	UsedTokens   int
	CallsUsed    int
}

type GenerationPort interface {
	Generate(ctx context.Context, req GenerationRequest) (GenerationResponse, error)
}

type Mutation struct {
	Principal    scope.Principal
	Action       string
	ResourceKind string
	ResourceID   string
	Revision     int64
}
