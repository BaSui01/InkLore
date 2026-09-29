package engineloop

import "strings"

const (
	ErrorCodeInvalidRequest   = "invalid_request"
	ErrorCodePermissionDenied = "permission_denied"
	ErrorCodeInternalError    = "internal_error"
	ErrorCodeRetryableError   = "retryable_error"

	ErrorReasonExecutionFailed       = "execution_failed"
	ErrorReasonModelMiddlewareFailed = "model_middleware_failed"
	ErrorReasonModelStreamFailed     = "model_stream_failed"
	ErrorReasonToolDisabled          = "tool_disabled"
	ErrorReasonToolUnknown           = "tool_unknown"
	ErrorReasonToolPermissionDenied  = "tool_permission_denied"
	ErrorReasonToolMiddlewareFailed  = "tool_middleware_failed"
	ErrorReasonToolExecutionFailed   = "tool_execution_failed"
	ErrorReasonToolInvalidArguments  = "tool_invalid_arguments"
	ErrorReasonAgentGraphInvalid     = "agent_graph_invalid"
	ErrorReasonAgentGraphFailed      = "agent_graph_failed"
	ErrorReasonAgentMainNodeFailed   = "agent_main_node_failed"
	ErrorReasonAgentSubagentFailed   = "agent_subagent_failed"
)

type ErrorContract struct {
	Code     string `json:"code"`
	Category string `json:"category"`
	Message  string `json:"message"`
}

func NewErrorContract(category, message, code string) ErrorContract {
	category = strings.TrimSpace(category)
	if category == "" {
		category = ErrorCodeInternalError
	}
	message = strings.TrimSpace(message)
	if message == "" {
		message = "Agent 运行失败"
	}
	code = strings.TrimSpace(code)
	if code == "" {
		code = ErrorReasonExecutionFailed
	}
	return ErrorContract{
		Code:     code,
		Category: category,
		Message:  message,
	}
}

func WithErrorContract(data map[string]interface{}, code, message, reason string) map[string]interface{} {
	if data == nil {
		data = map[string]interface{}{}
	}
	contract := NewErrorContract(code, message, reason)
	data["error_contract"] = contract
	if _, exists := data["error"]; !exists {
		data["error"] = contract
	}
	if _, exists := data["error_message"]; !exists {
		data["error_message"] = contract.Message
	}
	return data
}

func ErrorStreamEvent(message, code, reason string, data map[string]interface{}) AgentEvent {
	data = WithErrorContract(data, code, message, reason)
	return AgentEvent{
		Type:    StandardEventRunError,
		Content: NewErrorContract(code, message, reason).Message,
		Data:    data,
	}
}
