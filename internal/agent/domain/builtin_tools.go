package domain

import "encoding/json"

// BuiltinTool describes an installed Go handler. Its metadata is compile-time;
// ExecutionStatus is evaluated against the requesting tenant's current version.
type BuiltinTool struct {
	ToolKey         string          `json:"tool_key"`
	Title           string          `json:"title"`
	Description     string          `json:"description"`
	ExecutionStatus string          `json:"execution_status"`
	SideEffect      string          `json:"side_effect"`
	InputSchema     json.RawMessage `json:"input_schema"`
	OutputSchema    json.RawMessage `json:"output_schema"`
}

// BuiltinToolCatalogPort exposes only capabilities with an installed handler.
type BuiltinToolCatalogPort interface {
	BuiltinTools() []BuiltinTool
}
