package authdomain

import "context"

// principalCtxKey 是认证主体在请求上下文中的键。
type principalCtxKey struct{}

// WithPrincipal 把已认证主体写入请求上下文。
//
// 存在的意义：gin.Context 只在中转层可见，而域内的授权判定（例如 Agent 工具审批的单据
// 归属与审批权限校验）只能拿到 context.Context。middleware.Auth 会同时写入两处，
// 保证传输层与域层对「当前是谁」的判断一致。
func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	if ctx == nil {
		return ctx
	}
	return context.WithValue(ctx, principalCtxKey{}, principal)
}

// PrincipalFromContext 取出请求上下文中的认证主体。
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	if ctx == nil {
		return Principal{}, false
	}
	principal, ok := ctx.Value(principalCtxKey{}).(Principal)
	return principal, ok
}
