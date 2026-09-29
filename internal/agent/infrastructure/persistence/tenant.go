package enginepersistence

import (
	"context"

	"InkLore/internal/platform/tenantcontext"
)

// DefaultTenantID 是无租户上下文时使用的显式默认租户。
const DefaultTenantID = tenantcontext.DefaultTenantID

// TenantFromContext 取请求上下文的租户；缺失时退化为 DefaultTenantID。
//
// 与作品域仓储、agentrun 记录器保持同一语义：缺租户不报错也不落空值，
// 而是归入显式默认租户，避免存量数据无归属、也避免写入被 NOT NULL 拦下。
func TenantFromContext(ctx context.Context) string {
	return tenantcontext.OrDefault(ctx)
}
