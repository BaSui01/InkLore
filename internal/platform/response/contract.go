package response

import (
	"errors"
	"net/http"
	"strconv"

	platformerr "InkLore/internal/platform/errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const ContextKeyRequestID = "request_id"
const ContextKeyTraceID = "x_trace_id"
const ContextKeyErrorCode = "error_code"

type SuccessEnvelope struct {
	Code      string `json:"code"`
	Data      any    `json:"data"`
	RequestID string `json:"request_id"`
	TraceID   string `json:"trace_id"`
}
type ErrorEnvelope struct {
	Code       string         `json:"code"`
	MessageKey string         `json:"message_key"`
	Details    map[string]any `json:"details"`
	RequestID  string         `json:"request_id"`
	TraceID    string         `json:"trace_id"`
}

func IDs(c *gin.Context) (string, string) {
	requestID := c.GetString(ContextKeyRequestID)
	traceID := c.GetString(ContextKeyTraceID)
	if requestID == "" {
		requestID = uuid.NewString()
		c.Set(ContextKeyRequestID, requestID)
	}
	if traceID == "" {
		traceID = uuid.New().String()
		traceID = traceID[0:8] + traceID[9:13] + traceID[14:18] + traceID[19:23] + traceID[24:]
		c.Set(ContextKeyTraceID, traceID)
	}
	return requestID, traceID
}
func OK(c *gin.Context, status int, data any) {
	requestID, traceID := IDs(c)
	c.JSON(status, SuccessEnvelope{Code: "ok", Data: data, RequestID: requestID, TraceID: traceID})
}
func FailureBody(err error, requestID, traceID string) (int, ErrorEnvelope) {
	status, code := http.StatusInternalServerError, "platform.internal_error"
	details := map[string]any{}
	var appErr *platformerr.AppError
	if errors.As(err, &appErr) && platformerr.IsContractStatus(appErr.Code, appErr.HTTPStatus) {
		status, code = appErr.HTTPStatus, appErr.Code
		details = platformerr.SafeDetails(appErr.Details)
	}
	return status, ErrorEnvelope{Code: code, MessageKey: "errors." + code, Details: details, RequestID: requestID, TraceID: traceID}
}
func Fail(c *gin.Context, err error) {
	requestID, traceID := IDs(c)
	status, body := FailureBody(err, requestID, traceID)
	c.Set(ContextKeyErrorCode, body.Code)
	if seconds, ok := body.Details["retry_after_seconds"].(int64); ok {
		c.Header("Retry-After", strconv.FormatInt(seconds, 10))
	}
	if body.Code == "platform.operation_in_progress" {
		c.Header("Retry-After", "1")
	}
	c.AbortWithStatusJSON(status, body)
}
