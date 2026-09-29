package authdomain

import (
	"InkLore/internal/platform/nulltype"
	"InkLore/internal/platform/timeutil"
	"encoding/json"
	"strings"
	"time"
)

// 角色常量 - 内置标准角色代码（向后兼容及认证中心标准角色映射）
// 架构说明：系统核心鉴权决策全面基于细粒度「权限点 (Permissions)」与「数据范围 (Data Scope)」，
// 角色为认证中心动态下发（AuthCenterRoles []string），不局限于固定角色。
const (
	RoleSuperAdmin        = "super_admin"        // 超级管理员（全平台运维）
	RoleTenantAdmin       = "tenant_admin"       // 企业租户管理员（商户企业级管理）
	RoleGeneralManager    = "general_manager"    // 总经理 / 全局数据
	RoleDepartmentManager = "department_manager" // 部长 / 部门数据
	RoleTeamLeader        = "team_leader"        // 组长 / 团队数据
	RoleEmployee          = "employee"           // 员工 / 个人数据
)

// 状态常量
const (
	StatusActive   = "active"
	StatusDisabled = "disabled"
)

// 完全租户隔离约定（用户决策 2026-09-05）：平台不设任何"默认租户"。
// 租户 ID 唯一权威来源是认证中心（Auth Center 签发的 JWT claims，经 RequireTenant 校验后进入上下文）；
// 代码与迁移中禁止硬编码任何租户 UUID，历史 "default"/"default_tenant" 为错误值且已废除：
// 租户上下文缺失时一律 fail-closed（middleware.ErrTenantRequired / tenant_context_required），
// 唯一跨租户例外是 super_admin 系统管理路径（显式 dbtx.WithGlobalTenantScope + 权限 + 审计）。

// User 用户模型
type User struct {
	UpdatedAt                     time.Time       `json:"updated_at"`
	CreatedAt                     time.Time       `json:"created_at"`
	OrgSyncedAt                   *time.Time      `json:"org_synced_at,omitempty"`
	PrimaryDepartmentID           *int64          `json:"primary_department_id,omitempty"`
	ResetRequestedAt              nulltype.Time   `json:"-"`
	EmailVerifiedAt               nulltype.Time   `json:"email_verified_at"`
	LastLoginAt                   nulltype.Time   `json:"last_login_at"`
	EmailVerificationExpires      nulltype.Time   `json:"-"`
	ResetTokenExpiresAt           nulltype.Time   `json:"-"`
	DepartmentName                string          `json:"department_name,omitempty"`
	TeamName                      string          `json:"team_name,omitempty"`
	DisplayName                   string          `json:"display_name"`
	PasswordHash                  string          `json:"-"`
	Username                      string          `json:"username"`
	AuthCenterUserID              string          `json:"auth_center_user_id,omitempty"`
	AuthCenterApplicationID       string          `json:"-"`
	AuthCenterTenantID            string          `json:"-"`
	AuthCenterOrganizationID      string          `json:"-"`
	AuthCenterUserStatus          string          `json:"-"`
	AuthCenterTenantStatus        string          `json:"-"`
	Role                          string          `json:"role"`
	Status                        string          `json:"status"`
	EmailVerificationToken        nulltype.String `json:"-"`
	Email                         nulltype.String `json:"email"`
	Mobile                        nulltype.String `json:"mobile"`
	FeishuOpenID                  nulltype.String `json:"feishu_open_id"`
	WeComUserID                   nulltype.String `json:"wecom_user_id"`
	DingTalkUserID                nulltype.String `json:"dingtalk_user_id"`
	SSOSub                        nulltype.String `json:"-"`
	ResetTokenHash                nulltype.String `json:"-"`
	AuthCenterPermissions         []string        `json:"-"`
	AvatarURL                     nulltype.String `json:"avatar_url"`
	AuthCenterRoles               []string        `json:"-"`
	AuthCenterPermissionVersion   int64           `json:"-"`
	AuthCenterOrganizationVersion int64           `json:"-"`
	DepartmentIDs                 []int64         `json:"department_ids,omitempty"`
	TeamID                        nulltype.Int64  `json:"team_id"`
	DepartmentID                  nulltype.Int64  `json:"department_id"`
	ID                            int64           `json:"id"`
	EmailVerified                 bool            `json:"email_verified"`
}

// MarshalJSON 自定义 JSON 序列化，将 sql.Null* 类型转换为友好格式
func (u *User) MarshalJSON() ([]byte, error) {
	type Alias User

	var email *string
	if u.Email.Valid {
		email = &u.Email.String
	}

	var emailVerifiedAt *time.Time
	if u.EmailVerifiedAt.Valid {
		emailVerifiedAt = &u.EmailVerifiedAt.Time
	}

	var avatarURL *string
	if u.AvatarURL.Valid {
		avatarURL = &u.AvatarURL.String
	}

	var departmentID *int64
	if u.DepartmentID.Valid {
		departmentID = &u.DepartmentID.Int64
	}

	var teamID *int64
	if u.TeamID.Valid {
		teamID = &u.TeamID.Int64
	}

	var lastLoginAt *time.Time
	if u.LastLoginAt.Valid {
		lastLoginAt = &u.LastLoginAt.Time
	}

	var mobile *string
	if u.Mobile.Valid {
		mobile = &u.Mobile.String
	}
	var feishuOpenID *string
	if u.FeishuOpenID.Valid {
		feishuOpenID = &u.FeishuOpenID.String
	}
	var wecomUserID *string
	if u.WeComUserID.Valid {
		wecomUserID = &u.WeComUserID.String
	}
	var dingtalkUserID *string
	if u.DingTalkUserID.Valid {
		dingtalkUserID = &u.DingTalkUserID.String
	}

	// Keep empty slices non-nil so admin list/detail JSON always emits [] not null.
	roles := make([]string, 0, len(u.AuthCenterRoles))
	roles = append(roles, u.AuthCenterRoles...)
	permissions := make([]string, 0, len(u.AuthCenterPermissions))
	permissions = append(permissions, u.AuthCenterPermissions...)

	return json.Marshal(&struct {
		*Alias
		Email                 *string    `json:"email"`
		Mobile                *string    `json:"mobile"`
		FeishuOpenID          *string    `json:"feishu_open_id"`
		WeComUserID           *string    `json:"wecom_user_id"`
		DingTalkUserID        *string    `json:"dingtalk_user_id"`
		EmailVerifiedAt       *time.Time `json:"email_verified_at"`
		AvatarURL             *string    `json:"avatar_url"`
		DepartmentID          *int64     `json:"department_id"`
		TeamID                *int64     `json:"team_id"`
		LastLoginAt           *time.Time `json:"last_login_at"`
		AuthCenterRoles       []string   `json:"auth_center_roles"`
		AuthCenterPermissions []string   `json:"auth_center_permissions"`
	}{
		Alias:                 (*Alias)(u),
		Email:                 email,
		Mobile:                mobile,
		FeishuOpenID:          feishuOpenID,
		WeComUserID:           wecomUserID,
		DingTalkUserID:        dingtalkUserID,
		EmailVerifiedAt:       emailVerifiedAt,
		AvatarURL:             avatarURL,
		DepartmentID:          departmentID,
		TeamID:                teamID,
		LastLoginAt:           lastLoginAt,
		AuthCenterRoles:       roles,
		AuthCenterPermissions: permissions,
	})
}

// HasPermission 判断用户是否拥有来自认证中心同步的细粒度权限点（支持通配符与别名）
func (u *User) HasPermission(perm string) bool {
	if u == nil {
		return false
	}
	return PrincipalHasPermission(PrincipalFromUser(u), perm)
}

// HasAnyPermission 判断用户是否拥有传入权限点中的任意一个
func (u *User) HasAnyPermission(perms ...string) bool {
	if u == nil || len(perms) == 0 {
		return false
	}
	principal := PrincipalFromUser(u)
	for _, perm := range perms {
		if PrincipalHasPermission(principal, perm) {
			return true
		}
	}
	return false
}

// HasAllPermissions 判断用户是否拥有传入的全部权限点
func (u *User) HasAllPermissions(perms ...string) bool {
	if u == nil || len(perms) == 0 {
		return false
	}
	principal := PrincipalFromUser(u)
	for _, perm := range perms {
		if !PrincipalHasPermission(principal, perm) {
			return false
		}
	}
	return true
}

// HasRole 动态判断用户是否拥有指定角色（支持认证中心下发的动态多角色与本地兼容角色）
func (u *User) HasRole(role string) bool {
	if u == nil {
		return false
	}
	if len(u.AuthCenterRoles) > 0 {
		return PrincipalHasAuthCenterRole(PrincipalFromUser(u), role)
	}
	return AuthCenterRoleMatches(u.Role, role)
}

// HasAnyRole 动态判断用户是否拥有指定角色集合中的任意一个
func (u *User) HasAnyRole(roles ...string) bool {
	if u == nil || len(roles) == 0 {
		return false
	}
	if len(u.AuthCenterRoles) > 0 {
		return PrincipalHasAuthCenterRole(PrincipalFromUser(u), roles...)
	}
	for _, r := range roles {
		if AuthCenterRoleMatches(u.Role, r) {
			return true
		}
	}
	return false
}

// EffectiveRoles 返回用户当前的全部有效角色代码列表（优先以认证中心动态角色为准）
func (u *User) EffectiveRoles() []string {
	if u == nil {
		return nil
	}
	if len(u.AuthCenterRoles) > 0 {
		seen := map[string]struct{}{}
		var result []string
		for _, r := range u.AuthCenterRoles {
			r = strings.TrimSpace(r)
			if r == "" {
				continue
			}
			if _, ok := seen[r]; !ok {
				seen[r] = struct{}{}
				result = append(result, r)
			}
		}
		return result
	}
	if r := strings.TrimSpace(u.Role); r != "" {
		return []string{r}
	}
	return nil
}

// EffectivePermissions 返回用户当前由认证中心下发的全部细粒度权限列表
func (u *User) EffectivePermissions() []string {
	if u == nil {
		return nil
	}
	return append([]string(nil), u.AuthCenterPermissions...)
}

// IsAdmin 是否管理员（优先基于超级管理员角色、全局数据范围或配置写权限判定，兼顾历史单列角色）
func (u *User) IsAdmin() bool {
	if u == nil {
		return false
	}
	if HasAuthCenterSuperRole(u) || u.HasGlobalScope() {
		return true
	}
	return u.Role == RoleSuperAdmin || u.Role == RoleGeneralManager || u.Role == RoleTenantAdmin
}

// HasDepartment 是否有所属部门
func (u *User) HasDepartment() bool {
	return len(u.EffectiveDepartmentIDs()) > 0
}

// HasTeam 是否有所属团队
func (u *User) HasTeam() bool {
	return u.TeamID.Valid && u.TeamID.Int64 > 0
}

// EffectiveDepartmentIDs 返回当前用户可用于数据范围判断的部门投影集合。
func (u *User) EffectiveDepartmentIDs() []int64 {
	if u == nil {
		return nil
	}
	seen := map[int64]struct{}{}
	var ids []int64
	add := func(id int64) {
		if id <= 0 {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	for _, id := range u.DepartmentIDs {
		add(id)
	}
	if u.PrimaryDepartmentID != nil {
		add(*u.PrimaryDepartmentID)
	}
	return ids
}

// HasGlobalScope 是否有全局数据范围（可以看所有数据，优先基于权限与超级角色）
func (u *User) HasGlobalScope() bool {
	if u == nil {
		return false
	}
	return PrincipalHasGlobalDataScope(PrincipalFromUser(u))
}

// HasDepartmentScope 是否有部门级别权限（优先基于权限点与部门管理角色）
func (u *User) HasDepartmentScope() bool {
	if u == nil {
		return false
	}
	if u.HasGlobalScope() {
		return true
	}
	if u.HasAnyPermission(PermStatsReadDept, PermAuditReadDept, PermBusinessUserReadDept, PermOrgWriteDept) {
		return true
	}
	return u.HasRole(RoleDepartmentManager)
}

// HasTeamScope 是否有团队级别权限（优先基于权限点与组长角色）
func (u *User) HasTeamScope() bool {
	if u == nil {
		return false
	}
	if u.HasDepartmentScope() {
		return true
	}
	if u.HasAnyPermission(PermStatsReadTeam, PermAuditReadTeam, PermBusinessUserReadTeam) {
		return true
	}
	return u.HasRole(RoleTeamLeader)
}

// IsActive 是否激活状态
func (u *User) IsActive() bool {
	return u.Status == StatusActive
}

// EffectiveTenantID 返回认证中心投影的租户标识。空值必须由上层严格
// 授权流程拒绝，不能静默伪造默认租户。
func (u *User) EffectiveTenantID() string {
	if u == nil {
		return ""
	}
	return strings.TrimSpace(u.AuthCenterTenantID)
}

// HasEmail 是否已绑定邮箱
func (u *User) HasEmail() bool {
	return u.Email.Valid && u.Email.String != ""
}

// CanManageUsers 是否可以管理用户（基于用户管理权限判定，兼容超管角色）
func (u *User) CanManageUsers() bool {
	if u == nil {
		return false
	}
	if u.IsAdmin() {
		return true
	}
	return u.HasAnyPermission(PermBusinessUserWriteAll, PermBusinessUserWrite, PermOrgWriteAll)
}

// CanManageQuotas 是否可以管理配额（基于配额管理权限判定，兼容部门经理及以上角色）
func (u *User) CanManageQuotas() bool {
	if u == nil {
		return false
	}
	if u.IsAdmin() || u.HasRole(RoleDepartmentManager) {
		return true
	}
	return u.HasAnyPermission(PermQuotaWrite, PermConfigWrite)
}

// CanViewAllStats 是否可以查看所有统计（基于统计全局读取权限判定，兼容部门经理及以上角色）
func (u *User) CanViewAllStats() bool {
	if u == nil {
		return false
	}
	if u.IsAdmin() || u.HasRole(RoleDepartmentManager) {
		return true
	}
	return u.HasPermission(PermStatsReadAll)
}

// 三端架构专属角色判定辅助方法
func (u *User) IsSuperAdmin() bool {
	if u == nil {
		return false
	}
	if HasAuthCenterSuperRole(u) {
		return true
	}
	return len(u.AuthCenterRoles) == 0 && u.Role == RoleSuperAdmin
}

func (u *User) IsTenantAdmin() bool {
	if u == nil {
		return false
	}
	if u.IsSuperAdmin() {
		return true
	}
	if u.HasRole(RoleTenantAdmin) || u.HasRole("tenant.admin") || u.HasRole("enterprise_admin") || u.HasRole("owner") {
		return true
	}
	return len(u.AuthCenterRoles) == 0 && (u.Role == RoleTenantAdmin || u.Role == RoleGeneralManager)
}

// 向后兼容历史单机角色的别名方法（建议新业务代码使用 HasPermission / HasRole 替代）
func (u *User) IsDirector() bool { return u.HasRole(RoleGeneralManager) }
func (u *User) IsManager() bool  { return u.HasRole(RoleDepartmentManager) }
func (u *User) IsTeamLead() bool { return u.HasRole(RoleTeamLeader) }
func (u *User) IsEmployee() bool { return u.HasRole(RoleEmployee) }

// Session 会话模型
type Session struct {
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
	ID        string    `json:"id"`
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
	UserID    int64     `json:"user_id"`
}

// IsExpired 会话是否过期
func (s *Session) IsExpired() bool {
	return timeutil.Now().After(s.ExpiresAt)
}
