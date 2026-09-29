package logger

import (
	"fmt"
	"strings"
)

// DefaultMaxSQLLogLength 慢 SQL 与执行错误 SQL 日志最大截断长度（2KB）
const DefaultMaxSQLLogLength = 2048

// TruncateString 对超长字符串进行截断，保留前后部分并在中间添加省略提示
func TruncateString(s string, limit int) string {
	if limit <= 0 || len(s) <= limit {
		return s
	}
	if limit < 20 {
		return s[:limit] + "..."
	}
	headLen := limit * 2 / 3
	tailLen := limit - headLen
	return fmt.Sprintf("%s ...[TRUNCATED total_bytes=%d]... %s", s[:headLen], len(s), s[len(s)-tailLen:])
}

// TruncateSQLLog 对 SQL 语句执行脱敏并进行长度安全截断，防止数兆 Base64 / 巨大 Payload 灌入日志与 Telemetry
func TruncateSQLLog(sql string) string {
	sanitized := SanitizeLog(strings.TrimSpace(sql))
	if len(sanitized) <= DefaultMaxSQLLogLength {
		return sanitized
	}
	headLen := 1024
	tailLen := 512
	if len(sanitized) <= headLen+tailLen {
		return sanitized
	}
	return fmt.Sprintf("%s ...[SQL TRUNCATED total_bytes=%d]... %s", sanitized[:headLen], len(sanitized), sanitized[len(sanitized)-tailLen:])
}
