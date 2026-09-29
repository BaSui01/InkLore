package domain

import (
	"time"
)

// StandardEventType 是引擎对外的唯一事件类型体系（SSE 事件名 = 该类型字符串）。
type StandardEventType string

const (
	StandardEventRunStart             StandardEventType = "run_start"
	StandardEventRunDone              StandardEventType = "run_done"
	StandardEventRunError             StandardEventType = "run_error"
	StandardEventThinkingDelta        StandardEventType = "thinking_delta"
	StandardEventTurnDone             StandardEventType = "turn_done"
	StandardEventTextDelta            StandardEventType = "text_delta"
	StandardEventToolStart            StandardEventType = "tool_start"
	StandardEventToolResult           StandardEventType = "tool_result"
	StandardEventToolError            StandardEventType = "tool_error"
	StandardEventToolApprovalRequired StandardEventType = "tool_approval_required"
	StandardEventToolApprovalApproved StandardEventType = "tool_approval_approved"
	StandardEventToolApprovalRejected StandardEventType = "tool_approval_rejected"
	StandardEventToolApprovalConsumed StandardEventType = "tool_approval_consumed"
	StandardEventTokenUsage           StandardEventType = "token_usage"
	StandardEventContextUsage         StandardEventType = "context_usage"

	StandardEventGraphStart    StandardEventType = "graph_start"
	StandardEventGraphDone     StandardEventType = "graph_done"
	StandardEventNodeStart     StandardEventType = "node_start"
	StandardEventNodeDelta     StandardEventType = "node_delta"
	StandardEventNodeDone      StandardEventType = "node_done"
	StandardEventSubagentStart StandardEventType = "subagent_start"
	StandardEventSubagentDelta StandardEventType = "subagent_delta"
	StandardEventSubagentDone  StandardEventType = "subagent_done"

	// 异步长任务内联桥接事件（与 websocket 广播同构）
	StandardEventTaskProgress  StandardEventType = "task_progress"
	StandardEventTaskPreview   StandardEventType = "task_preview"
	StandardEventTaskCompleted StandardEventType = "task_completed"
	StandardEventTaskFailed    StandardEventType = "task_failed"
)

// AgentEvent 是 SSE 流、事件持久化与事件中间件共用的唯一事件格式。
type AgentEvent struct {
	CreatedAt  time.Time         `json:"created_at"`
	Data       interface{}       `json:"data,omitempty"`
	RunID      *int64            `json:"run_id,omitempty"`
	AgentID    *int64            `json:"agent_id,omitempty"`
	SubagentID *int64            `json:"subagent_id,omitempty"`
	Type       StandardEventType `json:"type"`
	NodeID     string            `json:"node_id,omitempty"`
	Content    string            `json:"content,omitempty"`
	Depth      int               `json:"depth,omitempty"`
	Sequence   int64             `json:"sequence"`
	Turn       int               `json:"turn,omitempty"`
}

// StreamCallback 流式回调（返回 error 可中止 Agent 执行）。
type StreamCallback func(event AgentEvent) error
