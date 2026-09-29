package identity

import (
	"context"
	"errors"
	"time"

	apperrors "InkLore/internal/platform/errors"
	"InkLore/internal/platform/uow"

	"github.com/google/uuid"
)

type refreshRow struct {
	ID, SessionID string
	UsedAt        *time.Time
	ExpiresAt     time.Time
}

// lookupRefresh locates immutable IDs only. Security decisions happen after locks.
func (s *Service) lookupRefresh(ctx context.Context, refresh, audience string) (sessionRow, bool, error) {
	var row sessionRow
	if !validAudience(audience) {
		return row, false, invalid()
	}
	if !validToken(refresh) {
		return row, false, nil
	}
	db, err := s.manager.DB(ctx)
	if err != nil {
		return row, false, err
	}
	q := db.Raw("SELECT s.* FROM platform_refresh_tokens r JOIN platform_sessions s ON s.id=r.session_id WHERE r.token_hash=?", tokenHash(refresh)).Scan(&row)
	if q.Error != nil {
		return row, false, q.Error
	}
	if q.RowsAffected != 1 {
		return row, false, nil
	}
	if row.Audience != audience {
		return row, false, forbidden()
	}
	return row, true, nil
}

// lockSession preserves revocation/replay handling even for a disabled account.
// Foreign keys guarantee the rows still exist; all status checks follow locks.
func (s *Service) lockSession(ctx context.Context, row sessionRow) (sessionRow, error) {
	db, err := s.manager.Require(ctx)
	if err != nil {
		return sessionRow{}, err
	}
	for _, query := range []struct {
		SQL  string
		Args []any
	}{
		{"SELECT id FROM platform_tenants WHERE id=? FOR SHARE", []any{row.TenantID}},
		{"SELECT id FROM platform_users WHERE id=? FOR SHARE", []any{row.UserID}},
		{"SELECT id FROM platform_memberships WHERE tenant_id=? AND user_id=? FOR SHARE", []any{row.TenantID, row.UserID}},
	} {
		var id string
		q := db.Raw(query.SQL, query.Args...).Scan(&id)
		if q.Error != nil {
			return sessionRow{}, q.Error
		}
		if q.RowsAffected != 1 {
			return sessionRow{}, unauthenticated()
		}
	}
	var locked sessionRow
	q := db.Raw("SELECT * FROM platform_sessions WHERE id=? AND tenant_id=? AND user_id=? FOR UPDATE", row.ID, row.TenantID, row.UserID).Scan(&locked)
	if q.Error != nil {
		return sessionRow{}, q.Error
	}
	if q.RowsAffected != 1 {
		return sessionRow{}, unauthenticated()
	}
	return locked, nil
}
func principalOf(row sessionRow) Principal {
	return Principal{UserID: row.UserID, TenantID: row.TenantID, SessionID: row.ID, Audience: row.Audience}
}
func (s *Service) SessionCSRF(ctx context.Context, refresh, audience string) (CSRF, error) {
	var out CSRF
	err := s.manager.Within(ctx, func(txctx context.Context) error {
		row, found, err := s.lookupRefresh(txctx, refresh, audience)
		if err != nil {
			return err
		}
		if !found {
			return unauthenticated()
		}
		_, row, err = s.live(txctx, principalOf(row), false)
		if err != nil {
			return err
		}
		db, err := s.manager.Require(txctx)
		if err != nil {
			return err
		}
		var token refreshRow
		q := db.Raw("SELECT id,session_id,used_at,expires_at FROM platform_refresh_tokens WHERE token_hash=? AND session_id=? FOR SHARE", tokenHash(refresh), row.ID).Scan(&token)
		if q.Error != nil {
			return q.Error
		}
		if q.RowsAffected != 1 || token.UsedAt != nil || !token.ExpiresAt.After(time.Now()) {
			return unauthenticated()
		}
		out.CSRFToken = s.csrf(row.ID, row.Audience, row.CreatedAt)
		return s.checkCSRF(row, out.CSRFToken)
	})
	if err != nil {
		return CSRF{}, err
	}
	return out, nil
}

// Refresh must own the root transaction: a public replay error is returned only
// after revocation commits. Wrapping this method in another UoW is rejected.
func (s *Service) Refresh(ctx context.Context, refresh, audience, csrf string) (Session, error) {
	if uow.InTransaction(ctx) {
		return Session{}, errors.New("identity: refresh requires a root transaction")
	}
	var out Session
	var publicErr error
	err := s.manager.Within(ctx, func(txctx context.Context) error {
		row, found, err := s.lookupRefresh(txctx, refresh, audience)
		if err != nil {
			return err
		}
		if !found {
			return unauthenticated()
		}
		row, err = s.lockSession(txctx, row)
		if err != nil {
			return err
		}
		if err = s.checkCSRF(row, csrf); err != nil {
			return err
		}
		db, err := s.manager.Require(txctx)
		if err != nil {
			return err
		}
		var old refreshRow
		q := db.Raw("SELECT id,session_id,used_at,expires_at FROM platform_refresh_tokens WHERE token_hash=? AND session_id=? FOR UPDATE", tokenHash(refresh), row.ID).Scan(&old)
		if q.Error != nil {
			return q.Error
		}
		if q.RowsAffected != 1 {
			return unauthenticated()
		}
		if old.UsedAt != nil {
			if err = s.revoke(txctx, row.ID); err != nil {
				return err
			}
			if err = s.event(txctx, principalOf(row), "auth.refresh_reused", "denied"); err != nil {
				return err
			}
			publicErr = apperrors.Contract(401, "auth.refresh_reused", nil)
			return nil
		}
		p, _, err := s.live(txctx, principalOf(row), true)
		if err != nil {
			return err
		}
		if !old.ExpiresAt.After(time.Now()) {
			return unauthenticated()
		}
		next, err := randomToken()
		if err != nil {
			return err
		}
		id := uuid.NewString()
		now := time.Now().UTC().Truncate(time.Microsecond)
		if err = db.Exec("INSERT INTO platform_refresh_tokens (id,session_id,token_hash,expires_at,created_at) VALUES (?,?,?,?,?)", id, row.ID, tokenHash(next), row.ExpiresAt, now).Error; err != nil {
			return err
		}
		if err = db.Exec("UPDATE platform_refresh_tokens SET used_at=?,replaced_by=? WHERE id=?", now, id, old.ID).Error; err != nil {
			return err
		}
		if err = db.Exec("UPDATE platform_sessions SET last_seen_at=?,updated_at=?,revision=revision+1 WHERE id=?", now, now, row.ID).Error; err != nil {
			return err
		}
		access, err := s.sign(p, now)
		if err != nil {
			return err
		}
		out = Session{AccessToken: access, ExpiresIn: 900, CSRFToken: s.csrf(row.ID, row.Audience, row.CreatedAt), Principal: p, RefreshToken: next, RefreshExpiresAt: row.ExpiresAt}
		return s.event(txctx, p, "auth.refresh", "succeeded")
	})
	if err != nil {
		return Session{}, err
	}
	if publicErr != nil {
		return Session{}, publicErr
	}
	return out, nil
}

// Revocation of the owning session invalidates every descendant token without
// deleting replay evidence or taking token locks in a conflicting order.
func (s *Service) revoke(ctx context.Context, sessionID string) error {
	db, err := s.manager.Require(ctx)
	if err != nil {
		return err
	}
	return db.Exec("UPDATE platform_sessions SET revoked_at=clock_timestamp(),updated_at=clock_timestamp(),revision=revision+1 WHERE id=? AND revoked_at IS NULL", sessionID).Error
}
func (s *Service) Logout(ctx context.Context, refresh, audience, csrf string) (LogoutResult, error) {
	var out LogoutResult
	err := s.manager.Within(ctx, func(txctx context.Context) error {
		row, found, err := s.lookupRefresh(txctx, refresh, audience)
		if err != nil {
			return err
		}
		if !found {
			out.Revoked = true
			return nil
		}
		row, err = s.lockSession(txctx, row)
		if err != nil {
			return err
		}
		if err = s.checkCSRF(row, csrf); err != nil {
			return err
		}
		if row.RevokedAt == nil {
			if err = s.revoke(txctx, row.ID); err != nil {
				return err
			}
			if err = s.event(txctx, principalOf(row), "auth.logout", "succeeded"); err != nil {
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
func (s *Service) WSTicket(ctx context.Context, p Principal) (Ticket, error) {
	var out Ticket
	err := s.manager.Within(ctx, func(txctx context.Context) error {
		current, err := s.Authorize(txctx, p)
		if err != nil {
			return err
		}
		ticket, err := randomToken()
		if err != nil {
			return err
		}
		db, err := s.manager.Require(txctx)
		if err != nil {
			return err
		}
		if err = db.Exec("INSERT INTO platform_ws_tickets (id,session_id,ticket_hash,expires_at) VALUES (?,?,?,?)", uuid.NewString(), current.SessionID, tokenHash(ticket), time.Now().UTC().Add(time.Minute)).Error; err != nil {
			return err
		}
		out = Ticket{Ticket: ticket, ExpiresIn: 60}
		return s.event(txctx, current, "auth.ws_ticket", "succeeded")
	})
	if err != nil {
		return Ticket{}, err
	}
	return out, nil
}
