package sqlscope

import (
	"context"
	"errors"
	"strings"

	"InkLore/internal/platform/tenantcontext"
)

// 本文件收纳各业务域 infrastructure 共用的 SQL 值转换与租户作用域助手。
//
// 存在理由：ADR-0014 拆分为 7 域后，每个域的 PostgreSQL 仓储都需要同一套
// 「空串 <-> NULL」「缺租户退化」语义。若各自复制一份，7 份实现必然漂移。
// 因此实现收敛到 platform（不按业务域拆分的微内核基座）。

// DefaultTenantID 是无租户上下文时使用的显式默认租户。
const DefaultTenantID = tenantcontext.DefaultTenantID

// ErrDatabaseUnavailable 表示仓储已装配但底层数据库句柄为空。
//
// 它必须是一个可判定的哨兵错误，而不是 nil 解引用：装配正确性由组合根保证，
// 运行时若出现说明配置与装配不一致，应显式报错而非静默降级。
var ErrDatabaseUnavailable = errors.New("数据库未初始化")

// TenantFromContext 从请求上下文取租户；缺失时退化为 DefaultTenantID。
func TenantFromContext(ctx context.Context) string {
	return tenantcontext.OrDefault(ctx)
}

// NullableString 把空串写成 NULL，用于可空外键列。
func NullableString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// StringOrEmpty 把可空列还原为领域层的空串语义（空串 = 未归属）。
func StringOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// CountWords 按 Unicode 码点统计字数。
func CountWords(content string) int {
	return len([]rune(content))
}
