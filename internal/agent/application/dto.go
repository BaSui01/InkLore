package application

import (
	d "InkLore/internal/agent/domain"
	"encoding/json"
)

// DefinitionInput represents creation payload for agent definitions.
type DefinitionInput struct {
	DefinitionKey     string   `json:"definition_key"`
	Title             string   `json:"title"`
	Enabled           bool     `json:"enabled"`
	SystemInstruction string   `json:"system_instruction"`
	ToolVersionIDs    []string `json:"tool_version_ids"`
	TaskKinds         []string `json:"task_kinds"`
}

// DefinitionUpdate represents update payload for agent definitions.
type DefinitionUpdate struct {
	DefinitionKey     string   `json:"definition_key"`
	Title             string   `json:"title"`
	Enabled           bool     `json:"enabled"`
	SystemInstruction string   `json:"system_instruction"`
	ToolVersionIDs    []string `json:"tool_version_ids"`
	TaskKinds         []string `json:"task_kinds"`
	ExpectedRevision  int64    `json:"expected_revision"`
}

// DefinitionPage wraps a paginated list of definitions.
type DefinitionPage struct {
	Items      []d.DefinitionView `json:"items"`
	NextCursor *string            `json:"next_cursor"`
	HasMore    bool               `json:"has_more"`
}

// WorkflowInput represents creation payload for workflows.
type WorkflowInput struct {
	WorkflowKey string             `json:"workflow_key"`
	Title       string             `json:"title"`
	Enabled     bool               `json:"enabled"`
	Manifest    d.WorkflowManifest `json:"manifest"`
}

// WorkflowUpdate represents update payload for workflows.
type WorkflowUpdate struct {
	WorkflowKey      string             `json:"workflow_key"`
	Title            string             `json:"title"`
	Enabled          bool               `json:"enabled"`
	Manifest         d.WorkflowManifest `json:"manifest"`
	ExpectedRevision int64              `json:"expected_revision"`
}

// WorkflowPage wraps a paginated list of workflows.
type WorkflowPage struct {
	Items      []d.WorkflowView `json:"items"`
	NextCursor *string          `json:"next_cursor"`
	HasMore    bool             `json:"has_more"`
}

// ToolInput represents creation payload for tools.
type ToolInput struct {
	MCPServerID      *string         `json:"mcp_server_id"`
	RemoteToolName   *string         `json:"remote_tool_name"`
	ToolKey          string          `json:"tool_key"`
	Title            string          `json:"title"`
	SideEffect       string          `json:"side_effect"`
	InputSchema      json.RawMessage `json:"input_schema"`
	OutputSchema     json.RawMessage `json:"output_schema"`
	TimeoutSeconds   int             `json:"timeout_seconds"`
	Enabled          bool            `json:"enabled"`
	ApprovalRequired bool            `json:"approval_required"`
}

// ToolUpdate represents update payload for tools.
type ToolUpdate struct {
	MCPServerID      *string         `json:"mcp_server_id"`
	RemoteToolName   *string         `json:"remote_tool_name"`
	ToolKey          string          `json:"tool_key"`
	Title            string          `json:"title"`
	SideEffect       string          `json:"side_effect"`
	InputSchema      json.RawMessage `json:"input_schema"`
	OutputSchema     json.RawMessage `json:"output_schema"`
	TimeoutSeconds   int             `json:"timeout_seconds"`
	ExpectedRevision int64           `json:"expected_revision"`
	Enabled          bool            `json:"enabled"`
	ApprovalRequired bool            `json:"approval_required"`
}

// ToolPage wraps a paginated list of tools.
type ToolPage struct {
	Items      []d.ToolView `json:"items"`
	NextCursor *string      `json:"next_cursor"`
	HasMore    bool         `json:"has_more"`
}

// BuiltinToolPage is a read-only projection of installed handlers, not tenant tools.
type BuiltinToolPage struct {
	Items      []d.BuiltinTool `json:"items"`
	NextCursor *string         `json:"next_cursor"`
	HasMore    bool            `json:"has_more"`
}

// MCPServerInput represents creation payload for MCP servers.
type MCPServerInput struct {
	Title          string   `json:"title"`
	Endpoint       string   `json:"endpoint"`
	EnvRef         string   `json:"env_ref"`
	AllowedMethods []string `json:"allowed_methods"`
	Enabled        bool     `json:"enabled"`
}

// MCPServerUpdate represents update payload for MCP servers.
type MCPServerUpdate struct {
	Title            string   `json:"title"`
	Endpoint         string   `json:"endpoint"`
	EnvRef           string   `json:"env_ref"`
	AllowedMethods   []string `json:"allowed_methods"`
	Enabled          bool     `json:"enabled"`
	ExpectedRevision int64    `json:"expected_revision"`
}

// MCPServerPage wraps a paginated list of MCP servers.
type MCPServerPage struct {
	Items      []d.MCPServerView `json:"items"`
	NextCursor *string           `json:"next_cursor"`
	HasMore    bool              `json:"has_more"`
}

// CatalogDefinitionPage wraps a paginated list of creator-safe definitions.
type CatalogDefinitionPage struct {
	Items      []d.CatalogDefinition `json:"items"`
	NextCursor *string               `json:"next_cursor"`
	HasMore    bool                  `json:"has_more"`
}

// CatalogWorkflowPage wraps a paginated list of creator-safe workflows.
type CatalogWorkflowPage struct {
	Items      []d.CatalogWorkflow `json:"items"`
	NextCursor *string             `json:"next_cursor"`
	HasMore    bool                `json:"has_more"`
}
