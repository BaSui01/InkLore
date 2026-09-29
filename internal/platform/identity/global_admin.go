package identity

import (
	"context"
	"time"

	"InkLore/internal/platform/uow"
)

// ProvisionGlobalAdmins adds active global administrators to a newly created tenant.
func ProvisionGlobalAdmins(ctx context.Context, manager *uow.Manager, tenantID string) error {
	db, err := manager.Require(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	return db.Exec(`INSERT INTO platform_memberships
		(id,tenant_id,user_id,roles,status,revision,created_at,updated_at)
		SELECT gen_random_uuid(),?,u.id,ARRAY['creator','operator','auditor']::text[],'active',1,?,?
		FROM platform_users u WHERE u.status='active' AND 'super_admin'=ANY(u.global_roles)
		ON CONFLICT (tenant_id,user_id) DO NOTHING`, tenantID, now, now).Error
}
