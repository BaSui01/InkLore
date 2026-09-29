package idempotency

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"regexp"

	apperrors "InkLore/internal/platform/errors"
	"InkLore/internal/platform/response"
	"InkLore/internal/platform/scope"
	"InkLore/internal/platform/uow"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var tracePattern = regexp.MustCompile("^[0-9a-f]{32}$")

func validation() error { return apperrors.Contract(400, "platform.validation_failed", nil) }
func internal() error   { return apperrors.Contract(500, "platform.internal_error", nil) }
func inProgress() error {
	return apperrors.Contract(409, "platform.operation_in_progress", map[string]any{"retry_after_seconds": int64(1)})
}

// Execute owns the root UoW. Cached deterministic errors return Result,nil; errors
// returned separately mean no result was committed (or commit outcome is unknown).
func (e *Executor) Execute(ctx context.Context, req Request, authorize Authorize, command Command) (Result, error) {
	return e.execute(ctx, req, authorize, command, false)
}

// ExecuteReadConsistent keeps the preview and its idempotency record in one
// repeatable-read transaction; it does not grant permission to apply the preview.
func (e *Executor) ExecuteReadConsistent(ctx context.Context, req Request, authorize Authorize, command Command) (Result, error) {
	return e.execute(ctx, req, authorize, command, true)
}

func (e *Executor) execute(ctx context.Context, req Request, authorize Authorize, command Command, consistent bool) (Result, error) {
	if e == nil || e.manager == nil || ctx == nil || authorize == nil || command == nil {
		return Result{}, internal()
	}
	if uow.InTransaction(ctx) {
		return Result{}, errors.New("idempotency: Execute requires a nontransaction context")
	}
	if !scope.ValidID(req.TenantID) || !scope.ValidID(req.ActorID) || !scope.ValidID(req.Key) || !scope.ValidID(req.RequestID) || !tracePattern.MatchString(req.TraceID) {
		return Result{}, validation()
	}
	switch req.Method {
	case "POST", "PUT", "PATCH", "DELETE":
	default:
		return Result{}, validation()
	}
	canonicalPath, err := CanonicalPath(req.Path)
	if err != nil {
		return Result{}, validation()
	}
	req.Path = canonicalPath
	// These token-bearing endpoints use their own session transactions. Password
	// changes are deliberately NOT exempt under the frozen baseline.
	switch req.Path {
	case "/v1/auth/login", "/v1/auth/refresh", "/v1/auth/logout", "/v1/auth/ws-ticket":
		return Result{}, validation()
	}
	if err = authorize(ctx); err != nil {
		return Result{}, err
	}
	raw := req.Body
	if len(raw) == 0 {
		raw = json.RawMessage("null")
	}
	body, err := CanonicalJSON(raw)
	if err != nil {
		return Result{}, validation()
	}
	query := req.Query
	if query == nil {
		query = map[string][]string{}
	}
	queryJSON, err := json.Marshal(query)
	if err != nil {
		return Result{}, validation()
	}
	queryJSON, err = CanonicalJSON(queryJSON)
	if err != nil {
		return Result{}, validation()
	}
	requestHash := digest(e.secret, body, queryJSON)
	logicalKey, _ := json.Marshal([]string{req.TenantID, req.ActorID, req.Method, req.Path, req.Key})
	lockHash := sha256.Sum256(logicalKey)
	lockID := int64(binary.BigEndian.Uint64(lockHash[:8]))
	var result Result
	within := e.manager.Within
	if consistent {
		within = e.manager.WithinRepeatableRead
	}
	err = within(ctx, func(txctx context.Context) error {
		db, err := e.manager.Require(txctx)
		if err != nil {
			return err
		}
		// Response JSON may contain private data. Never let GORM interpolate it
		// into SQL logs, regardless of the caller's configured logger.
		db = db.Session(&gorm.Session{Logger: logger.Discard})
		var acquired bool
		if err = db.Raw("SELECT pg_try_advisory_xact_lock(?)", lockID).Scan(&acquired).Error; err != nil {
			return err
		}
		if !acquired {
			return inProgress()
		}
		if err = authorize(txctx); err != nil {
			return err
		}
		id, cached, err := claim(db, req, requestHash)
		if err != nil {
			return err
		}
		if cached != nil {
			result = *cached
			return nil
		}
		var outcome CommandOutcome
		var public *apperrors.AppError
		boundaryErr := e.manager.Recoverable(txctx, func(commandctx context.Context) error {
			var commandErr error
			outcome, commandErr = command(commandctx)
			if commandErr != nil {
				// Error returns always roll back; explicit invalidation uses PublicError.
				outcome.Commit = false
				return commandErr
			}
			public = outcome.PublicError
			if public != nil {
				if !apperrors.IsContractStatus(public.Code, public.HTTPStatus) {
					return internal()
				}
				if public.HTTPStatus != 409 && public.HTTPStatus != 422 {
					return public
				}
				if outcome.Commit && public.HTTPStatus != 409 {
					return internal()
				}
				if !outcome.Commit {
					return public
				}
			} else if !outcome.Commit {
				return internal()
			}
			return nil
		})
		if boundaryErr != nil {
			if errors.Is(boundaryErr, context.Canceled) || errors.Is(boundaryErr, context.DeadlineExceeded) {
				return boundaryErr
			}
			if !errors.As(boundaryErr, &public) || public == nil || !apperrors.IsContractStatus(public.Code, public.HTTPStatus) || (public.HTTPStatus != 409 && public.HTTPStatus != 422) {
				return boundaryErr
			}
			outcome.Commit = false
		}
		if public != nil {
			status, envelope := response.FailureBody(public, req.RequestID, req.TraceID)
			encoded, err := json.Marshal(envelope)
			if err != nil {
				return internal()
			}
			result = Result{Status: status, Body: encoded}
		} else {
			if !outcome.Commit || outcome.Result.Replay {
				return internal()
			}
			switch outcome.Result.Status {
			case 200, 201, 202:
			default:
				return internal()
			}
			data := outcome.Result.Body
			if len(data) == 0 {
				data = json.RawMessage("null")
			}
			if !json.Valid(data) {
				return internal()
			}
			encoded, err := json.Marshal(response.SuccessEnvelope{Code: "ok", Data: data, RequestID: req.RequestID, TraceID: req.TraceID})
			if err != nil {
				return internal()
			}
			result = Result{Status: outcome.Result.Status, Body: encoded}
		}
		return complete(db, id, requestHash, result)
	})
	if err != nil {
		return Result{}, err
	}
	return result, nil
}
