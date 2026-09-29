package authdomain

import "errors"

// 这些错误是 Auth 领域跨 Application、Persistence 和 Transport 传递的稳定契约。
// 具体存储实现只能通过别名暴露它们，避免同一业务错误因包边界不同而出现多个身份。
var (
	ErrUserNotFound           = errors.New("用户不存在")
	ErrDepartmentNotFound     = errors.New("部门不存在")
	ErrTeamNotFound           = errors.New("团队不存在")
	ErrOrgProjectionInvalid   = errors.New("组织投影数据无效")
	ErrOrgSyncRunNotFound     = errors.New("组织同步记录不存在")
	ErrOrgManagedByAuthCenter = errors.New("组织架构由用户中心统一维护")
)
