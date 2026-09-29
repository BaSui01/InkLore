package domain

import "context"

type ToolParam struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Default     any    `json:"default,omitempty"`
}

type ToolDescriptor struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Category    string      `json:"category"`
	Parameters  []ToolParam `json:"parameters"`
}

type ToolResult struct {
	ToolID      string   `json:"tool_id"`
	Success     bool     `json:"success"`
	Score       int      `json:"score,omitempty"` // 0 to 100
	Summary     string   `json:"summary"`
	Details     any      `json:"details"`
	Suggestions []string `json:"suggestions,omitempty"`
}

type Tool interface {
	Descriptor() ToolDescriptor
	Execute(ctx context.Context, input map[string]any) (ToolResult, error)
}
