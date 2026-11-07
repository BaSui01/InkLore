package identity

import (
	"context"
	"errors"
	"net/mail"
	"strings"

	apperrors "InkLore/internal/platform/errors"
	"InkLore/internal/platform/scope"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func (s *Service) Login(ctx context.Context, input LoginInput) (Session, error) {
	var out Session
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Username = strings.TrimSpace(input.Username)
	identifier := input.Email
	if input.Username != "" {
		identifier = input.Username
	}
	if !s.allowLogin(ctx, tokenHash(input.ClientIP+"\x00"+strings.ToLower(identifier))) {
		return out, apperrors.Contract(429, "auth.rate_limited", map[string]any{"retry_after_seconds": int64(60)})
	}
	validIdentifier := false
	if input.Username != "" {
		validIdentifier = input.Email == "" && input.Username == "root"
	} else {
		address, err := mail.ParseAddress(input.Email)
		validIdentifier = err == nil && address.Address == input.Email && len(input.Email) <= 254
	}
	if !validIdentifier || (input.TenantID != "" && !scope.ValidID(input.TenantID)) || !validAudience(input.Audience) || ValidatePassword(input.Password) != nil {
		return out, invalid()
	}
	db, err := s.manager.DB(ctx)
	if err != nil {
		return out, err
	}
	var candidate userRow
	var result *gorm.DB
	if input.Username != "" {
		result = db.Raw("SELECT id,password_hash FROM platform_users WHERE lower(username)=?", strings.ToLower(input.Username)).Scan(&candidate)
	} else {
		result = db.Raw("SELECT id,password_hash FROM platform_users WHERE lower(email)=?", input.Email).Scan(&candidate)
	}
	if result.Error != nil {
		return out, result.Error
	}
	hash := s.dummyHash
	if result.RowsAffected == 1 {
		if cost, e := bcrypt.Cost([]byte(candidate.PasswordHash)); e == nil && cost == PasswordCost {
			hash = []byte(candidate.PasswordHash)
		}
	}
	passwordErr := bcrypt.CompareHashAndPassword(hash, []byte(input.Password))
	if passwordErr != nil || result.RowsAffected != 1 || string(hash) != candidate.PasswordHash {
		return out, credentials()
	}
	if input.TenantID == "" {
		var tid string
		err = db.Raw("SELECT tenant_id FROM platform_memberships WHERE user_id=? AND status='active' AND CASE WHEN ?='creator' THEN 'creator'=ANY(roles) ELSE roles && ARRAY['operator','auditor']::text[] END ORDER BY created_at ASC,id ASC LIMIT 1", candidate.ID, input.Audience).Scan(&tid).Error
		if err != nil || tid == "" {
			return out, credentials()
		}
		input.TenantID = tid
	}
	err = s.manager.Within(ctx, func(txctx context.Context) error {
		p, user, err := s.member(txctx, Principal{UserID: candidate.ID, TenantID: input.TenantID, Audience: input.Audience})
		if err != nil {
			var e *apperrors.AppError
			if errors.As(err, &e) && (e.HTTPStatus == 401 || e.HTTPStatus == 403) {
				return credentials()
			}
			return err
		}
		if user.PasswordHash != candidate.PasswordHash {
			return credentials()
		}
		out, err = s.newSession(txctx, p)
		if err != nil {
			return err
		}
		p = out.Principal
		return s.event(txctx, p, "auth.login", "succeeded")
	})
	if err != nil {
		return Session{}, err
	}
	return out, nil
}
