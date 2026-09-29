package middleware

import (
	"sort"
	"strings"
	"time"

	platformerr "InkLore/internal/platform/errors"
	"InkLore/internal/platform/logger"
	"InkLore/internal/platform/response"
	"github.com/gin-gonic/gin"
)

// Logger records only bounded routing and transport metadata, never user payloads.
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		method := c.Request.Method
		switch method {
		case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
		default:
			method = "OTHER"
		}
		log := logger.FromContext(c.Request.Context())
		log.Info("http_request_started", "request_id", c.GetString(response.ContextKeyRequestID),
			"domain", requestLogDomain(route), "operation", method+" "+route,
			"method", method, "route", route)
		c.Next()
		// Gin can resolve the final route template after method/OPTIONS handling.
		if matched := c.FullPath(); matched != "" {
			route = matched
		}
		errorCode := c.GetString(response.ContextKeyErrorCode)
		if errorCode == "" {
			errorCode = "none"
		}
		logger.FromContext(c.Request.Context()).InfoContext(c.Request.Context(), "http_request", "request_id", c.GetString(response.ContextKeyRequestID),
			"domain", requestLogDomain(route), "operation", method+" "+route,
			"method", method, "route", route, "status", c.Writer.Status(),
			"error_code", errorCode, "client_ip", c.ClientIP(),
			"duration_ms", time.Since(start).Milliseconds(), "bytes_out", c.Writer.Size())
	}
}

// requestLogDomain accepts only fixed route-template segments, never raw paths.
func requestLogDomain(route string) string {
	parts := strings.Split(route, "/")
	if len(parts) > 2 && parts[1] == "v1" {
		switch parts[2] {
		case "content", "outline", "worldview", "inspiration", "compliance", "agents", "llm":
			return parts[2]
		}
	}
	return "platform"
}

// Recovery disables Gin's request/stack dump, including on broken connections.
func Recovery() gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, _ any) {
		logger.FromContext(c.Request.Context()).Error("http_panic", "request_id", c.GetString(response.ContextKeyRequestID), "route", c.FullPath())
		if c.Writer.Written() {
			c.Abort()
			return
		}
		response.Fail(c, platformerr.Contract(500, "platform.internal_error", nil))
	})
}

// CompleteAllow extends Gin's route-tree result; it never scans paths itself.
func CompleteAllow(c *gin.Context) {
	methods := map[string]bool{"OPTIONS": true}
	for _, method := range strings.Split(c.Writer.Header().Get("Allow"), ",") {
		if method = strings.TrimSpace(method); method != "" {
			methods[method] = true
		}
	}
	values := make([]string, 0, len(methods))
	for method := range methods {
		values = append(values, method)
	}
	sort.Strings(values)
	c.Header("Allow", strings.Join(values, ", "))
}
