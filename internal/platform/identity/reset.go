package identity

import (
	"context"
	"errors"

	"InkLore/internal/platform/scope"
	"InkLore/internal/platform/uow"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ResetPassword consumes the persistent, purpose-bound email proof in the same
// transaction as password rotation and revocation of every existing session.
func (s *Service) ResetPassword(ctx context.Context, input ResetPasswordInput) error {
	if uow.InTransaction(ctx) {
		return errors.New("email reset requires its own transaction")
	}
	if err := s.validateEmailLogin(ctx, &input.EmailLoginInput); err != nil {
		return err
	}
	// Count and verify the proof before bcrypt. Invalid random challenges must
	// never force cost-12 password hashing, even when the caller rotates emails.
	var proofValid bool
	err := s.manager.Within(ctx, func(txctx context.Context) error {
		_, valid, err := s.checkEmailChallenge(txctx, input.EmailLoginInput, "reset")
		proofValid = valid
		return err
	})
	if err != nil {
		return err
	}
	if !proofValid {
		return credentials()
	}
	hash, err := HashPassword(input.NewPassword)
	if err != nil {
		return err
	}
	accepted := false
	err = s.manager.Within(ctx, func(txctx context.Context) error {
		row, valid, err := s.verifyEmailChallenge(txctx, input.EmailLoginInput, "reset")
		if err != nil || !valid {
			return err
		}
		p, valid, err := s.emailMember(txctx, row, input.Email)
		if err != nil || !valid {
			return err
		}
		if p.UserRevision >= scope.MaxRevision {
			return passwordConflict(p.UserRevision)
		}
		if err = s.consumeEmailChallenge(txctx, row); err != nil {
			return err
		}
		db, err := s.manager.Require(txctx)
		if err != nil {
			return err
		}
		db = db.Session(&gorm.Session{Logger: logger.Discard})
		q := db.Exec("UPDATE platform_users SET password_hash=?,email_verified_at=COALESCE(email_verified_at,clock_timestamp()),revision=revision+1,updated_at=clock_timestamp() WHERE id=? AND revision=?", hash, p.UserID, p.UserRevision)
		if q.Error != nil {
			return q.Error
		}
		if q.RowsAffected != 1 {
			return passwordConflict(p.UserRevision)
		}
		if err = db.Exec("UPDATE platform_sessions SET revoked_at=clock_timestamp(),updated_at=clock_timestamp(),revision=revision+1 WHERE user_id=? AND revoked_at IS NULL", p.UserID).Error; err != nil {
			return err
		}
		if err = s.event(txctx, p, "auth.password_reset", "succeeded"); err != nil {
			return err
		}
		accepted = true
		return nil
	})
	if err != nil {
		return err
	}
	if !accepted {
		return credentials()
	}
	return nil
}
