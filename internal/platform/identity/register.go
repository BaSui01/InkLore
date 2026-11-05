package identity

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/uow"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// RegisterCreator preserves password registration. Email verification is granted
// only by atomically consuming a register-purpose challenge when one is supplied.
func (s *Service) RegisterCreator(ctx context.Context, input RegisterInput) (Session, error) {
	if uow.InTransaction(ctx) {
		return Session{}, errors.New("email registration requires its own transaction")
	}
	proof := EmailLoginInput{Email: input.Email, ChallengeID: input.ChallengeID, Code: input.Code, Audience: "creator", ClientIP: input.ClientIP}
	verifyEmail := input.ChallengeID != "" || input.Code != ""
	if verifyEmail {
		if err := s.validateEmailLogin(ctx, &proof); err != nil {
			return Session{}, err
		}
	} else {
		var valid bool
		proof.Email, valid = normalizedEmail(input.Email)
		if !valid {
			return Session{}, invalid()
		}
		if !s.allowLogin(ctx, tokenHash(input.ClientIP+"\x00"+proof.Email)) {
			return Session{}, emailRateLimit()
		}
	}
	input.Email = proof.Email
	input.PenName = strings.TrimSpace(input.PenName)
	if utf8.RuneCountInString(input.PenName) < 1 || utf8.RuneCountInString(input.PenName) > 60 {
		return Session{}, invalid()
	}
	hash, err := HashPassword(input.Password)
	if err != nil {
		return Session{}, err
	}
	var out Session
	accepted := false
	err = s.manager.Within(ctx, func(txctx context.Context) error {
		var row challengeRow
		if verifyEmail {
			var valid bool
			var err error
			row, valid, err = s.checkEmailChallenge(txctx, proof, "register")
			if err != nil || !valid {
				return err
			}
		}
		db, err := s.manager.Require(txctx)
		if err != nil {
			return err
		}
		db = db.Session(&gorm.Session{Logger: logger.Discard})
		if err = db.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,98))", "registration:"+tokenHash(input.Email)).Error; err != nil {
			return err
		}
		if verifyEmail {
			if err = s.consumeEmailChallenge(txctx, row); err != nil {
				return err
			}
		}
		var exists bool
		if err = db.Raw("SELECT EXISTS(SELECT 1 FROM platform_users WHERE lower(email)=?)", input.Email).Scan(&exists).Error; err != nil {
			return err
		}
		if exists {
			if verifyEmail {
				return nil
			}
			return pe.Contract(409, "auth.account_unavailable", nil)
		}
		tenantID, userID, membershipID := uuid.NewString(), uuid.NewString(), uuid.NewString()
		now := time.Now().UTC().Truncate(time.Microsecond)
		var verifiedAt *time.Time
		if verifyEmail {
			verifiedAt = &now
		}
		if err = db.Exec("INSERT INTO platform_tenants(id,name,status,revision,created_at,updated_at) VALUES(?,?,'active',1,?,?)", tenantID, input.PenName+"的创作室", now, now).Error; err != nil {
			return err
		}
		if err = db.Exec("INSERT INTO platform_users(id,email,display_name,password_hash,status,revision,email_verified_at,created_at,updated_at) VALUES(?,?,?,?,'active',1,?,?,?)", userID, input.Email, input.PenName, hash, verifiedAt, now, now).Error; err != nil {
			return err
		}
		if err = db.Exec("INSERT INTO platform_memberships(id,tenant_id,user_id,roles,status,revision,created_at,updated_at) VALUES(?,?,?,ARRAY['creator']::text[],'active',1,?,?)", membershipID, tenantID, userID, now, now).Error; err != nil {
			return err
		}
		if err = ProvisionGlobalAdmins(txctx, s.manager, tenantID); err != nil {
			return err
		}
		p := Principal{UserID: userID, TenantID: tenantID, DisplayName: input.PenName, Roles: []string{"creator"}, GlobalRoles: []string{}, Audience: "creator", UserRevision: 1, MembershipRevision: 1}
		out, err = s.newSession(txctx, p)
		if err != nil {
			return err
		}
		if err = s.event(txctx, out.Principal, "auth.register", "succeeded"); err != nil {
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
