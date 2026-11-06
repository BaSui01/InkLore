package middleware

import (
	"context"
	"net/http"

	platformerr "InkLore/internal/platform/errors"
	"InkLore/internal/platform/logger"
	"InkLore/internal/platform/response"
	"InkLore/internal/platform/tracecontext"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Contract establishes independent correlation IDs and never reflects untrusted IDs.
func Contract() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		id, parseErr := uuid.Parse(requestID)
		if parseErr != nil || id.Version() != 4 || id.Variant() != uuid.RFC4122 || id.String() != requestID {
			requestID = uuid.NewString()
		}
		tp, err := tracecontext.ParseTraceparent(c.GetHeader("traceparent"))
		if err != nil {
			tp = tracecontext.NewRootTraceparent()
		}
		c.Set(response.ContextKeyRequestID, requestID)
		c.Set(response.ContextKeyTraceID, tp.TraceID)
		c.Header("X-Request-ID", requestID)
		c.Header("X-Trace-ID", tp.TraceID)
		c.Header("Traceparent", tp.String())
		ctx := context.WithValue(c.Request.Context(), logger.RequestIDKey, requestID)
		ctx = context.WithValue(ctx, logger.TraceIDKey, tp.TraceID)
		c.Request = c.Request.WithContext(tracecontext.WithTraceparent(ctx, tp))
		c.Next()
	}
}

func ContractCORS(origins []string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(origins))
	for _, origin := range origins {
		allowed[origin] = true
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			if !allowed[origin] {
				response.Fail(c, platformerr.Contract(403, "auth.forbidden", nil))
				return
			}
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, X-CSRF-Token, Last-Event-ID, X-Request-ID, traceparent")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Expose-Headers", "X-Request-ID, X-Trace-ID, Traceparent, Idempotent-Replay, Retry-After, Content-Disposition, ETag")
		}
		if c.Request.Method == http.MethodOptions && c.Writer.Header().Get("Allow") != "" {
			CompleteAllow(c)
			response.OK(c, 200, struct{}{})
			c.Abort()
			return
		}
		c.Next()
	}
}
