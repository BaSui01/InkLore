package identity

import (
	"context"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	apperrors "InkLore/internal/platform/errors"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// RegisterCreator registers a new independent creator, creates their default workspace,
// and returns an active creator session.
func (s *Service) RegisterCreator(ctx context.Context, input RegisterInput) (Session, error) {
	var out Session
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.PenName = strings.TrimSpace(input.PenName)

	if !s.allowLogin(ctx, tokenHash(input.ClientIP+"\x00"+input.Email)) {
		return out, apperrors.Contract(429, "auth.rate_limited", map[string]any{"retry_after_seconds": int64(60)})
	}

	address, err := mail.ParseAddress(input.Email)
	if err != nil || address.Address != input.Email || len(input.Email) > 254 {
		return out, invalid()
	}
	if utf8.RuneCountInString(input.PenName) < 1 || utf8.RuneCountInString(input.PenName) > 60 {
		return out, invalid()
	}
	if ValidatePassword(input.Password) != nil {
		return out, invalid()
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), PasswordCost)
	if err != nil {
		return out, err
	}

	err = s.manager.Within(ctx, func(txctx context.Context) error {
		tx, err := s.manager.Require(txctx)
		if err != nil {
			return err
		}

		// Check uniqueness
		var exists bool
		if err = tx.Raw("SELECT EXISTS(SELECT 1 FROM platform_users WHERE lower(email)=?)", input.Email).Scan(&exists).Error; err != nil {
			return err
		}
		if exists {
			return apperrors.Contract(409, "auth.account_unavailable", nil)
		}

		tenantID := uuid.NewString()
		userID := uuid.NewString()
		membershipID := uuid.NewString()
		sessionID := uuid.NewString()
		now := time.Now().UTC().Truncate(time.Microsecond)
		expiry := now.Add(SessionLifetime)

		tenantName := input.PenName + "的创作室"
		if err = tx.Exec("INSERT INTO platform_tenants (id, name, status, revision, created_at, updated_at) VALUES (?, ?, 'active', 1, ?, ?)", tenantID, tenantName, now, now).Error; err != nil {
			return err
		}

		if err = tx.Exec("INSERT INTO platform_users (id, email, display_name, password_hash, status, revision, created_at, updated_at) VALUES (?, ?, ?, ?, 'active', 1, ?, ?)", userID, input.Email, input.PenName, string(hash), now, now).Error; err != nil {
			return err
		}

		if err = tx.Exec("INSERT INTO platform_memberships (id, tenant_id, user_id, roles, status, revision, created_at, updated_at) VALUES (?, ?, ?, ARRAY['creator']::text[], 'active', 1, ?, ?)", membershipID, tenantID, userID, now, now).Error; err != nil {
			return err
		}
		if err = ProvisionGlobalAdmins(txctx, s.manager, tenantID); err != nil {
			return err
		}

		csrf := s.csrf(sessionID, "creator", now)
		refresh, err := randomToken()
		if err != nil {
			return err
		}

		if err = tx.Exec("INSERT INTO platform_sessions (id, tenant_id, user_id, audience, csrf_hash, membership_revision, expires_at, created_at, updated_at, last_seen_at) VALUES (?, ?, ?, 'creator', ?, 1, ?, ?, ?, ?)", sessionID, tenantID, userID, tokenHash(csrf), expiry, now, now, now).Error; err != nil {
			return err
		}

		if err = tx.Exec("INSERT INTO platform_refresh_tokens (id, session_id, token_hash, expires_at, created_at) VALUES (?, ?, ?, ?, ?)", uuid.NewString(), sessionID, tokenHash(refresh), expiry, now).Error; err != nil {
			return err
		}

		p := Principal{
			UserID:             userID,
			TenantID:           tenantID,
			DisplayName:        input.PenName,
			Roles:              []string{"creator"},
			GlobalRoles:        []string{},
			Audience:           "creator",
			SessionID:          sessionID,
			UserRevision:       1,
			MembershipRevision: 1,
		}

		access, err := s.sign(p, now)
		if err != nil {
			return err
		}

		out = Session{
			AccessToken:      access,
			ExpiresIn:        900,
			CSRFToken:        csrf,
			Principal:        p,
			RefreshToken:     refresh,
			RefreshExpiresAt: expiry,
		}

		return s.event(txctx, p, "auth.register", "succeeded")
	})

	if err != nil {
		return Session{}, err
	}
	return out, nil
}
