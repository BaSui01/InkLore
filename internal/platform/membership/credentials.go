package membership

import (
	"context"

	apperrors "InkLore/internal/platform/errors"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/scope"

	"gorm.io/gorm"
)

// ResetPassword uses the request's password and returns no credential material.
// The membership revision is the command CAS and response revision. The user's
// independent revision also advances when its password hash changes.
func (s *Service) ResetPassword(ctx context.Context, p identity.Principal, id string, in ResetPassword) (RevokeSessions, error) {
	var out RevokeSessions
	if !scope.ValidID(id) || !validRevision(in.ExpectedRevision) || !validPassword(in.NewPassword) {
		return out, validation()
	}
	if err := s.AuthorizeMember(ctx, p, id, true); err != nil {
		return out, err
	}
	hash, err := identity.HashPassword(in.NewPassword)
	if err != nil {
		return out, err
	}
	err = s.manager.Within(ctx, func(txctx context.Context) error {
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
		var count int64
		if err = db.Raw("SELECT count(*) FROM platform_memberships WHERE user_id=? AND status='active'", current.UserID).Scan(&count).Error; err != nil {
			return err
		}
		if count != 1 || current.Status != "active" {
			return apperrors.Contract(403, "auth.cross_tenant_account", nil)
		}
		if err = bumpMember(db, p.TenantID, current, in.ExpectedRevision); err != nil {
			return err
		}
		if err = db.Exec("UPDATE platform_users SET password_hash=?,revision=revision+1,updated_at=clock_timestamp() WHERE id=?", string(hash), current.UserID).Error; err != nil {
			return err
		}
		r := db.Exec("UPDATE platform_sessions SET revoked_at=clock_timestamp(),revision=revision+1,updated_at=clock_timestamp() WHERE user_id=? AND revoked_at IS NULL", current.UserID)
		if r.Error != nil {
			return r.Error
		}
		out = RevokeSessions{RevokedCount: r.RowsAffected, Revision: current.Revision + 1}
		return nil
	})
	if err != nil {
		return RevokeSessions{}, err
	}
	return out, nil
}

// RevokeSessions only revokes this tenant member's sessions. Password reset is
// intentionally stronger and revokes sessions in every tenant for the account.
func (s *Service) RevokeSessions(ctx context.Context, p identity.Principal, id string, in RevisionCommand) (RevokeSessions, error) {
	var out RevokeSessions
	if !scope.ValidID(id) || !validRevision(in.ExpectedRevision) {
		return out, validation()
	}
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
		if err = bumpMember(db, p.TenantID, current, in.ExpectedRevision); err != nil {
			return err
		}
		r := db.Exec("UPDATE platform_sessions SET revoked_at=clock_timestamp(),revision=revision+1,updated_at=clock_timestamp() WHERE tenant_id=? AND user_id=? AND revoked_at IS NULL", p.TenantID, current.UserID)
		if r.Error != nil {
			return r.Error
		}
		out = RevokeSessions{RevokedCount: r.RowsAffected, Revision: current.Revision + 1}
		return nil
	})
	if err != nil {
		return RevokeSessions{}, err
	}
	return out, nil
}

// The tenant is already locked. Lock the global account before its membership
// to serialize password reset with active relationship changes. FOR UPDATE also
// conflicts with the user FK key-share lock of a new relationship insertion.
func lockTarget(db *gorm.DB, tenantID, id string) (Member, error) {
	current, err := readMember(db, tenantID, id, false)
	if err != nil {
		return Member{}, err
	}
	var userID string
	r := db.Raw("SELECT id FROM platform_users WHERE id=? FOR UPDATE", current.UserID).Scan(&userID)
	if r.Error != nil {
		return Member{}, r.Error
	}
	if r.RowsAffected == 0 {
		return Member{}, apperrors.Contract(404, "platform.resource_not_found", nil)
	}
	return readMember(db, tenantID, id, true)
}

func bumpMember(db *gorm.DB, tenantID string, current Member, expected int64) error {
	r := db.Exec("UPDATE platform_memberships SET revision=revision+1,updated_at=clock_timestamp() WHERE tenant_id=? AND id=? AND revision=?", tenantID, current.ID, expected)
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected != 1 {
		return conflict(current.Revision)
	}
	return nil
}
