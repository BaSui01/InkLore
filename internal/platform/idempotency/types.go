// Package idempotency executes local commands and replay records atomically.
package idempotency

import (
	apperrors "InkLore/internal/platform/errors"
	"InkLore/internal/platform/uow"
	"context"
	"encoding/json"
	"errors"
	"strconv"
)

const ReplayHeader = "Idempotent-Replay"

type Request struct {
	TenantID, ActorID, Method, Path, Key string
	Body                                 json.RawMessage
	// Query contains semantic parameters only, including DELETE expected_revision.
	// The transport removes nonsemantic parameters; value order is preserved.
	Query              map[string][]string
	RequestID, TraceID string
}

type Result struct {
	Status int
	// Body is data in successful CommandOutcome, and the full envelope from Execute.
	Body   json.RawMessage
	Replay bool
}

type CommandOutcome struct {
	// Commit permits explicit invalidation writes with a registered public 409.
	// Successful commands also set Commit. Ordinary deterministic failures use false.
	Commit      bool
	Result      Result
	PublicError *apperrors.AppError
}

// Authorize checks current identity and scope, retaining locks in a transaction.
// It must not check expected_revision: a replay must not run CAS again.
type Authorize func(context.Context) error

// Command uses txctx synchronously without network side effects. Recoverable
// isolates ordinary failures, including rollback-only failures from nested Within.
type Command func(context.Context) (CommandOutcome, error)

type Executor struct {
	manager *uow.Manager
	secret  []byte
}

// New copies a dedicated environment-injected key. The composition root ensures
// independence from JWT, cursor and CSRF keys; this package checks its size.
func New(manager *uow.Manager, secret []byte) (*Executor, error) {
	if manager == nil || len(secret) < 32 {
		return nil, errors.New("idempotency: manager and independent HMAC key of at least 32 bytes required")
	}
	return &Executor{manager: manager, secret: append([]byte(nil), secret...)}, nil
}

// Headers leaves fresh request/trace response headers to the transport.
func (r Result) Headers() map[string]string {
	headers := map[string]string{}
	if r.Replay {
		headers[ReplayHeader] = "true"
	}
	var body struct {
		Code    string         `json:"code"`
		Details map[string]any `json:"details"`
	}
	if json.Unmarshal(r.Body, &body) == nil {
		if seconds, ok := apperrors.SafeDetails(body.Details)["retry_after_seconds"].(int64); ok {
			headers["Retry-After"] = strconv.FormatInt(seconds, 10)
		}
		if body.Code == "platform.operation_in_progress" {
			headers["Retry-After"] = "1"
		}
	}
	return headers
}
