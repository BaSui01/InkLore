// package application implements the inspiration and deconstruction application services.
package application

import (
	d "InkLore/internal/inspiration/domain"
	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/scope"
	"context"
	"errors"
	"time"
)

type Options struct {
	Repository  d.Repository
	UoW         UnitOfWork
	Scope       ScopePort
	Identity    IdentityPort
	Idempotency IdempotencyPort
	Cursor      CursorPort
	Files       FilePort
	Generation  GenerationPort
	OnMutation  func(context.Context, Mutation) error
	Now         func() time.Time
}

type Service struct {
	o Options
}

func NewService(o Options) (*Service, error) {
	if o.Repository == nil || o.UoW == nil {
		return nil, errors.New("inspiration: repository and UoW required")
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	return &Service{o: o}, nil
}

func (s *Service) now() time.Time {
	return s.o.Now().UTC()
}

func (s *Service) event(ctx context.Context, p scope.Principal, action, kind, id string, rev int64) error {
	if s.o.OnMutation != nil {
		return s.o.OnMutation(ctx, Mutation{
			Principal:    p,
			Action:       action,
			ResourceKind: kind,
			ResourceID:   id,
			Revision:     rev,
		})
	}
	return nil
}

// Error contract helpers matching platform and domain specifications
func missing() error {
	return pe.Contract(404, "platform.resource_not_found", nil)
}

func validationFailed(field string) error {
	return pe.Contract(400, "platform.validation_failed", map[string]any{"field_errors": []map[string]string{{"field": field, "reason": "invalid"}}})
}

func stateError() error {
	return pe.Contract(409, "inspiration.invalid_state", nil)
}

func conflict(rev int64) error {
	return pe.Contract(409, "inspiration.version_conflict", map[string]any{"current_revision": rev})
}

func assetDuplicate(existingID string) error {
	return pe.Contract(409, "inspiration.asset_duplicate", map[string]any{"operation_id": existingID})
}

func cardDuplicate() error {
	return pe.Contract(409, "inspiration.card_duplicate", nil)
}

func restoreConflict() error {
	return pe.Contract(409, "inspiration.restore_conflict", nil)
}

func sourceChanged() error {
	return pe.Contract(409, "inspiration.source_changed", nil)
}

func targetRevisionMismatch() error {
	return pe.Contract(409, "inspiration.target_revision_mismatch", nil)
}

func invalidSource() error {
	return pe.Contract(422, "inspiration.invalid_source", nil)
}

func invalidEvidence() error {
	return pe.Contract(422, "inspiration.invalid_evidence", nil)
}

func analysisTooLarge() error {
	return pe.Contract(422, "inspiration.analysis_too_large", nil)
}

func budgetExhausted() error {
	return pe.Contract(429, "inspiration.budget_exhausted", nil)
}

func cas(actual, want int64) error {
	if want < 1 || want > 9007199254740991 {
		return pe.Contract(400, "platform.validation_failed", nil)
	}
	if actual != want {
		return conflict(actual)
	}
	return nil
}

func (s *Service) AuthorizeCommand(ctx context.Context, p scope.Principal, kind, id string, trash bool) (string, error) {
	if id == "" {
		return "", nil
	}
	if !scope.ValidID(id) {
		return "", pe.Contract(400, "platform.validation_failed", nil)
	}
	return id, nil
}
