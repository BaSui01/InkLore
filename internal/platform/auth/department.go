package authdomain

import (
	"InkLore/internal/platform/nulltype"
	"time"
)

type DataScopeFilter struct {
	DepartmentIDs []int64 `json:"-"`
	// TeamID 限制到同一团队成员；与 DepartmentIDs / UserID 互斥使用（优先 UserID）。
	TeamID int64 `json:"-"`
	UserID int64 `json:"-"`
}

// Department 部门模型
type Department struct {
	UpdatedAt              time.Time      `json:"updated_at"`
	CreatedAt              time.Time      `json:"created_at"`
	SyncedAt               *time.Time     `json:"synced_at,omitempty"`
	Manager                *User          `json:"manager,omitempty"`
	Description            string         `json:"description"`
	DisplayName            string         `json:"display_name"`
	Status                 string         `json:"status"`
	AuthCenterDepartmentID string         `json:"auth_center_department_id,omitempty"`
	AuthCenterParentID     string         `json:"auth_center_parent_id,omitempty"`
	Source                 string         `json:"source,omitempty"`
	SyncStatus             string         `json:"sync_status,omitempty"`
	ParentName             string         `json:"parent_name,omitempty"`
	RawSnapshot            string         `json:"-"`
	ManagerName            string         `json:"manager_name,omitempty"`
	Name                   string         `json:"name"`
	ParentID               nulltype.Int64 `json:"parent_id"`
	ManagerID              nulltype.Int64 `json:"manager_id"`
	ID                     int64          `json:"id"`
	UserCount              int            `json:"user_count,omitempty"`
	TeamCount              int            `json:"team_count,omitempty"`
}

// Team 团队/小组模型
type Team struct {
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	TeamLead       *User          `json:"team_lead,omitempty"`
	Department     *Department    `json:"department,omitempty"`
	Status         string         `json:"status"`
	Description    string         `json:"description"`
	DisplayName    string         `json:"display_name"`
	Name           string         `json:"name"`
	DepartmentName string         `json:"department_name,omitempty"`
	TeamLeadID     nulltype.Int64 `json:"team_lead_id"`
	ID             int64          `json:"id"`
	DepartmentID   int64          `json:"department_id"`
	MemberCount    int            `json:"member_count,omitempty"`
}

// UserDepartmentMembership 是用户中心部门成员关系在本地的只读投影。
type UserDepartmentMembership struct {
	UpdatedAt              time.Time  `json:"updated_at"`
	CreatedAt              time.Time  `json:"created_at"`
	SyncedAt               *time.Time `json:"synced_at,omitempty"`
	AuthCenterDepartmentID string     `json:"auth_center_department_id"`
	Role                   string     `json:"role"`
	Status                 string     `json:"status"`
	AuthCenterUserID       string     `json:"auth_center_user_id"`
	Username               string     `json:"username,omitempty"`
	DisplayName            string     `json:"display_name,omitempty"`
	Email                  string     `json:"email,omitempty"`
	DepartmentName         string     `json:"department_name,omitempty"`
	DepartmentDisplayName  string     `json:"department_display_name,omitempty"`
	ID                     int64      `json:"id"`
	DepartmentID           int64      `json:"department_id"`
	UserID                 int64      `json:"user_id"`
	IsPrimary              bool       `json:"is_primary"`
}

// OrgProjectionMembershipKey 标识用户中心中的一条部门成员关系。
type OrgProjectionMembershipKey struct {
	AuthCenterUserID       string
	AuthCenterDepartmentID string
}

// AuditLog 审计日志模型
type AuditLog struct {
	CreatedAt    time.Time `json:"created_at"`
	User         *User     `json:"user,omitempty"`
	NewValue     string    `json:"new_value"`
	ResourceType string    `json:"resource_type"`
	ResourceID   string    `json:"resource_id"`
	OldValue     string    `json:"old_value"`
	IPAddress    string    `json:"ip_address"`
	UserAgent    string    `json:"user_agent"`
	Result       string    `json:"result"`
	ErrorMessage string    `json:"error_message"`
	Action       string    `json:"action"`
	Username     string    `json:"username,omitempty"`
	UserRole     string    `json:"user_role,omitempty"`
	ID           int64     `json:"id"`
	UserID       int64     `json:"user_id"`
}

// IsActive 部门是否激活
func (d *Department) IsActive() bool {
	return d.Status == StatusActive
}

// HasParent 部门是否有父部门
func (d *Department) HasParent() bool {
	return d.ParentID.Valid && d.ParentID.Int64 > 0
}

// HasManager 部门是否有经理
func (d *Department) HasManager() bool {
	return d.ManagerID.Valid && d.ManagerID.Int64 > 0
}

// IsActive 团队是否激活
func (t *Team) IsActive() bool {
	return t.Status == StatusActive
}

// HasTeamLead 团队是否有组长
func (t *Team) HasTeamLead() bool {
	return t.TeamLeadID.Valid && t.TeamLeadID.Int64 > 0
}

// IsSuccess 操作是否成功
func (a *AuditLog) IsSuccess() bool {
	return a.Result == "success"
}

// GetActionDisplay 获取操作类型的显示名称
func (a *AuditLog) GetActionDisplay() string {
	actionMap := map[string]string{
		"create_user":       "创建用户",
		"update_user":       "更新用户",
		"delete_user":       "删除用户",
		"create_department": "创建部门",
		"update_department": "更新部门",
		"delete_department": "删除部门",
		"create_team":       "创建团队",
		"update_team":       "更新团队",
		"delete_team":       "删除团队",
		"set_quota":         "设置配额",
		"reset_quota":       "重置配额",
		"update_role":       "修改角色",
		"assign_department": "分配部门",
		"assign_team":       "分配团队",
	}

	if display, ok := actionMap[a.Action]; ok {
		return display
	}
	return a.Action
}

// GetResourceTypeDisplay 获取资源类型的显示名称
func (a *AuditLog) GetResourceTypeDisplay() string {
	typeMap := map[string]string{
		"user":       "用户",
		"department": "部门",
		"team":       "团队",
		"workflow":   "工作流",
		"quota":      "配额",
		"system":     "系统",
	}

	if display, ok := typeMap[a.ResourceType]; ok {
		return display
	}
	return a.ResourceType
}

// DepartmentStats 部门统计信息
type DepartmentStats struct {
	DepartmentName string `json:"department_name"`
	DepartmentID   int64  `json:"department_id"`
	UserCount      int    `json:"user_count"`
	TeamCount      int    `json:"team_count"`
	ManagerCount   int    `json:"manager_count"`
	TeamLeadCount  int    `json:"team_lead_count"`
	EmployeeCount  int    `json:"employee_count"`
	ActiveUsers    int    `json:"active_users"`
	DisabledUsers  int    `json:"disabled_users"`
}

// TeamStats 团队统计信息
type TeamStats struct {
	TeamName        string `json:"team_name"`
	DepartmentName  string `json:"department_name"`
	TeamID          int64  `json:"team_id"`
	DepartmentID    int64  `json:"department_id"`
	MemberCount     int    `json:"member_count"`
	ActiveMembers   int    `json:"active_members"`
	DisabledMembers int    `json:"disabled_members"`
}

// DepartmentTree 部门树节点（用于前端展示层级结构）
type DepartmentTree struct {
	*Department
	Children []*DepartmentTree `json:"children,omitempty"`
}

// AuditLogFilter 审计日志查询过滤器
type AuditLogFilter struct {
	// TenantID 租户企业过滤：为空表示平台全局全量，非空则限定归属于该租户下的用户操作记录
	TenantID     string           `json:"tenant_id,omitempty"`
	UserID       *int64           `json:"user_id,omitempty"`
	Scope        *DataScopeFilter `json:"-"`
	ResourceID   string           `json:"resource_id,omitempty"`
	Result       string           `json:"result,omitempty"`
	StartTime    string           `json:"start_time,omitempty"`
	EndTime      string           `json:"end_time,omitempty"`
	Username     string           `json:"username,omitempty"`
	Action       []string         `json:"action,omitempty"`
	ResourceType []string         `json:"resource_type,omitempty"`
	Limit        int              `json:"limit,omitempty"`
	Offset       int              `json:"offset,omitempty"`
}
