package identity

import (
	"context"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"time"
)

// newSession is the single credential issuance path for password and email login.
// The caller has locked and verified the current account and membership.
func (s *Service) newSession(ctx context.Context, p Principal) (Session, error) {
	tx, err := s.manager.Require(ctx)
	if err != nil {
		return Session{}, err
	}
	tx = tx.Session(&gorm.Session{Logger: logger.Discard})
	p.SessionID = uuid.NewString()
	now := time.Now().UTC().Truncate(time.Microsecond)
	expiry := now.Add(SessionLifetime)
	csrf := s.csrf(p.SessionID, p.Audience, now)
	refresh, err := randomToken()
	if err != nil {
		return Session{}, err
	}
	if err = tx.Exec("INSERT INTO platform_sessions (id,tenant_id,user_id,audience,csrf_hash,membership_revision,expires_at,created_at,updated_at,last_seen_at) VALUES (?,?,?,?,?,?,?,?,?,?)", p.SessionID, p.TenantID, p.UserID, p.Audience, tokenHash(csrf), p.MembershipRevision, expiry, now, now, now).Error; err != nil {
		return Session{}, err
	}
	if err = tx.Exec("INSERT INTO platform_refresh_tokens (id,session_id,token_hash,expires_at,created_at) VALUES (?,?,?,?,?)", uuid.NewString(), p.SessionID, tokenHash(refresh), expiry, now).Error; err != nil {
		return Session{}, err
	}
	access, err := s.sign(p, now)
	if err != nil {
		return Session{}, err
	}
	return Session{AccessToken: access, ExpiresIn: 900, CSRFToken: csrf, Principal: p, RefreshToken: refresh, RefreshExpiresAt: expiry}, nil
}
