// Package domain contains domain models and invariants for agent governance.
package domain

import (
	"encoding/json"
	"time"
)

type TaskKind string

const (
	TaskKindChapterDraft       TaskKind = "chapter_draft"
	TaskKindChapterRewrite     TaskKind = "chapter_rewrite"
	TaskKindOutlineSuggest     TaskKind = "outline_suggest"
	TaskKindWorldviewSuggest   TaskKind = "worldview_suggest"
	TaskKindInspirationAnalyze TaskKind = "inspiration_analyze"
	TaskKindComplianceAudit    TaskKind = "compliance_audit"
	TaskKindComplianceRewrite  TaskKind = "compliance_rewrite"
)

type NodeRole string

const (
	RoleSupervisor NodeRole = "supervisor"
	RoleStructure  NodeRole = "structure"
	RoleWriter     NodeRole = "writer"
	RolePolisher   NodeRole = "polisher"
	RoleReviewer   NodeRole = "reviewer"
	RoleTool       NodeRole = "tool"
)

type SideEffect string

const (
	SideEffectReadOnly      SideEffect = "read_only"
	SideEffectTransactional SideEffect = "transactional"
	SideEffectExternal      SideEffect = "external"
)

type PublicationStatus string

const (
	PublicationStatusDraft     PublicationStatus = "draft"
	PublicationStatusPublished PublicationStatus = "published"
)

// AgentDefinition is the mutable head for an agent definition.
type AgentDefinition struct {
	ID               string     `json:"id"`
	TenantID         string     `json:"tenant_id"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	Revision         int64      `json:"revision"`
	DefinitionKey    string     `json:"definition_key"`
	Title            string     `json:"title"`
	Enabled          bool       `json:"enabled"`
	CurrentVersionID *string    `json:"current_version_id"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty"`
}

// DefinitionVersion is an immutable published or draft version of an agent definition.
type DefinitionVersion struct {
	ID                string    `json:"id"`
	TenantID          string    `json:"tenant_id"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	DefinitionID      string    `json:"definition_id"`
	Version           int       `json:"version"`
	SystemInstruction string    `json:"system_instruction"`
	ToolVersionIDs    []string  `json:"tool_version_ids"`
	TaskKinds         []string  `json:"task_kinds"`
}

// DefinitionView represents the public view conforming to API Definition schema.
type DefinitionView struct {
	ID                string            `json:"id"`
	Revision          int64             `json:"revision"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
	DefinitionKey     string            `json:"definition_key"`
	Title             string            `json:"title"`
	Enabled           bool              `json:"enabled"`
	SystemInstruction string            `json:"system_instruction"`
	ToolVersionIDs    []string          `json:"tool_version_ids"`
	TaskKinds         []string          `json:"task_kinds"`
	CurrentVersionID  string            `json:"current_version_id"`
	Version           int               `json:"version"`
	PublicationStatus PublicationStatus `json:"publication_status"`
}

// AgentWorkflow is the mutable head for a workflow.
type AgentWorkflow struct {
	ID               string     `json:"id"`
	TenantID         string     `json:"tenant_id"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	Revision         int64      `json:"revision"`
	WorkflowKey      string     `json:"workflow_key"`
	Title            string     `json:"title"`
	Enabled          bool       `json:"enabled"`
	CurrentVersionID *string    `json:"current_version_id"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty"`
}

// Binding maps data from an upstream node into a target parameter.
type Binding struct {
	TargetParameter string `json:"target_parameter"`
	SourceNode      string `json:"source_node"`
	SourcePointer   string `json:"source_pointer"`
}

// ManifestNode defines a single step/node inside a workflow DAG.
type ManifestNode struct {
	NodeKey             string    `json:"node_key"`
	Role                string    `json:"role"`
	DependsOn           []string  `json:"depends_on"`
	Required            bool      `json:"required"`
	DefinitionVersionID *string   `json:"definition_version_id,omitempty"`
	ToolVersionID       *string   `json:"tool_version_id,omitempty"`
	InputBindings       []Binding `json:"input_bindings"`
}

// WorkflowManifest represents the workflow graph definition.
type WorkflowManifest struct {
	Nodes []ManifestNode `json:"nodes"`
}

// WorkflowVersion is an immutable version of a workflow.
type WorkflowVersion struct {
	ID           string           `json:"id"`
	TenantID     string           `json:"tenant_id"`
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
	WorkflowID   string           `json:"workflow_id"`
	Version      int              `json:"version"`
	Manifest     WorkflowManifest `json:"manifest"`
	ManifestHash string           `json:"manifest_hash"`
}

// WorkflowView represents the public view conforming to API Workflow schema.
type WorkflowView struct {
	ID               string           `json:"id"`
	Revision         int64            `json:"revision"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
	WorkflowKey      string           `json:"workflow_key"`
	Title            string           `json:"title"`
	Enabled          bool             `json:"enabled"`
	Manifest         WorkflowManifest `json:"manifest"`
	CurrentVersionID string           `json:"current_version_id"`
	Version          int              `json:"version"`
	ManifestHash     string           `json:"manifest_hash"`
}

// AgentTool is the mutable head for a tool.
type AgentTool struct {
	ID               string     `json:"id"`
	TenantID         string     `json:"tenant_id"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	Revision         int64      `json:"revision"`
	ToolKey          string     `json:"tool_key"`
	Title            string     `json:"title"`
	Enabled          bool       `json:"enabled"`
	CurrentVersionID *string    `json:"current_version_id"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty"`
}

// ToolVersion is an immutable version of a tool.
type ToolVersion struct {
	ID               string          `json:"id"`
	TenantID         string          `json:"tenant_id"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
	ToolID           string          `json:"tool_id"`
	Version          int             `json:"version"`
	InputSchema      json.RawMessage `json:"input_schema"`
	OutputSchema     json.RawMessage `json:"output_schema"`
	SideEffect       string          `json:"side_effect"`
	ApprovalRequired bool            `json:"approval_required"`
	TimeoutSeconds   int             `json:"timeout_seconds"`
	MCPServerID      *string         `json:"mcp_server_id,omitempty"`
	RemoteToolName   *string         `json:"remote_tool_name,omitempty"`
}

// ToolView represents the public view conforming to API Tool schema.
type ToolView struct {
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
	MCPServerID      *string         `json:"mcp_server_id"`
	RemoteToolName   *string         `json:"remote_tool_name"`
	ID               string          `json:"id"`
	ToolKey          string          `json:"tool_key"`
	Title            string          `json:"title"`
	SideEffect       string          `json:"side_effect"`
	CurrentVersionID string          `json:"current_version_id"`
	InputSchema      json.RawMessage `json:"input_schema"`
	OutputSchema     json.RawMessage `json:"output_schema"`
	Revision         int64           `json:"revision"`
	TimeoutSeconds   int             `json:"timeout_seconds"`
	Version          int             `json:"version"`
	Enabled          bool            `json:"enabled"`
	// ExecutionStatus is a live runtime projection, not a persisted configuration flag.
	// "not_bound" means no executable worker/handler is attached to this tool.
	ExecutionStatus  string `json:"execution_status"`
	ApprovalRequired bool   `json:"approval_required"`
}

// ToolExecutionNotBound denotes absence of a compatible, live worker capability.
const (
	ToolExecutionNotBound  = "not_bound"
	ToolExecutionAvailable = "available"
)

// MCPServer represents a registered MCP endpoint.
type MCPServer struct {
	ID             string     `json:"id"`
	TenantID       string     `json:"tenant_id"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	Revision       int64      `json:"revision"`
	Title          string     `json:"title"`
	Endpoint       string     `json:"endpoint"`
	EnvRef         string     `json:"env_ref"`
	AllowedMethods []string   `json:"allowed_methods"`
	Enabled        bool       `json:"enabled"`
	DeletedAt      *time.Time `json:"deleted_at,omitempty"`
}

// MCPServerView conforms to API MCPServer schema.
type MCPServerView struct {
	ID             string    `json:"id"`
	Revision       int64     `json:"revision"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Title          string    `json:"title"`
	Endpoint       string    `json:"endpoint"`
	EnvRef         string    `json:"env_ref"`
	AllowedMethods []string  `json:"allowed_methods"`
	Enabled        bool      `json:"enabled"`
}

// CatalogDefinition represents creator-safe definition metadata.
type CatalogDefinition struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	VersionID string   `json:"version_id"`
	TaskKinds []string `json:"task_kinds"`
}

// CatalogWorkflow represents creator-safe workflow metadata.
type CatalogWorkflow struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	VersionID string `json:"version_id"`
}

// Deleted confirms deletion of a resource.
type Deleted struct {
	ID       string `json:"id"`
	Revision int64  `json:"revision"`
	Deleted  bool   `json:"deleted"`
}
