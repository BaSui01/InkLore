package tenantcontext

import (
	"context"
	"errors"
	"strings"
)

type Scope struct{ ID string }

type contextKey struct{}
type legacyUnscopedContextKey struct{}

var ErrRequired = errors.New("trusted tenant context is required")
var ErrMismatch = errors.New("tenant context cannot change resource ownership")

// WithTenant 只接受已认证主体或持久化资源恢复的租户；统一载体避免 HTTP、
// 后台任务与 ORM 各自绑定后无法相互识别。绑定本身不授予业务访问权限。
func WithTenant(ctx context.Context, tenantID string) (context.Context, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	id := strings.TrimSpace(tenantID)
	if id == "" {
		return ctx, ErrRequired
	}
	if current := GetTenantID(ctx); current != "" && current != id {
		return ctx, ErrMismatch
	}
	return context.WithValue(ctx, contextKey{}, Scope{ID: id}), nil
}

func RequireTenant(ctx context.Context) (Scope, error) {
	if ctx == nil {
		return Scope{}, ErrRequired
	}
	scope, ok := ctx.Value(contextKey{}).(Scope)
	if !ok || strings.TrimSpace(scope.ID) == "" {
		return Scope{}, ErrRequired
	}
	return scope, nil
}

func GetTenantID(ctx context.Context) string {
	if scope, err := RequireTenant(ctx); err == nil {
		return scope.ID
	}
	return ""
}

// WithLegacyUnscoped 仅供已废弃的单租户构造兼容路径使用。
// HTTP、后台业务和正式 App 装配不得调用；新写入必须使用可信租户。
func WithLegacyUnscoped(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, legacyUnscopedContextKey{}, true)
}

func IsLegacyUnscoped(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	value, _ := ctx.Value(legacyUnscopedContextKey{}).(bool)
	return value
}

// DefaultTenantID 是无租户上下文时使用的显式默认租户。
//
// 它存在的意义不是"允许没有租户"，而是让**尚未改造完成**的写入路径有确定的归属：
// 既不会写空值，也不会因为 NOT NULL 直接失败，从而可以在后续任务里逐表补齐作用域。
const DefaultTenantID = "default"

// OrDefault 返回请求上下文的租户；缺失时退化为 DefaultTenantID。
func OrDefault(ctx context.Context) string {
	if tenant := strings.TrimSpace(GetTenantID(ctx)); tenant != "" {
		return tenant
	}
	return DefaultTenantID
}
