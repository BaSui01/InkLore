package identity

import (
	"context"
	"crypto/hmac"
	"encoding/json"
	"time"

	"InkLore/internal/platform/scope"
	"InkLore/internal/platform/uow"
)

type userRow struct {
	ID, DisplayName, PasswordHash, Status string
	GlobalRolesJSON                       string
	Revision                              int64
}
type sessionRow struct {
	ID, TenantID, UserID, Audience, CSRFHash string
	MembershipRevision                       int64
	CreatedAt, ExpiresAt                     time.Time
	RevokedAt                                *time.Time
}

// Lock order for every identity transaction is tenant, user, membership, session,
// refresh token. Callers must authorize before locking their business rows.
func (s *Service) member(ctx context.Context, p Principal) (Principal, userRow, error) {
	var user userRow
	if !scope.ValidID(p.TenantID) || !scope.ValidID(p.UserID) || !validAudience(p.Audience) {
		return Principal{}, user, unauthenticated()
	}
	db, err := s.manager.DB(ctx)
	if err != nil {
		return Principal{}, user, err
	}
	suffix := ""
	if uow.InTransaction(ctx) && !uow.ReadSnapshot(ctx) {
		suffix = " FOR SHARE"
	}
	var tenant struct{ Status string }
	q := db.Raw("SELECT status FROM platform_tenants WHERE id=?"+suffix, p.TenantID).Scan(&tenant)
	if q.Error != nil {
		return Principal{}, user, q.Error
	}
	if q.RowsAffected != 1 || tenant.Status != "active" {
		return Principal{}, user, unauthenticated()
	}
	q = db.Raw("SELECT id,display_name,password_hash,status,revision,array_to_json(global_roles)::text AS global_roles_json FROM platform_users WHERE id=?"+suffix, p.UserID).Scan(&user)
	if q.Error != nil {
		return Principal{}, user, q.Error
	}
	if q.RowsAffected != 1 || user.Status != "active" {
		return Principal{}, user, unauthenticated()
	}
	var m struct {
		RolesJSON string
		Status    string
		Revision  int64
	}
	q = db.Raw("SELECT array_to_json(roles)::text AS roles_json,status,revision FROM platform_memberships WHERE tenant_id=? AND user_id=?"+suffix, p.TenantID, p.UserID).Scan(&m)
	if q.Error != nil {
		return Principal{}, user, q.Error
	}
	if q.RowsAffected != 1 || m.Status != "active" {
		return Principal{}, user, unauthenticated()
	}
	// Principal is copied by value but its slices may still be shared across
	// concurrent requests. Decode into fresh storage, never mutate caller roles.
	p.Roles = nil
	p.GlobalRoles = nil
	if err = json.Unmarshal([]byte(m.RolesJSON), &p.Roles); err != nil {
		return Principal{}, user, err
	}
	if err = json.Unmarshal([]byte(user.GlobalRolesJSON), &p.GlobalRoles); err != nil {
		return Principal{}, user, err
	}
	if p.GlobalRoles == nil {
		p.GlobalRoles = []string{}
	}
	p.DisplayName = user.DisplayName
	p.UserRevision = user.Revision
	p.MembershipRevision = m.Revision
	if !allowedAudience(p) {
		return Principal{}, user, forbidden()
	}
	return p, user, nil
}
func (s *Service) live(ctx context.Context, p Principal, exclusive bool) (Principal, sessionRow, error) {
	var row sessionRow
	if !scope.ValidID(p.SessionID) {
		return Principal{}, row, unauthenticated()
	}
	current, _, err := s.member(ctx, p)
	if err != nil {
		return Principal{}, row, err
	}
	db, err := s.manager.DB(ctx)
	if err != nil {
		return Principal{}, row, err
	}
	suffix := ""
	if uow.InTransaction(ctx) && !uow.ReadSnapshot(ctx) {
		suffix = " FOR SHARE"
		if exclusive {
			suffix = " FOR UPDATE"
		}
	}
	q := db.Raw("SELECT * FROM platform_sessions WHERE id=? AND tenant_id=? AND user_id=?"+suffix, p.SessionID, p.TenantID, p.UserID).Scan(&row)
	if q.Error != nil {
		return Principal{}, row, q.Error
	}
	if q.RowsAffected != 1 {
		return Principal{}, row, unauthenticated()
	}
	if row.Audience != p.Audience {
		return Principal{}, row, forbidden()
	}
	if row.RevokedAt != nil || !row.ExpiresAt.After(time.Now()) || row.MembershipRevision != current.MembershipRevision {
		return Principal{}, row, unauthenticated()
	}
	return current, row, nil
}

// Authenticate takes the compact JWT (without the HTTP Bearer prefix).
// Only signed IDs are retained: live PostgreSQL roles replace all JWT roles.
func (s *Service) Authenticate(ctx context.Context, bearerToken, audience string) (Principal, error) {
	claims, err := s.verify(bearerToken)
	if err != nil {
		return Principal{}, err
	}
	if !validAudience(audience) && audience != "either" {
		return Principal{}, invalid()
	}
	if audience != "either" && claims.Audience != audience {
		return Principal{}, forbidden()
	}
	return s.Authorize(ctx, Principal{UserID: claims.Subject, TenantID: claims.TenantID, Audience: claims.Audience, SessionID: claims.SessionID})
}

// Authorize joins an existing command UoW and retains row share locks until its
// commit. Standalone calls own a short read UoW; commands must recheck inside theirs.
func (s *Service) Authorize(ctx context.Context, p Principal, allowedRoles ...string) (Principal, error) {
	var current Principal
	err := s.manager.Within(ctx, func(txctx context.Context) error {
		var err error
		current, _, err = s.live(txctx, p, false)
		if err != nil {
			return err
		}
		if len(allowedRoles) == 0 {
			return nil
		}
		for _, required := range allowedRoles {
			for _, role := range current.Roles {
				if required == role {
					return nil
				}
			}
			if current.Audience == "admin" && hasGlobalRole(current, "super_admin") && (required == "operator" || required == "auditor" || required == "super_admin") {
				return nil
			}
		}
		return forbidden()
	})
	if err != nil {
		return Principal{}, err
	}
	return current, nil
}

func hasGlobalRole(p Principal, required string) bool {
	for _, role := range p.GlobalRoles {
		if role == required {
			return true
		}
	}
	return false
}
func (s *Service) Me(ctx context.Context, p Principal) (Principal, error) { return s.Authorize(ctx, p) }
func (s *Service) ValidateCSRF(ctx context.Context, p Principal, csrf string) error {
	return s.manager.Within(ctx, func(txctx context.Context) error {
		_, row, err := s.live(txctx, p, false)
		if err != nil {
			return err
		}
		return s.checkCSRF(row, csrf)
	})
}
func (s *Service) checkCSRF(row sessionRow, csrf string) error {
	expected := s.csrf(row.ID, row.Audience, row.CreatedAt)
	if !validToken(csrf) || !hmac.Equal([]byte(expected), []byte(csrf)) || !hmac.Equal([]byte(tokenHash(csrf)), []byte(row.CSRFHash)) {
		return forbidden()
	}
	return nil
}
func (s *Service) event(ctx context.Context, p Principal, action, outcome string) error {
	if s.onEvent == nil {
		return nil
	}
	return s.onEvent(ctx, Event{Principal: p, ActorID: p.UserID, TenantID: p.TenantID, Audience: p.Audience, ResourceID: p.SessionID, Action: action, Outcome: outcome})
}
