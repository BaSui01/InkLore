package identity

import (
	"context"
	"errors"

	"InkLore/internal/platform/uow"
)

func (s *Service) LoginEmail(ctx context.Context, input EmailLoginInput) (Session, error) {
	if uow.InTransaction(ctx) {
		return Session{}, errors.New("email login requires its own transaction")
	}
	if err := s.validateEmailLogin(ctx, &input); err != nil {
		return Session{}, err
	}
	var out Session
	accepted := false
	err := s.manager.Within(ctx, func(txctx context.Context) error {
		row, valid, err := s.checkEmailChallenge(txctx, input, "login")
		if err != nil || !valid {
			return err
		}
		p, valid, err := s.emailMember(txctx, row, input.Email)
		if err != nil || !valid {
			return err
		}
		db, err := s.manager.Require(txctx)
		if err != nil {
			return err
		}
		// Password-only registration can leave an unverified address controlled
		// by somebody else. OTP login must not silently transfer ownership while
		// preserving that registrant's password or existing sessions. The real
		// mailbox owner can instead use ResetPassword, which rotates the password
		// and revokes all sessions atomically before marking the email verified.
		var verified bool
		if err = db.Raw("SELECT email_verified_at IS NOT NULL FROM platform_users WHERE id=?", p.UserID).Scan(&verified).Error; err != nil {
			return err
		}
		if !verified {
			return nil
		}
		if err = s.consumeEmailChallenge(txctx, row); err != nil {
			return err
		}
		out, err = s.newSession(txctx, p)
		if err != nil {
			return err
		}
		if err = s.event(txctx, out.Principal, "auth.login", "succeeded"); err != nil {
			return err
		}
		accepted = true
		return nil
	})
	if err != nil {
		return Session{}, err
	}
	if !accepted {
		return Session{}, credentials()
	}
	return out, nil
}

// VerifiedEmail exposes only the authenticated caller's previously proven address
// for the administrator's test-delivery action. It never accepts an arbitrary to.
func (s *Service) VerifiedEmail(ctx context.Context, p Principal) (string, error) {
	var email string
	err := s.manager.Within(ctx, func(txctx context.Context) error {
		current, err := s.Authorize(txctx, p)
		if err != nil {
			return err
		}
		db, err := s.manager.Require(txctx)
		if err != nil {
			return err
		}
		q := db.Raw("SELECT email FROM platform_users WHERE id=? AND email_verified_at IS NOT NULL", current.UserID).Scan(&email)
		if q.Error != nil {
			return q.Error
		}
		if q.RowsAffected != 1 {
			return forbidden()
		}
		return nil
	})
	return email, err
}
