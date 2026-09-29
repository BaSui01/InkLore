package logger

import (
	"errors"
	"regexp"
	"strings"
)

// 脱敏替换规则
var replacementRules = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{regexp.MustCompile(`(?i)(password|passwd|pwd|secret)[\s]*[=:]\s*["']?[^"'\s&]+["']?`), "${1}=[REDACTED]"},
	{regexp.MustCompile(`(?i)(api[_-]?key|apikey|api[_-]?secret)[\s]*[=:]\s*["']?[^"'\s&]+["']?`), "${1}=[REDACTED]"},
	{regexp.MustCompile(`(?i)(token|auth[_-]?token|access[_-]?token|refresh[_-]?token)[\s]*[=:]\s*["']?[^"'\s&]+["']?`), "${1}=[REDACTED]"},
	{regexp.MustCompile(`(?i)(session[_-]?id|sessionid|sid)[\s]*[=:]\s*["']?[^"'\s&]+["']?`), "${1}=[REDACTED]"},
	{regexp.MustCompile(`(?i)authorization[\s]*:[\s]*(bearer|basic)\s+[^\s]+`), "Authorization: ${1} [REDACTED]"},
	{regexp.MustCompile(`sk-[a-zA-Z0-9]{48}`), "sk-[REDACTED]"},
	{regexp.MustCompile(`sk-[a-zA-Z0-9_-]{4,}`), "sk-[REDACTED]"},
	{regexp.MustCompile(`AIza[a-zA-Z0-9_-]{35}`), "AIza[REDACTED]"},
	{regexp.MustCompile(`sk-ant-[a-zA-Z0-9_-]{95}`), "sk-ant-[REDACTED]"},
	{regexp.MustCompile(`(?i)SQLSTATE\s+[0-9A-Z]{5}`), "[REDACTED]"},
}

// SanitizeLog 脱敏日志内容
// 移除或替换日志中的敏感信息
func SanitizeLog(msg string) string {
	result := msg

	for _, rule := range replacementRules {
		result = rule.pattern.ReplaceAllString(result, rule.replacement)
	}

	return result
}

// SanitizeValue 对任意 key/value 执行脱敏
func SanitizeValue(key string, val any) any {
	if str, ok := val.(string); ok {
		return SanitizeLog(str)
	}
	return val
}

// SanitizeURL 脱敏 URL 中的敏感参数
func SanitizeURL(url string) string {
	// 处理查询参数
	sensitiveParams := []string{
		"password", "passwd", "pwd",
		"token", "access_token", "refresh_token",
		"api_key", "apikey", "key",
		"secret", "session_id", "sid",
	}

	result := url
	for _, param := range sensitiveParams {
		// 匹配 param=value 格式
		pattern := regexp.MustCompile(`(?i)([?&])` + param + `=([^&]*)`)
		result = pattern.ReplaceAllString(result, "${1}"+param+"=[REDACTED]")
	}

	return result
}

// SanitizeError 脱敏错误信息
func SanitizeError(err error) string {
	if err == nil {
		return ""
	}
	return SanitizeLog(err.Error())
}

var (
	// urlErrorPrefixRegex 匹配 Go http.Client 的 url.Error 模式，如:
	// Post "https://example.com/v1/images/edits":
	// send request: Post "https://example.com/v1/images/edits":
	urlErrorPrefixRegex = regexp.MustCompile(`(?i)(?:(?:failed\s+to\s+)?send\s+request:\s*)?(?:(?:post|get|put|delete|patch|options|head)\s+)?"https?://[^"]+"\s*:\s*`)

	// httpURLRegex 匹配任意裸露的 http/https URL
	httpURLRegex = regexp.MustCompile(`https?://[^\s"',;]+`)

	// sendRequestPrefixRegex 匹配裸露的 send request: / read response: / net/http: 前缀
	technicalPrefixRegex = regexp.MustCompile(`(?i)(?:(?:failed\s+to\s+)?send\s+request:\s*|read\s+response:\s*|net/http:\s*|dial\s+tcp\s+[^\s:]*:\s*)`)

	// redundantColonsRegex 匹配多余的冒号和空格
	redundantColonsRegex = regexp.MustCompile(`:\s*(?::\s*)+`)
)

// SanitizeTransportErrorMessage 将底层网络/HTTP请求错误转换为对用户友好的错误信息，
// 并彻底剥除 URL、密钥和底层堆栈信息。
func SanitizeTransportErrorMessage(msg string) string {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return ""
	}

	// 1. 基础凭据脱敏（密码、密钥、Token等）
	msg = SanitizeLog(msg)

	// 2. 剥离 Go http.Client 的 URL 前缀（如 Post "https://...": ）
	msg = urlErrorPrefixRegex.ReplaceAllString(msg, "")

	// 3. 剥离底层技术前缀（如 send request: / net/http: / dial tcp ...: ）
	msg = technicalPrefixRegex.ReplaceAllString(msg, "")

	// 4. 剥离剩余裸露的 URL
	msg = httpURLRegex.ReplaceAllString(msg, "[上游服务]")

	// 5. 检查并识别常见的网络与底层传输错误签名，转换为中文友好提示
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "tls handshake timeout"):
		msg = regexp.MustCompile(`(?i)tls handshake timeout`).ReplaceAllString(msg, "上游服务连接握手超时 (TLS timeout)")
	case strings.Contains(lower, "timeout awaiting response headers") ||
		strings.Contains(lower, "client.timeout exceeded while awaiting headers") ||
		strings.Contains(lower, "context deadline exceeded") ||
		strings.Contains(lower, "request timed out"):
		timeoutRe := regexp.MustCompile(`(?i)(?:timeout awaiting response headers|client\.timeout exceeded while awaiting headers|context deadline exceeded|request timed out)(?:\s*\([^)]*\))?`)
		msg = timeoutRe.ReplaceAllString(msg, "上游服务响应超时 (Request timeout)")
	case strings.Contains(lower, "connect: connection refused") || strings.Contains(lower, "connection refused"):
		refusedRe := regexp.MustCompile(`(?i)(?:connect:\s*)?connection refused`)
		msg = refusedRe.ReplaceAllString(msg, "上游服务拒绝连接 (Connection refused)")
	case strings.Contains(lower, "no such host") || strings.Contains(lower, "dial tcp: lookup") || strings.Contains(lower, "lookup "):
		hostRe := regexp.MustCompile(`(?i)(?:lookup\s+[^\s:]+:\s*)?no such host`)
		msg = hostRe.ReplaceAllString(msg, "上游服务地址解析失败 (DNS lookup failed)")
	case strings.Contains(lower, "connection reset by peer"):
		msg = regexp.MustCompile(`(?i)(?:read:\s*)?connection reset by peer`).ReplaceAllString(msg, "上游服务连接重置 (Connection reset)")
	case strings.Contains(lower, "broken pipe"):
		msg = regexp.MustCompile(`(?i)broken pipe`).ReplaceAllString(msg, "网络连接中断 (Broken pipe)")
	case strings.Contains(lower, "unexpected eof") || strings.Contains(lower, "read: unexpected eof"):
		msg = regexp.MustCompile(`(?i)(?:read:\s*)?unexpected eof`).ReplaceAllString(msg, "上游服务连接异常断开 (Unexpected EOF)")
	case strings.Contains(lower, "i/o timeout"):
		msg = regexp.MustCompile(`(?i)i/o timeout`).ReplaceAllString(msg, "网络传输超时 (I/O timeout)")
	case strings.Contains(lower, "certificate has expired") || strings.Contains(lower, "x509: certificate"):
		msg = regexp.MustCompile(`(?i)(?:x509:\s*certificate[^\n,;]*|certificate has expired[^\n,;]*)`).ReplaceAllString(msg, "上游服务 SSL 证书异常")
	}

	// 6. 再次清理可能残留的前缀与标点
	msg = technicalPrefixRegex.ReplaceAllString(msg, "")
	msg = redundantColonsRegex.ReplaceAllString(msg, ": ")
	msg = strings.TrimSpace(msg)
	msg = strings.TrimPrefix(msg, ":")
	msg = strings.TrimSpace(msg)

	return msg
}

// SanitizeUserFacingError 将错误转换为脱敏且对用户友好的 error
func SanitizeUserFacingError(err error) error {
	if err == nil {
		return nil
	}
	cleanMsg := SanitizeTransportErrorMessage(err.Error())
	if cleanMsg == "" {
		return err
	}
	return errors.New(cleanMsg)
}
