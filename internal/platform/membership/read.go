package membership

import (
	"context"
	"encoding/json"

	apperrors "InkLore/internal/platform/errors"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/scope"
	"InkLore/internal/platform/uow"

	"gorm.io/gorm"
)

// Authorize is the idempotency authorization callback. For writes it acquires
// the tenant lock before any identity row locks, including on replay. Outside a
// UoW it only checks authorization; mutations always repeat it within the UoW.
func (s *Service) Authorize(ctx context.Context, p identity.Principal, write bool) error {
	if !scope.ValidID(p.TenantID) || !scope.ValidID(p.UserID) || !scope.ValidID(p.SessionID) {
		return apperrors.Contract(401, "auth.unauthenticated", nil)
	}
	if p.Audience != "admin" {
		return apperrors.Contract(403, "auth.forbidden", nil)
	}
	if write && uow.InTransaction(ctx) {
		if err := s.lockTenant(ctx, p); err != nil {
			return err
		}
	}
	roles := []string{"operator", "auditor"}
	if write {
		roles = []string{"operator"}
	}
	live, err := s.authorizer.Authorize(ctx, p, roles...)
	if err != nil {
		return err
	}
	if live.Audience != "admin" || live.TenantID != p.TenantID || live.UserID != p.UserID || live.SessionID != p.SessionID {
		return apperrors.Contract(403, "auth.forbidden", nil)
	}
	if !hasRole(live.Roles, "operator") && (write || !hasRole(live.Roles, "auditor")) {
		return apperrors.Contract(403, "auth.forbidden", nil)
	}
	return nil
}

// AuthorizeMember checks visibility without CAS so a valid replay can return
// the original command result. Use it before looking up an idempotency result.
// For target writes this must be the FIRST authorization in the command UoW;
// do not acquire actor share locks through Authorize/identity beforehand.
func (s *Service) AuthorizeMember(ctx context.Context, p identity.Principal, id string, write bool) error {
	if write && uow.InTransaction(ctx) {
		if !scope.ValidID(id) {
			return validation()
		}
		if err := s.lockTenant(ctx, p); err != nil {
			return err
		}
		db, err := s.manager.Require(ctx)
		if err != nil {
			return err
		}
		// Lock both global users in a stable order BEFORE identity acquires an
		// actor share lock. Different tenants may administer the same accounts.
		var users []string
		if err = db.Raw("SELECT u.id FROM platform_users u WHERE u.id=? OR u.id=(SELECT user_id FROM platform_memberships WHERE tenant_id=? AND id=?) ORDER BY u.id FOR UPDATE", p.UserID, p.TenantID, id).Scan(&users).Error; err != nil {
			return err
		}
	}
	if err := s.Authorize(ctx, p, write); err != nil {
		return err
	}
	if !scope.ValidID(id) {
		return validation()
	}
	db, err := s.manager.DB(ctx)
	if err != nil {
		return err
	}
	_, err = readMember(db, p.TenantID, id, false)
	return err
}

func (s *Service) lockTenant(ctx context.Context, p identity.Principal) error {
	if !scope.ValidID(p.TenantID) || !scope.ValidID(p.UserID) || !scope.ValidID(p.SessionID) {
		return apperrors.Contract(401, "auth.unauthenticated", nil)
	}
	if p.Audience != "admin" {
		return apperrors.Contract(403, "auth.forbidden", nil)
	}
	db, err := s.manager.Require(ctx)
	if err != nil {
		return err
	}
	var tenant string
	r := db.Raw("SELECT id FROM platform_tenants WHERE id=? AND status='active' FOR UPDATE", p.TenantID).Scan(&tenant)
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected == 0 {
		return apperrors.Contract(401, "auth.unauthenticated", nil)
	}
	return nil
}

type memberRow struct {
	Member    `gorm:"embedded"`
	RolesJSON string
}

const memberSelect = "SELECT m.id,m.revision,m.created_at,m.updated_at,m.user_id,u.email,u.display_name,array_to_json(m.roles)::text AS roles_json,m.status FROM platform_memberships m JOIN platform_users u ON u.id=m.user_id "

func decodeMember(row memberRow) (Member, error) {
	out := row.Member
	if err := json.Unmarshal([]byte(row.RolesJSON), &out.Roles); err != nil {
		return Member{}, err
	}
	out.CreatedAt = out.CreatedAt.UTC()
	out.UpdatedAt = out.UpdatedAt.UTC()
	return out, nil
}

func readMember(db *gorm.DB, tenantID, id string, lock bool) (Member, error) {
	query := memberSelect + "WHERE m.tenant_id=? AND m.id=?"
	if lock {
		query += " FOR UPDATE OF m"
	}
	var row memberRow
	r := db.Raw(query, tenantID, id).Scan(&row)
	if r.Error != nil {
		return Member{}, r.Error
	}
	if r.RowsAffected == 0 {
		return Member{}, apperrors.Contract(404, "platform.resource_not_found", nil)
	}
	return decodeMember(row)
}

func (s *Service) Get(ctx context.Context, p identity.Principal, id string) (Member, error) {
	if err := s.Authorize(ctx, p, false); err != nil {
		return Member{}, err
	}
	if !scope.ValidID(id) {
		return Member{}, validation()
	}
	db, err := s.manager.DB(ctx)
	if err != nil {
		return Member{}, err
	}
	return readMember(db, p.TenantID, id, false)
}

func (s *Service) List(ctx context.Context, p identity.Principal, q ListQuery) (Page, error) {
	out := Page{Items: []Member{}}
	if err := s.Authorize(ctx, p, false); err != nil {
		return out, err
	}
	if q.Limit == 0 {
		q.Limit = 20
	}
	if q.Limit < 1 || q.Limit > 100 {
		return out, validation()
	}
	if (q.BeforeID == "") != q.BeforeUpdatedAt.IsZero() || (q.BeforeID != "" && !scope.ValidID(q.BeforeID)) {
		return out, apperrors.Contract(400, "platform.cursor_invalid", nil)
	}
	db, err := s.manager.DB(ctx)
	if err != nil {
		return out, err
	}
	query := memberSelect + "WHERE m.tenant_id=?"
	args := []any{p.TenantID}
	if q.BeforeID != "" {
		query += " AND (m.updated_at,m.id)<(?,?::uuid)"
		args = append(args, q.BeforeUpdatedAt.UTC(), q.BeforeID)
	}
	query += " ORDER BY m.updated_at DESC,m.id DESC LIMIT ?"
	args = append(args, q.Limit+1)
	var rows []memberRow
	if err = db.Raw(query, args...).Scan(&rows).Error; err != nil {
		return out, err
	}
	if len(rows) > q.Limit {
		out.HasMore = true
		rows = rows[:q.Limit]
	}
	for _, row := range rows {
		m, err := decodeMember(row)
		if err != nil {
			return Page{}, err
		}
		out.Items = append(out.Items, m)
	}
	return out, nil
}

func validation() error { return apperrors.Contract(400, "platform.validation_failed", nil) }
func hasRole(roles []string, role string) bool {
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}
func conflict(revision int64) error {
	return apperrors.Contract(409, "platform.version_conflict", map[string]any{"current_revision": revision})
}
