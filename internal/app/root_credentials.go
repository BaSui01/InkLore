package app

import (
	"context"
	"net/mail"
	"strings"

	"InkLore/internal/platform/audit"
	platformerr "InkLore/internal/platform/errors"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/uow"
)

// SetRootCredentials binds the reserved Admin username to an existing operator email.
func SetRootCredentials(ctx context.Context, manager *uow.Manager, email, password string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || len(email) > 254 {
		return platformerr.Contract(400, "platform.validation_failed", nil)
	}
	hash, err := identity.HashPassword(password)
	if err != nil {
		return err
	}
	return manager.Within(ctx, func(txctx context.Context) error {
		db, err := manager.Require(txctx)
		if err != nil {
			return err
		}
		var userID string
		query := db.Raw("SELECT id FROM platform_users WHERE lower(email)=? AND status='active' FOR UPDATE", email).Scan(&userID)
		if query.Error != nil {
			return query.Error
		}
		if query.RowsAffected != 1 {
			return platformerr.Contract(404, "platform.resource_not_found", nil)
		}
		var isOperator bool
		if err = db.Raw("SELECT EXISTS(SELECT 1 FROM platform_memberships WHERE user_id=? AND status='active' AND 'operator'=ANY(roles))", userID).Scan(&isOperator).Error; err != nil {
			return err
		}
		if !isOperator {
			return platformerr.Contract(403, "auth.forbidden", nil)
		}
		if err = db.Exec("UPDATE platform_users SET username='root',global_roles=ARRAY['super_admin']::text[],password_hash=?,revision=revision+1,updated_at=clock_timestamp() WHERE id=?", hash, userID).Error; err != nil {
			return err
		}
		if err = db.Exec(`INSERT INTO platform_memberships
			(id,tenant_id,user_id,roles,status,revision,created_at,updated_at)
			SELECT gen_random_uuid(),t.id,?,ARRAY['creator','operator','auditor']::text[],'active',1,clock_timestamp(),clock_timestamp()
			FROM platform_tenants t WHERE t.status='active'
			ON CONFLICT (tenant_id,user_id) DO UPDATE SET roles=EXCLUDED.roles,status='active',revision=platform_memberships.revision+1,updated_at=clock_timestamp()`, userID).Error; err != nil {
			return err
		}
		if err = db.Exec("UPDATE platform_sessions SET revoked_at=clock_timestamp(),revision=revision+1,updated_at=clock_timestamp() WHERE user_id=? AND revoked_at IS NULL", userID).Error; err != nil {
			return err
		}
		var tenantIDs []string
		if err = db.Raw("SELECT id FROM platform_tenants WHERE status='active' ORDER BY created_at,id").Scan(&tenantIDs).Error; err != nil {
			return err
		}
		for _, tenantID := range tenantIDs {
			requestID, traceID := correlation(txctx)
			if _, err = audit.New(manager).Append(txctx, audit.Record{TenantID: tenantID, ActorID: &userID, Audience: "system", Action: "auth.password_changed", ResourceID: &userID, Outcome: "succeeded", RequestID: requestID, TraceID: traceID}); err != nil {
				return err
			}
		}
		return nil
	})
}
