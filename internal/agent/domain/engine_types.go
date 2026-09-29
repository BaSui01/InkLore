package domain

import "time"

type Definition struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	SystemPrompt string   `json:"system_prompt"`
	Model        string   `json:"model"`
	Temperature  float64  `json:"temperature"`
	Tools        []string `json:"tools,omitempty"`
}

type RunInput struct {
	Definition Definition     `json:"definition"`
	RequestID  string         `json:"request_id"`
	UserInput  string         `json:"user_input"`
	Context    map[string]any `json:"context,omitempty"`
	// UserID 是发起本次运行的认证用户，也是工具审批单据的归属主体。
	// 为 0 表示未认证：需要审批的工具无法建单，只能被拒绝（fail-closed）。
	UserID int64 `json:"user_id,omitempty"`
	// ApprovalTokens 是用户批准后回传的审批令牌；循环消费成功后才会真正执行工具。
	ApprovalTokens []string `json:"-"`
	// PermissionLevel 控制审批严格度：always_ask 时所有工具都需要审批。
	PermissionLevel string `json:"permission_level,omitempty"`
	// SessionID / AgentID 用于把审批单关联回会话与 Agent，便于按来源检索。
	SessionID *int64 `json:"-"`
	AgentID   *int64 `json:"-"`
}

type TraceStepType string

const (
	StepThink       TraceStepType = "think"
	StepToolCall    TraceStepType = "tool_call"
	StepObservation TraceStepType = "observation"
	StepEmit        TraceStepType = "emit"
)

type TraceStep struct {
	Type      TraceStepType  `json:"type"`
	Content   string         `json:"content"`
	Timestamp time.Time      `json:"timestamp"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type RunResult struct {
	Content    string      `json:"content"`
	Model      string      `json:"model"`
	TraceSteps []TraceStep `json:"trace_steps,omitempty"`
}
