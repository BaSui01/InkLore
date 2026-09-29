package membership

import (
	"context"
	"encoding/json"
	"net/mail"
	"strings"
	"unicode/utf8"

	apperrors "InkLore/internal/platform/errors"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/scope"

	"github.com/google/uuid"
)

func isForeignKeyViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "23503") || strings.Contains(msg, "violates foreign key constraint")
}

func validRoles(roles []string) bool {
	if len(roles) == 0 || len(roles) > 3 {
		return false
	}
	seen := map[string]bool{}
	for _, role := range roles {
		if (role != "operator" && role != "creator" && role != "auditor") || seen[role] {
			return false
		}
		seen[role] = true
	}
	return true
}
func validPassword(password string) bool {
	return identity.ValidatePassword(password) == nil
}
func validRevision(revision int64) bool { return revision > 0 && revision <= 9007199254740991 }

// Create never attaches an existing global account, even in the same tenant.
// Hashing occurs before this method opens a UoW. A caller already inside a UoW
// (including the idempotency executor) necessarily retains its existing locks.
func (s *Service) Create(ctx context.Context, p identity.Principal, in CreateMember) (Member, error) {
	var out Member
	if err := s.Authorize(ctx, p, true); err != nil {
		return out, err
	}
	address, err := mail.ParseAddress(in.Email)
	if err != nil || address.Address != in.Email || !utf8.ValidString(in.Email) || utf8.RuneCountInString(in.Email) > 254 || strings.ContainsAny(in.Email, "\r\n") || !utf8.ValidString(in.DisplayName) || utf8.RuneCountInString(in.DisplayName) < 1 || utf8.RuneCountInString(in.DisplayName) > 60 || !validRoles(in.Roles) || !validPassword(in.Password) {
		return out, validation()
	}
	hash, err := identity.HashPassword(in.Password)
	if err != nil {
		return out, err
	}
	roles, _ := json.Marshal(in.Roles)
	err = s.manager.Within(ctx, func(txctx context.Context) error {
		if err := s.Authorize(txctx, p, true); err != nil {
			return err
		}
		db, err := s.manager.Require(txctx)
		if err != nil {
			return err
		}
		userID := uuid.NewString()
		memberID := uuid.NewString()
		// DO NOTHING preserves a usable transaction for deterministic idempotent 409.
		r := db.Exec("INSERT INTO platform_users(id,email,display_name,password_hash) VALUES (?,?,?,?) ON CONFLICT DO NOTHING", userID, strings.ToLower(in.Email), in.DisplayName, string(hash))
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected == 0 {
			return apperrors.Contract(409, "auth.account_unavailable", nil)
		}
		if err = db.Exec("INSERT INTO platform_memberships(id,tenant_id,user_id,roles) VALUES (?,?,?,ARRAY(SELECT jsonb_array_elements_text(?::jsonb)))", memberID, p.TenantID, userID, string(roles)).Error; err != nil {
			return err
		}
		out, err = readMember(db, p.TenantID, memberID, false)
		return err
	})
	if err != nil {
		return Member{}, err
	}
	return out, nil
}

func (s *Service) Update(ctx context.Context, p identity.Principal, id string, in UpdateMember) (Member, error) {
	var out Member
	if !scope.ValidID(id) || !validRevision(in.ExpectedRevision) || !validRoles(in.Roles) || (in.Status != "active" && in.Status != "disabled") {
		return out, validation()
	}
	roles, _ := json.Marshal(in.Roles)
	err := s.manager.Within(ctx, func(txctx context.Context) error {
		if err := s.AuthorizeMember(txctx, p, id, true); err != nil {
			return err
		}
		db, err := s.manager.Require(txctx)
		if err != nil {
			return err
		}
		current, err := lockTarget(db, p.TenantID, id)
		if err != nil {
			return err
		}
		if current.Revision != in.ExpectedRevision {
			return conflict(current.Revision)
		}
		// 防御性编程：当前登录用户不能停用自己
		if current.UserID == p.UserID && in.Status != "active" {
			return apperrors.Contract(403, "auth.forbidden", nil)
		}
		// 防御性编程：当前登录用户不能从自己身上移除系统管理员(operator)角色
		if current.UserID == p.UserID && hasRole(current.Roles, "operator") && !hasRole(in.Roles, "operator") {
			return apperrors.Contract(403, "auth.forbidden", nil)
		}
		// 防御性编程：必须保留至少一个系统管理员（operator），防停用、防降级
		if hasRole(current.Roles, "operator") && (in.Status != "active" || !hasRole(in.Roles, "operator")) {
			var activeCount int64
			if err = db.Raw("SELECT count(*) FROM platform_memberships m JOIN platform_users u ON u.id=m.user_id WHERE m.tenant_id=? AND m.status='active' AND u.status='active' AND 'operator'=ANY(m.roles)", p.TenantID).Scan(&activeCount).Error; err != nil {
				return err
			}
			if activeCount <= 1 {
				return apperrors.Contract(409, "auth.last_operator", nil)
			}
			var totalCount int64
			if err = db.Raw("SELECT count(*) FROM platform_memberships WHERE tenant_id=? AND 'operator'=ANY(roles)", p.TenantID).Scan(&totalCount).Error; err != nil {
				return err
			}
			if totalCount <= 1 {
				return apperrors.Contract(409, "auth.last_operator", nil)
			}
		}
		r := db.Exec("UPDATE platform_memberships SET roles=ARRAY(SELECT jsonb_array_elements_text(?::jsonb)),status=?,revision=revision+1,updated_at=clock_timestamp() WHERE tenant_id=? AND id=? AND revision=?", string(roles), in.Status, p.TenantID, id, in.ExpectedRevision)
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected != 1 {
			return conflict(current.Revision)
		}
		// Every role/state update invalidates old membership-bound sessions immediately.
		if err = db.Exec("UPDATE platform_sessions SET revoked_at=clock_timestamp(),revision=revision+1,updated_at=clock_timestamp() WHERE tenant_id=? AND user_id=? AND revoked_at IS NULL", p.TenantID, current.UserID).Error; err != nil {
			return err
		}
	out, err = readMember(db, p.TenantID, id, false)
		return err
	})
	if err != nil {
		return Member{}, err
	}
	return out, nil
}

func (s *Service) Delete(ctx context.Context, p identity.Principal, id string) error {
	if !scope.ValidID(id) {
		return validation()
	}
	return s.manager.Within(ctx, func(txctx context.Context) error {
		if err := s.AuthorizeMember(txctx, p, id, true); err != nil {
			return err
		}
		db, err := s.manager.Require(txctx)
		if err != nil {
			return err
		}
		current, err := lockTarget(db, p.TenantID, id)
		if err != nil {
			return err
		}
		// 防御性编程：严厉禁止删除当前登录的管理员账号
		if current.UserID == p.UserID {
			return apperrors.Contract(403, "auth.forbidden", nil)
		}
		// 防御性编程：必须保留至少一个系统管理员，禁止删除最后一个管理员
		if hasRole(current.Roles, "operator") {
			var activeCount int64
			if err = db.Raw("SELECT count(*) FROM platform_memberships m JOIN platform_users u ON u.id=m.user_id WHERE m.tenant_id=? AND m.status='active' AND u.status='active' AND 'operator'=ANY(m.roles)", p.TenantID).Scan(&activeCount).Error; err != nil {
				return err
			}
			if activeCount <= 1 {
				return apperrors.Contract(409, "auth.last_operator", nil)
			}
			var totalCount int64
			if err = db.Raw("SELECT count(*) FROM platform_memberships WHERE tenant_id=? AND 'operator'=ANY(roles)", p.TenantID).Scan(&totalCount).Error; err != nil {
				return err
			}
			if totalCount <= 1 {
				return apperrors.Contract(409, "auth.last_operator", nil)
			}
		}
		// 1. 级联清理该成员在该租户下的创作命令与创作会话（解除外键依赖）
		if err = db.Exec("DELETE FROM agent_novel_creation_commands WHERE tenant_id=? AND session_id IN (SELECT id FROM agent_novel_creation_sessions WHERE tenant_id=? AND owner_user_id=?)", p.TenantID, p.TenantID, current.UserID).Error; err != nil {
			if isForeignKeyViolation(err) {
				return apperrors.Contract(409, "platform.resource_in_use", map[string]any{"reason": "member has active novel creation commands in use"})
			}
			return err
		}
		if err = db.Exec("DELETE FROM agent_novel_creation_sessions WHERE tenant_id=? AND owner_user_id=?", p.TenantID, current.UserID).Error; err != nil {
			if isForeignKeyViolation(err) {
				return apperrors.Contract(409, "platform.resource_in_use", map[string]any{"reason": "member has active novel creation sessions in use"})
			}
			return err
		}
		// 2. 级联删除该成员会话下的刷新令牌与 WebSocket 票据（解除 platform_sessions 外键依赖）
		if err = db.Exec("UPDATE platform_refresh_tokens SET replaced_by = NULL WHERE session_id IN (SELECT id FROM platform_sessions WHERE tenant_id=? AND user_id=?)", p.TenantID, current.UserID).Error; err != nil {
			return err
		}
		if err = db.Exec("DELETE FROM platform_refresh_tokens WHERE session_id IN (SELECT id FROM platform_sessions WHERE tenant_id=? AND user_id=?)", p.TenantID, current.UserID).Error; err != nil {
			return err
		}
		if err = db.Exec("DELETE FROM platform_ws_tickets WHERE session_id IN (SELECT id FROM platform_sessions WHERE tenant_id=? AND user_id=?)", p.TenantID, current.UserID).Error; err != nil {
			return err
		}
		// 3. 级联删除该成员在该租户下的所有会话记录（解开 platform_sessions_tenant_id_user_id_fkey 外键约束）
		if err = db.Exec("DELETE FROM platform_sessions WHERE tenant_id=? AND user_id=?", p.TenantID, current.UserID).Error; err != nil {
			if isForeignKeyViolation(err) {
				return apperrors.Contract(409, "platform.resource_in_use", map[string]any{"reason": "member sessions are currently in use"})
			}
			return err
		}
		// 4. 删除租户成员关系
		r := db.Exec("DELETE FROM platform_memberships WHERE tenant_id=? AND id=?", p.TenantID, id)
		if r.Error != nil {
			if isForeignKeyViolation(r.Error) {
				return apperrors.Contract(409, "platform.resource_in_use", map[string]any{
					"reason": "member is referenced by historical records and cannot be deleted",
				})
			}
			return r.Error
		}
		if r.RowsAffected != 1 {
			return apperrors.Contract(404, "platform.resource_not_found", nil)
		}
		return nil
	})
}
