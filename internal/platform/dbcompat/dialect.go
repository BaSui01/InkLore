package dbcompat

import (
	"net/url"
	"regexp"
	"strings"
	"sync/atomic"
)

const (
	DriverSQLite   = "sqlite"
	DriverPostgres = "postgres"
)

// 全局当前驱动，由 app 初始化时设置。
// 默认保留 SQLite 仅服务未显式初始化 App 的单元测试夹具；运行配置的空 driver
// 通过 NormalizeDriver 收敛到 PostgreSQL。
var currentDriver atomic.Value

func init() {
	currentDriver.Store(DriverSQLite)
}

// SetDriver 设置全局数据库驱动（在 app.initDatabase 中调用）
func SetDriver(driver string) {
	currentDriver.Store(NormalizeDriver(driver))
}

// CurrentDriver 返回规范化后的当前驱动名称
func CurrentDriver() string {
	v, _ := currentDriver.Load().(string)
	return v
}

// NormalizeDriver maps accepted aliases to the canonical driver name used by
// the application. Empty runtime config uses PostgreSQL; SQLite must be
// requested explicitly by tests or temporary validation tools.
func NormalizeDriver(driver string) string {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "":
		return DriverPostgres
	case "sqlite", "sqlite3":
		return DriverSQLite
	case "postgres", "postgresql", "pgsql", "pg":
		return DriverPostgres
	default:
		return strings.ToLower(strings.TrimSpace(driver))
	}
}

func IsSQLite(driver string) bool {
	return NormalizeDriver(driver) == DriverSQLite
}

func IsPostgres(driver string) bool {
	return NormalizeDriver(driver) == DriverPostgres
}

var dsnTimeZonePattern = regexp.MustCompile(`(?i)(^|\s)timezone\s*=\s*(?:'[^']*'|"[^"]*"|\S+)`)

// EnsureDSNTimeZone normalizes a PostgreSQL DSN to the requested session
// timezone. The application stores instants in TIMESTAMPTZ and must not inherit
// a machine-specific session setting from an existing configuration string.
func EnsureDSNTimeZone(dsn, timezone string) string {
	trimmed := strings.TrimSpace(dsn)
	if trimmed == "" || strings.TrimSpace(timezone) == "" {
		return dsn
	}
	if strings.HasPrefix(trimmed, "postgres://") || strings.HasPrefix(trimmed, "postgresql://") {
		parsed, err := url.Parse(trimmed)
		if err == nil {
			query := parsed.Query()
			for key := range query {
				if strings.EqualFold(key, "timezone") {
					query.Del(key)
				}
			}
			query.Set("TimeZone", timezone)
			parsed.RawQuery = query.Encode()
			return parsed.String()
		}
	}
	if dsnTimeZonePattern.MatchString(trimmed) {
		return strings.TrimSpace(dsnTimeZonePattern.ReplaceAllString(trimmed, "${1}TimeZone="+timezone))
	}
	return trimmed + " TimeZone=" + timezone
}
