package app

import (
	"context"
	"net/mail"
	"strings"
	"unicode/utf8"

	"InkLore/internal/platform/audit"
	platformerr "InkLore/internal/platform/errors"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/uow"

	"github.com/google/uuid"
)

type BootstrapResult struct {
	TenantID string `json:"tenant_id"`
	UserID   string `json:"user_id"`
	MemberID string `json:"member_id"`
}

// BootstrapIdentity is a deployment command, never an HTTP registration handler.
func BootstrapIdentity(ctx context.Context, manager *uow.Manager, tenantName, email, displayName, password string) (BootstrapResult, error) {
	return BootstrapIdentityWithUserProvision(ctx, manager, tenantName, email, "", displayName, password, nil)
}

// BootstrapIdentityWithProvision keeps deployment-owned tenant provisioning in the identity UoW.
func BootstrapIdentityWithProvision(ctx context.Context, manager *uow.Manager, tenantName, email, displayName, password string, provision func(context.Context, string) error) (BootstrapResult, error) {
	return BootstrapIdentityWithUserProvision(ctx, manager, tenantName, email, "", displayName, password, provision)
}

// BootstrapIdentityWithUserProvision optionally assigns the reserved Admin username.
func BootstrapIdentityWithUserProvision(ctx context.Context, manager *uow.Manager, tenantName, email, username, displayName, password string, provision func(context.Context, string) error) (BootstrapResult, error) {
	var out BootstrapResult
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || len(email) > 254 || (username != "" && username != "root") || utf8.RuneCountInString(tenantName) < 1 || utf8.RuneCountInString(tenantName) > 120 || utf8.RuneCountInString(displayName) < 1 || utf8.RuneCountInString(displayName) > 60 {
		return out, platformerr.Contract(400, "platform.validation_failed", nil)
	}
	hash, err := identity.HashPassword(password)
	if err != nil {
		return out, err
	}
	out = BootstrapResult{TenantID: uuid.NewString(), UserID: uuid.NewString(), MemberID: uuid.NewString()}
	err = manager.Within(ctx, func(txctx context.Context) error {
		db, err := manager.Require(txctx)
		if err != nil {
			return err
		}
		if err = db.Exec("SELECT pg_advisory_xact_lock(?)", int64(731161)).Error; err != nil {
			return err
		}
		var exists bool
		if err = db.Raw("SELECT EXISTS(SELECT 1 FROM platform_users WHERE lower(email)=lower(?))", email).Scan(&exists).Error; err != nil {
			return err
		}
		if exists {
			return platformerr.Contract(409, "auth.account_unavailable", nil)
		}
		if err = db.Exec("INSERT INTO platform_tenants(id,name) VALUES(?,?)", out.TenantID, tenantName).Error; err != nil {
			return err
		}
		var assignedUsername any
		if username != "" {
			assignedUsername = username
		}
		if err = db.Exec("INSERT INTO platform_users(id,email,username,global_roles,display_name,password_hash) VALUES(?,?,?,CASE WHEN ?='root' THEN ARRAY['super_admin']::text[] ELSE ARRAY[]::text[] END,?,?)", out.UserID, strings.ToLower(email), assignedUsername, username, displayName, hash).Error; err != nil {
			return err
		}
		if err = db.Exec("INSERT INTO platform_memberships(id,tenant_id,user_id,roles) VALUES(?,?,?,CASE WHEN ?='root' THEN ARRAY['creator','operator','auditor']::text[] ELSE ARRAY['operator']::text[] END)", out.MemberID, out.TenantID, out.UserID, username).Error; err != nil {
			return err
		}
		if err = identity.ProvisionGlobalAdmins(txctx, manager, out.TenantID); err != nil {
			return err
		}
		if provision != nil {
			if err = provision(txctx, out.TenantID); err != nil {
				return err
			}
		}
		requestID, traceID := correlation(txctx)
		_, err = audit.New(manager).Append(txctx, audit.Record{TenantID: out.TenantID, ActorID: &out.UserID, Audience: "system", Action: audit.ActionBootstrap, ResourceID: &out.MemberID, Outcome: "succeeded", RequestID: requestID, TraceID: traceID, SafeChanges: []audit.SafeChange{{Field: "user_id", After: &out.UserID}, {Field: "member_id", After: &out.MemberID}}})
		return err
	})
	if err != nil {
		return BootstrapResult{}, err
	}
	return out, nil
}
