package engineloop

import (
	"InkLore/internal/platform/timeutil"
	"encoding/json"
	"sync/atomic"
	"time"

	enginedomain "InkLore/internal/agent/domain"
	ai "InkLore/internal/llm/domain"
)

// StandardEventType 引用 domain 唯一事件类型契约
type StandardEventType = enginedomain.StandardEventType

const (
	StandardEventRunStart             = enginedomain.StandardEventRunStart
	StandardEventRunDone              = enginedomain.StandardEventRunDone
	StandardEventRunError             = enginedomain.StandardEventRunError
	StandardEventThinkingDelta        = enginedomain.StandardEventThinkingDelta
	StandardEventTurnDone             = enginedomain.StandardEventTurnDone
	StandardEventTextDelta            = enginedomain.StandardEventTextDelta
	StandardEventToolStart            = enginedomain.StandardEventToolStart
	StandardEventToolResult           = enginedomain.StandardEventToolResult
	StandardEventToolError            = enginedomain.StandardEventToolError
	StandardEventToolApprovalRequired = enginedomain.StandardEventToolApprovalRequired
	StandardEventToolApprovalApproved = enginedomain.StandardEventToolApprovalApproved
	StandardEventToolApprovalRejected = enginedomain.StandardEventToolApprovalRejected
	StandardEventToolApprovalConsumed = enginedomain.StandardEventToolApprovalConsumed
	StandardEventTokenUsage           = enginedomain.StandardEventTokenUsage
	StandardEventContextUsage         = enginedomain.StandardEventContextUsage

	StandardEventGraphStart    = enginedomain.StandardEventGraphStart
	StandardEventGraphDone     = enginedomain.StandardEventGraphDone
	StandardEventNodeStart     = enginedomain.StandardEventNodeStart
	StandardEventNodeDelta     = enginedomain.StandardEventNodeDelta
	StandardEventNodeDone      = enginedomain.StandardEventNodeDone
	StandardEventSubagentStart = enginedomain.StandardEventSubagentStart
	StandardEventSubagentDelta = enginedomain.StandardEventSubagentDelta
	StandardEventSubagentDone  = enginedomain.StandardEventSubagentDone

	// 异步长任务内联桥接事件（与 websocket 广播同构）
	StandardEventTaskProgress  = enginedomain.StandardEventTaskProgress
	StandardEventTaskPreview   = enginedomain.StandardEventTaskPreview
	StandardEventTaskCompleted = enginedomain.StandardEventTaskCompleted
	StandardEventTaskFailed    = enginedomain.StandardEventTaskFailed
)

// AgentEvent 是 SSE 流、事件持久化与事件中间件共用的唯一事件格式。
type AgentEvent = enginedomain.AgentEvent

// eventSequence 进程内单调递增的事件序号，用于流内排序与去重。
var eventSequence atomic.Int64

func nextEventSequence() int64 {
	return eventSequence.Add(1)
}

// StampEvent 补全事件的序号与时间戳（已设置的不覆盖）。
func StampEvent(event *AgentEvent) {
	if event == nil {
		return
	}
	if event.Sequence == 0 {
		event.Sequence = nextEventSequence()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = timeutil.Now().UTC()
	}
}

// ===== 引擎事件的强类型 Data payload =====

// ThinkingDeltaData thinking_delta 事件载荷；Delta 为本次增量（不再携带累计全文）。
type ThinkingDeltaData struct {
	Delta         string `json:"delta,omitempty"`
	FallbackModel string `json:"fallback_model,omitempty"`
	Turn          int    `json:"turn"`
	Attempt       int    `json:"attempt,omitempty"`
	Retry         bool   `json:"retry,omitempty"`
	Fallback      bool   `json:"fallback,omitempty"`
}

// TurnDoneData turn_done 事件载荷（单轮思考结束，含本轮用量）。
type TurnDoneData struct {
	Usage        *ai.TokenUsage `json:"usage,omitempty"`
	Content      string         `json:"content,omitempty"`
	FinishReason string         `json:"finish_reason,omitempty"`
	Turn         int            `json:"turn"`
	ToolCalls    int            `json:"tool_calls"`
}

// ToolStartData tool_start / tool_approval_consumed 事件载荷。
type ToolStartData struct {
	Approval   map[string]interface{} `json:"approval,omitempty"`
	Tool       string                 `json:"tool"`
	ToolCallID string                 `json:"tool_call_id,omitempty"`
	Arguments  string                 `json:"arguments,omitempty"`
}

// ToolResultData tool_result / tool_error 事件载荷。
type ToolResultData struct {
	Output        *ToolOutput     `json:"output,omitempty"`
	ToolResult    *ToolCallResult `json:"tool_result,omitempty"`
	ErrorContract *ErrorContract  `json:"error_contract,omitempty"`
	Tool          string          `json:"tool"`
	ToolCallID    string          `json:"tool_call_id,omitempty"`
	Result        string          `json:"result,omitempty"`
	Error         string          `json:"error,omitempty"`
}

// ApprovalRequiredData tool_approval_required 事件载荷。
type ApprovalRequiredData struct {
	Tool       string          `json:"tool"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	Status     string          `json:"status"`
	Result     string          `json:"result,omitempty"`
	ExpiresAt  string          `json:"expires_at,omitempty"`
	Arguments  json.RawMessage `json:"arguments,omitempty"`
	ApprovalID int64           `json:"approval_id,omitempty"`
}

// TaskProgressData 对应 task_progress / task_preview 事件载荷
type TaskProgressData struct {
	TaskID       int64     `json:"task_id"`
	WorkflowID   int64     `json:"workflow_id,omitempty"`
	RunID        int64     `json:"run_id,omitempty"`
	Progress     int       `json:"progress"`
	Status       string    `json:"status"`
	Phase        string    `json:"phase,omitempty"`
	Message      string    `json:"message,omitempty"`
	ImageURL     string    `json:"image_url,omitempty"`
	ThumbnailURL string    `json:"thumbnail_url,omitempty"`
	Index        int       `json:"index,omitempty"`
	Total        int       `json:"total,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

// TaskCompletedData 对应 task_completed 事件载荷
type TaskCompletedData struct {
	TaskID       int64     `json:"task_id"`
	WorkflowID   int64     `json:"workflow_id,omitempty"`
	RunID        int64     `json:"run_id,omitempty"`
	Status       string    `json:"status"`
	ImageURL     string    `json:"image_url"`
	ThumbnailURL string    `json:"thumbnail_url,omitempty"`
	DurationMs   int64     `json:"duration_ms,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

// TaskFailedData 对应 task_failed 事件载荷
type TaskFailedData struct {
	TaskID     int64     `json:"task_id"`
	WorkflowID int64     `json:"workflow_id,omitempty"`
	RunID      int64     `json:"run_id,omitempty"`
	Status     string    `json:"status"`
	Error      string    `json:"error"`
	ErrorCode  string    `json:"error_code,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}
