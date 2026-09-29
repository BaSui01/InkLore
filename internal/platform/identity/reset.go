package identity

import (
	"context"
	"net/mail"
	"strings"
	"sync"
	"time"

	apperrors "InkLore/internal/platform/errors"

	"golang.org/x/crypto/bcrypt"
)

type resetEntry struct {
	userID    string
	email     string
	expiresAt time.Time
	claimed   bool
}

type resetStore struct {
	mu     sync.Mutex
	tokens map[string]resetEntry
}

func newResetStore() *resetStore {
	return &resetStore{
		tokens: make(map[string]resetEntry),
	}
}

func (s *resetStore) set(tokenHash, userID, email string, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	// Clean up expired tokens
	for k, v := range s.tokens {
		if now.After(v.expiresAt) {
			delete(s.tokens, k)
		}
	}
	s.tokens[tokenHash] = resetEntry{
		userID:    userID,
		email:     email,
		expiresAt: now.Add(ttl),
	}
}

func (s *resetStore) claim(tokenHash string) (resetEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.tokens[tokenHash]
	if !ok || time.Now().After(entry.expiresAt) {
		delete(s.tokens, tokenHash)
		return resetEntry{}, false
	}
	if entry.claimed {
		return resetEntry{}, false
	}
	entry.claimed = true
	s.tokens[tokenHash] = entry
	return entry, true
}

// finish consumes only committed resets; a rolled back transaction permits retry.
func (s *resetStore) finish(hash string, committed bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.tokens[hash]
	if !ok {
		return
	}
	if committed {
		delete(s.tokens, hash)
		return
	}
	entry.claimed = false
	s.tokens[hash] = entry
}

// ForgotPassword generates a password reset token for the given email if the user exists.
// Returns the reset token (for dev/simulation/email sender) and an error if validation fails.
func (s *Service) ForgotPassword(ctx context.Context, email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || len(email) > 254 {
		return "", invalid()
	}

	db, err := s.manager.DB(ctx)
	if err != nil {
		return "", err
	}

	var user struct {
		ID     string
		Status string
	}
	q := db.Raw("SELECT id, status FROM platform_users WHERE lower(email)=?", email).Scan(&user)
	if q.Error != nil {
		return "", q.Error
	}
	if q.RowsAffected != 1 || user.Status != "active" {
		// Return empty token without error to prevent user enumeration
		return "", nil
	}

	token, err := randomToken()
	if err != nil {
		return "", err
	}

	s.resets.set(tokenHash(token), user.ID, email, 15*time.Minute)

	return token, nil
}

// ResetPassword resets the user's password using a valid reset token and revokes all active sessions.
func (s *Service) ResetPassword(ctx context.Context, token, newPassword string) error {
	if !validToken(token) {
		return apperrors.Contract(400, "auth.invalid_token", nil)
	}
	if ValidatePassword(newPassword) != nil {
		return invalid()
	}

	if s.resets == nil {
		return apperrors.Contract(400, "auth.invalid_token", nil)
	}
	entry, ok := s.resets.claim(tokenHash(token))
	if !ok {
		return apperrors.Contract(400, "auth.invalid_token", nil)
	}

	committed := false
	defer func() { s.resets.finish(tokenHash(token), committed) }()
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), PasswordCost)
	if err != nil {
		return err
	}

	err = s.manager.Within(ctx, func(txctx context.Context) error {
		tx, err := s.manager.Require(txctx)
		if err != nil {
			return err
		}

		// Update password
		result := tx.Exec("UPDATE platform_users SET password_hash=?, revision=revision+1, updated_at=clock_timestamp() WHERE id=? AND status='active'", string(hash), entry.userID)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return apperrors.Contract(400, "auth.invalid_token", nil)
		}

		// Revoke all active sessions for this user
		if err = tx.Exec("UPDATE platform_sessions SET revoked_at=clock_timestamp(), updated_at=clock_timestamp(), revision=revision+1 WHERE user_id=? AND revoked_at IS NULL", entry.userID).Error; err != nil {
			return err
		}

		var tenantID string
		if err := tx.Raw("SELECT tenant_id FROM platform_memberships WHERE user_id=? ORDER BY created_at, id LIMIT 1", entry.userID).Scan(&tenantID).Error; err != nil {
			return err
		}

		return s.event(txctx, Principal{UserID: entry.userID, TenantID: tenantID, Audience: "creator"}, "auth.password_reset", "succeeded")
	})
	committed = err == nil
	return err
}
