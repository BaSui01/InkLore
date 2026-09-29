package authdomain

// UserListOptions 描述用户列表查询条件。
//
// 这是用户域的筛选值对象，不携带数据库映射或查询实现细节；持久化层只负责将它翻译成 SQL 条件。
type UserListOptions struct {
	TenantID      string
	UserID        int64
	DepartmentID  *int64
	TeamID        *int64
	Role          string
	Status        string
	Search        string
	DepartmentIDs []int64
	Page          int
	PageSize      int
	NoDepartment  bool
}
