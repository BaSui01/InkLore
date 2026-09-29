package identity

import (
	"context"
	"time"

	"InkLore/internal/platform/audit"
	"InkLore/internal/platform/scope"

	"github.com/google/uuid"
)

func (s *Service) ListTenants(ctx context.Context, p Principal) ([]TenantSummary, error) {
	out := []TenantSummary{}
	err := s.manager.Within(ctx, func(txctx context.Context) error {
		current, _, err := s.live(txctx, p, false)
		if err != nil {
			return err
		}
		if current.Audience != "admin" || !hasGlobalRole(current, "super_admin") {
			return forbidden()
		}
		db, err := s.manager.Require(txctx)
		if err != nil {
			return err
		}
		return db.Raw("SELECT id,name FROM platform_tenants WHERE status='active' ORDER BY name,id").Scan(&out).Error
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) SwitchTenant(ctx context.Context, p Principal, csrf, tenantID string) (Session, error) {
	var out Session
	if p.Audience != "admin" || !scope.ValidID(tenantID) {
		return out, invalid()
	}
	err := s.manager.Within(ctx, func(txctx context.Context) error {
		current, old, err := s.live(txctx, p, true)
		if err != nil {
			return err
		}
		if !hasGlobalRole(current, "super_admin") {
			return forbidden()
		}
		if err = s.checkCSRF(old, csrf); err != nil {
			return err
		}
		next, _, err := s.member(txctx, Principal{UserID: current.UserID, TenantID: tenantID, Audience: "admin"})
		if err != nil {
			return err
		}
		if !hasGlobalRole(next, "super_admin") {
			return forbidden()
		}
		db, err := s.manager.Require(txctx)
		if err != nil {
			return err
		}
		id := uuid.NewString()
		now := time.Now().UTC().Truncate(time.Microsecond)
		expiry := now.Add(SessionLifetime)
		csrfToken := s.csrf(id, "admin", now)
		refresh, err := randomToken()
		if err != nil {
			return err
		}
		if err = db.Exec("INSERT INTO platform_sessions (id,tenant_id,user_id,audience,csrf_hash,membership_revision,expires_at,created_at,updated_at,last_seen_at) VALUES (?,?,?,?,?,?,?,?,?,?)", id, tenantID, current.UserID, "admin", tokenHash(csrfToken), next.MembershipRevision, expiry, now, now, now).Error; err != nil {
			return err
		}
		if err = db.Exec("INSERT INTO platform_refresh_tokens (id,session_id,token_hash,expires_at,created_at) VALUES (?,?,?,?,?)", uuid.NewString(), id, tokenHash(refresh), expiry, now).Error; err != nil {
			return err
		}
		if err = s.revoke(txctx, old.ID); err != nil {
			return err
		}
		next.SessionID = id
		access, err := s.sign(next, now)
		if err != nil {
			return err
		}
		out = Session{AccessToken: access, ExpiresIn: int(AccessLifetime.Seconds()), CSRFToken: csrfToken, Principal: next, RefreshToken: refresh, RefreshExpiresAt: expiry}
		return s.event(txctx, next, audit.ActionTenantSwitched, "succeeded")
	})
	if err != nil {
		return Session{}, err
	}
	return out, nil
}
