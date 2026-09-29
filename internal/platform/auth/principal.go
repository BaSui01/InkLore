package authdomain

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

const (
	PrincipalSourceAuthCenter     = "auth_center"
	PrincipalSourceLocal          = "local"
	PrincipalSourceGatewayAPIKey  = "gateway_api_key"
	PrincipalSourceSystem         = "system"
	PrincipalSourceAnonymous      = "anonymous"
	PrincipalSourceInputUpload    = "input_image_upload"
	PrincipalSourceWorkflowAsset  = "workflow_asset"
	PrincipalSourceApproval       = "approval"
	PrincipalSourceLegacyWorkflow = "legacy_workflow"
	PrincipalSourceLegacyTask     = "legacy_task"

	defaultOrgProjectionMaxAge = 30 * time.Minute
)

const DefaultOrgProjectionMaxAge = defaultOrgProjectionMaxAge

type Principal struct {
	DepartmentID        *int64
	OrgSyncedAt         *time.Time
	TeamID              *int64
	User                *User
	ApplicationID       string
	TenantID            string
	OrganizationID      string
	Source              string
	Subject             string
	Permissions         []string
	Roles               []string
	DepartmentIDs       []int64
	PermissionVersion   int64
	OrganizationVersion int64
	UserID              int64
}

type Decision struct {
	Reason string
	Source string
	Allow  bool
}

func PrincipalFromUser(user *User) Principal {
	if user == nil {
		return AnonymousPrincipal()
	}
	departmentIDs := userDepartmentIDs(user)
	var departmentID *int64
	if len(departmentIDs) > 0 {
		id := departmentIDs[0]
		departmentID = &id
	}
	teamID := principalNullableInt64Ptr(user.TeamID.Valid, user.TeamID.Int64)
	subject := strings.TrimSpace(user.AuthCenterUserID)
	if subject == "" && user.SSOSub.Valid {
		subject = strings.TrimSpace(user.SSOSub.String)
	}
	if subject == "" {
		subject = "user:" + strings.TrimSpace(user.Username)
	}
	tenantID := user.EffectiveTenantID()
	return Principal{
		UserID:              user.ID,
		ApplicationID:       user.AuthCenterApplicationID,
		TenantID:            tenantID,
		OrganizationID:      user.AuthCenterOrganizationID,
		PermissionVersion:   user.AuthCenterPermissionVersion,
		OrganizationVersion: user.AuthCenterOrganizationVersion,
		Permissions:         append([]string(nil), user.AuthCenterPermissions...),
		Roles:               append([]string(nil), user.AuthCenterRoles...),
		Source:              PrincipalSourceAuthCenter,
		Subject:             subject,
		DepartmentID:        departmentID,
		DepartmentIDs:       departmentIDs,
		OrgSyncedAt:         user.OrgSyncedAt,
		TeamID:              teamID,
		User:                user,
	}

}

func (p Principal) EffectiveTenantID() string {
	return strings.TrimSpace(p.TenantID)
}

func PrincipalFromAPIKey(rawKey string) Principal {
	return Principal{
		Source:  PrincipalSourceGatewayAPIKey,
		Subject: apiKeySubject(rawKey),
	}
}

func SystemPrincipal(subject string) Principal {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		subject = "system"
	}
	return Principal{
		Source:  PrincipalSourceSystem,
		Subject: subject,
	}
}

func AnonymousPrincipal() Principal {
	return Principal{
		Source:  PrincipalSourceAnonymous,
		Subject: "anonymous",
	}
}

func principalNullableInt64Ptr(valid bool, value int64) *int64 {
	if !valid || value <= 0 {
		return nil
	}
	v := value
	return &v
}

func userDepartmentIDs(user *User) []int64 {
	if user == nil {
		return nil
	}
	return user.EffectiveDepartmentIDs()
}

func apiKeySubject(rawKey string) string {
	rawKey = strings.TrimSpace(rawKey)
	if rawKey == "" {
		return "api_key:missing"
	}
	sum := sha256.Sum256([]byte(rawKey))
	return "api_key:" + hex.EncodeToString(sum[:8])
}

func principalDecisionSource(principal Principal) string {
	source := strings.TrimSpace(principal.Source)
	if source == "" {
		return PrincipalSourceAnonymous
	}
	return source
}

func PrincipalDecisionSource(principal Principal) string {
	return principalDecisionSource(principal)
}

func principalHasSuperRole(principal Principal) bool {
	for _, role := range principal.Roles {
		if AuthCenterRoleMatches(role, RoleSuperAdmin) {
			return true
		}
	}
	return false
}

func PrincipalHasSuperRole(principal Principal) bool {
	return principalHasSuperRole(principal)
}

func principalHasAuthCenterRole(principal Principal, roles ...string) bool {
	if len(roles) == 0 {
		return false
	}
	wanted := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		role = strings.TrimSpace(role)
		if role != "" {
			wanted[role] = struct{}{}
		}
	}
	if len(wanted) == 0 {
		return false
	}
	for _, role := range principal.Roles {
		for wantedRole := range wanted {
			if AuthCenterRoleMatches(role, wantedRole) {
				return true
			}
		}
	}
	return false
}

func principalHasGlobalDataScope(principal Principal) bool {
	if principalHasSuperRole(principal) {
		return true
	}
	return principalHasAuthCenterRole(principal, RoleGeneralManager, RoleTenantAdmin)
}

func PrincipalHasAuthCenterRole(principal Principal, roles ...string) bool {
	return principalHasAuthCenterRole(principal, roles...)
}

func PrincipalHasGlobalDataScope(principal Principal) bool {
	return principalHasGlobalDataScope(principal)
}

func PrincipalHasPermission(principal Principal, permission string) bool {
	permission = strings.TrimSpace(permission)
	if permission == "" {
		return false
	}
	normReq := NormalizePermissionCode(permission)
	alts := AuthCenterPermissionAlternatives(permission)
	for _, granted := range principal.Permissions {
		granted = strings.TrimSpace(granted)
		if granted == AuthCenterPermissionWildcard || granted == "*" {
			return true
		}
		normGranted := NormalizePermissionCode(granted)
		if normGranted == normReq {
			return true
		}
		for _, alt := range alts {
			if normGranted == NormalizePermissionCode(alt) {
				return true
			}
		}
	}
	return false
}

func PrincipalOrgProjectionFresh(principal Principal, maxAge time.Duration) bool {
	return principalOrgProjectionFresh(principal, maxAge)
}

func principalOrgProjectionFresh(principal Principal, maxAge time.Duration) bool {
	syncedAt := principal.OrgSyncedAt
	if syncedAt == nil && principal.User != nil {
		syncedAt = principal.User.OrgSyncedAt
	}
	return orgProjectionFreshAt(syncedAt, maxAge)
}

func orgProjectionFreshAt(syncedAt *time.Time, maxAge time.Duration) bool {
	if syncedAt == nil || syncedAt.IsZero() {
		return false
	}
	if maxAge <= 0 {
		maxAge = defaultOrgProjectionMaxAge
	}
	return time.Since(*syncedAt) <= maxAge
}

func PrincipalDepartmentIDs(principal Principal) []int64 {
	return principalDepartmentIDs(principal)
}

func PrincipalTeamID(principal Principal) *int64 {
	return principalTeamID(principal)
}
