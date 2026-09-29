package domain

import (
	"context"
	"encoding/json"
	"time"

	runtimedomain "InkLore/internal/llm/domain"
)

// ToolHandler 工具处理函数（纯契约）
type ToolHandler func(ctx context.Context, args json.RawMessage) (string, error)

// ToolEntry 注册的工具项（包含定义与处理器）
type ToolEntry struct {
	Definition runtimedomain.Tool
	Handler    ToolHandler
}

// Tool 统一的 Agent 工具领域接口契约。
type Tool interface {
	Name() string
	Description() string
	Parameters() any
	PromptGuidelines() string
	Capability() ToolCapability
	Execute(ctx context.Context, rc *RuntimeContext, rawArgs json.RawMessage) (*ToolResult, error)
}

// ToolCapability 工具能力与执行策略配置
type ToolCapability struct {
	Name                string        `json:"name"`
	Type                string        `json:"type"`            // "builtin" | "business" | "mcp" | "subagent"
	RiskLevel           string        `json:"risk_level"`      // "low" | "medium" | "high" | "destructive"
	ApprovalPolicy      string        `json:"approval_policy"` // "never" | "ask_risk" | "always"
	AuditLevel          string        `json:"audit_level"`     // "none" | "summary" | "full"
	SideEffectLevel     string        `json:"side_effect_level,omitempty"`
	RequiredPermissions []string      `json:"required_permissions,omitempty"`
	RequiredActions     []string      `json:"required_actions,omitempty"`
	Timeout             time.Duration `json:"timeout,omitempty"`
	Parallelizable      bool          `json:"parallelizable,omitempty"`
	SuspendsRun         bool          `json:"suspends_run,omitempty"`
	ExemptFromApproval  bool          `json:"exempt_from_approval,omitempty"`
}

// ToolResult 工具执行输出
type ToolResult struct {
	ToolName    string         `json:"tool_name"`
	ToolCallID  string         `json:"tool_call_id"`
	Content     string         `json:"content"`
	Raw         any            `json:"raw,omitempty"`
	Structured  map[string]any `json:"structured,omitempty"`
	Artifacts   []any          `json:"artifacts,omitempty"`
	IsError     bool           `json:"is_error,omitempty"`
	Suspended   bool           `json:"suspended,omitempty"`
	ExecutionMS int64          `json:"execution_ms,omitempty"`
}

func (r *ToolResult) String() string {
	if r == nil {
		return ""
	}
	return r.Content
}

type ToolOutputStatus string

const (
	ToolOutputStatusOK    ToolOutputStatus = "ok"
	ToolOutputStatusError ToolOutputStatus = "error"
)

type ToolOutput struct {
	Metadata  map[string]any   `json:"metadata,omitempty"`
	Status    ToolOutputStatus `json:"status"`
	Text      string           `json:"text,omitempty"`
	Blocks    []ContentBlock   `json:"blocks,omitempty"`
	Artifacts []ArtifactBlock  `json:"artifacts,omitempty"`
}

type ContentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Name     string `json:"name,omitempty"`
	MIMEType string `json:"mime_type,omitempty"`
	URL      string `json:"url,omitempty"`
	Size     int64  `json:"size_bytes,omitempty"`
}

type ArtifactBlock struct {
	Type     string `json:"type"`
	Name     string `json:"name,omitempty"`
	MIMEType string `json:"mime_type,omitempty"`
	URL      string `json:"url,omitempty"`
	Key      string `json:"key,omitempty"`
	Data     []byte `json:"-"`
	Size     int64  `json:"size_bytes,omitempty"`
}
