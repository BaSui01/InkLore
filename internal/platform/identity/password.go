package identity

import (
	"context"

	apperrors "InkLore/internal/platform/errors"
	"InkLore/internal/platform/scope"
	"InkLore/internal/platform/uow"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ChangePasswordInput uses the account revision from Principal.UserRevision,
// independently of the active tenant membership's revision.
type ChangePasswordInput struct {
	CurrentPassword  string `json:"current_password"`
	NewPassword      string `json:"new_password"`
	ExpectedRevision int64  `json:"expected_revision"`
}

// AuthorizePasswordChange is the idempotency authorization callback. Call it
// before ValidateCSRF or any ordinary identity authorization inside the command
// transaction: upgrading a user share lock could deadlock concurrent changes.
// The outside precheck releases its short read transaction before returning.
// This callback deliberately checks neither password nor expected_revision, so
// an authorized new session may replay a previously committed command result.
func (s *Service) AuthorizePasswordChange(ctx context.Context, p Principal) error {
	if !uow.InTransaction(ctx) {
		_, err := s.Authorize(ctx, p)
		return err
	}
	if !scope.ValidID(p.TenantID) || !scope.ValidID(p.UserID) || !scope.ValidID(p.SessionID) || !validAudience(p.Audience) {
		return unauthenticated()
	}
	db, err := s.manager.Require(ctx)
	if err != nil {
		return err
	}
	var tenantID string
	q := db.Raw("SELECT id FROM platform_tenants WHERE id=? FOR SHARE", p.TenantID).Scan(&tenantID)
	if q.Error != nil {
		return q.Error
	}
	if q.RowsAffected != 1 {
		return unauthenticated()
	}
	var userID string
	q = db.Raw("SELECT id FROM platform_users WHERE id=? FOR UPDATE", p.UserID).Scan(&userID)
	if q.Error != nil {
		return q.Error
	}
	if q.RowsAffected != 1 {
		return unauthenticated()
	}
	// The user update lock serializes all sessions of this account across tenants.
	// Authorize now safely adds membership and session share locks, in that order.
	_, err = s.Authorize(ctx, p)
	return err
}

// ChangePassword joins the idempotency command UoW or owns a standalone UoW.
// The transport owns idempotency and CSRF/Origin checks. Never authorize cached
// results using password or revision checks: only AuthorizePasswordChange is a
// valid pre-cache gate. No password, hash, or token is included in the result.
func (s *Service) ChangePassword(ctx context.Context, p Principal, input ChangePasswordInput) (LogoutResult, error) {
	if input.ExpectedRevision < 1 || input.ExpectedRevision > scope.MaxRevision {
		return LogoutResult{}, invalid()
	}
	if err := ValidatePassword(input.NewPassword); err != nil {
		return LogoutResult{}, err
	}
	var out LogoutResult
	err := s.manager.Within(ctx, func(txctx context.Context) error {
		if err := s.AuthorizePasswordChange(txctx, p); err != nil {
			return err
		}
		db, err := s.manager.Require(txctx)
		if err != nil {
			return err
		}
		// Credential mutations remain silent even under an accidentally verbose DB logger.
		db = db.Session(&gorm.Session{Logger: logger.Discard})
		var user userRow
		q := db.Raw("SELECT id,password_hash,revision FROM platform_users WHERE id=?", p.UserID).Scan(&user)
		if q.Error != nil {
			return q.Error
		}
		if q.RowsAffected != 1 {
			return unauthenticated()
		}
		if len(input.CurrentPassword) > 72 || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.CurrentPassword)) != nil {
			return credentials()
		}
		if user.Revision != input.ExpectedRevision || user.Revision >= scope.MaxRevision {
			return passwordConflict(user.Revision)
		}
		hash, err := HashPassword(input.NewPassword)
		if err != nil {
			return err
		}
		q = db.Exec("UPDATE platform_users SET password_hash=?,revision=revision+1,updated_at=clock_timestamp() WHERE id=? AND revision=?", hash, p.UserID, input.ExpectedRevision)
		if q.Error != nil {
			return q.Error
		}
		if q.RowsAffected != 1 {
			return passwordConflict(user.Revision)
		}
		// No tenant/audience filter: changing the global password revokes every session
		// of the account. Refresh rows remain as immutable replay evidence.
		q = db.Exec("UPDATE platform_sessions SET revoked_at=clock_timestamp(),revision=revision+1,updated_at=clock_timestamp() WHERE user_id=? AND revoked_at IS NULL", p.UserID)
		if q.Error != nil {
			return q.Error
		}
		if s.onEvent != nil {
			p.UserRevision = user.Revision + 1
			if err = s.onEvent(txctx, Event{Principal: p, ActorID: p.UserID, TenantID: p.TenantID, Audience: p.Audience, ResourceID: p.UserID, Action: "auth.password_changed", Outcome: "succeeded"}); err != nil {
				return err
			}
		}
		out.Revoked = true
		return nil
	})
	if err != nil {
		return LogoutResult{}, err
	}
	return out, nil
}
func passwordConflict(revision int64) error {
	return apperrors.Contract(409, "platform.version_conflict", map[string]any{"current_revision": revision})
}
