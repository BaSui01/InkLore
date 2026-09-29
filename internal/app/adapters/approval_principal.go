package adapters

import (
	"context"

	approvalcontrol "InkLore/internal/agent/domain/approvalcontrol"
	engineapproval "InkLore/internal/agent/runtime/application/approval"
	platformauth "InkLore/internal/platform/auth"
	"InkLore/internal/platform/logger"
)

// NewApprovalPrincipalResolver 把平台认证主体映射为审批域主体（agent 域与 platform 域的桥接点）。
//
// 审批结论必须由「是谁在请求」决定，因此这里只信任请求上下文中的认证主体：
//
//   - 上下文里有主体：要求主体 UserID 与审批归属一致，角色照搬、绝不放大小于已有的权限；
//   - 上下文里没有主体且认证非强制（开发档位）：按审批来源放行并告警，保证本地可用；
//   - 上下文里没有主体且认证强制：明确拒绝。
//
// 关于权限：当前部署没有权限中心，平台令牌就是操作员凭证，主体不带权限时会补上通配权限，
// 使「本人审批本人的工具调用」成立；审批的意义在于把人留在环路里（模型无法自行拿到审批
// 令牌），而不是 RBAC。将来接入权限中心后，应改为直接使用主体自带权限并去掉这层兜底。
func NewApprovalPrincipalResolver(allowUnauthenticated bool) engineapproval.ApprovalPrincipalResolver {
	return engineapproval.ApprovalPrincipalResolverFunc(func(ctx context.Context, userID int64) (approvalcontrol.Principal, error) {
		if userID <= 0 {
			return approvalcontrol.Principal{}, engineapproval.ErrAgentApprovalForbidden
		}
		principal, ok := platformauth.PrincipalFromContext(ctx)
		if !ok {
			if !allowUnauthenticated {
				return approvalcontrol.Principal{}, engineapproval.ErrAgentApprovalForbidden
			}
			logger.WarnContext(ctx, "请求上下文缺少认证主体，开发档位按审批主体放行", "user_id", userID)
			return approvalcontrol.Principal{
				UserID:      userID,
				Source:      approvalcontrol.PrincipalSourceApproval,
				Permissions: []string{platformauth.AuthCenterPermissionWildcard},
			}, nil
		}
		if principal.UserID > 0 && principal.UserID != userID {
			return approvalcontrol.Principal{}, engineapproval.ErrAgentApprovalForbidden
		}
		permissions := append([]string(nil), principal.Permissions...)
		if len(permissions) == 0 {
			permissions = []string{platformauth.AuthCenterPermissionWildcard}
		}
		return approvalcontrol.Principal{
			User:        principal.User,
			UserID:      userID,
			Roles:       append([]string(nil), principal.Roles...),
			Permissions: permissions,
			Source:      approvalcontrol.PrincipalSourceAuthCenter,
		}, nil
	})
}
