package domain

import (
	"context"
)

// ListQuery parameters for keyset pagination.
type ListQuery struct {
	Limit      int
	BeforeTime string
	BeforeID   string
}

// GovernanceRepository persists definitions, workflows, tools, and MCP servers.
// Tenant isolation is strictly enforced across all operations.
type GovernanceRepository interface {
	// Definitions
	GetDefinition(ctx context.Context, tenantID, id string) (*AgentDefinition, *DefinitionVersion, error)
	// GetDefinitionVersion reads a frozen version even after its head is retired.
	GetDefinitionVersion(ctx context.Context, tenantID, versionID string) (*DefinitionVersion, error)
	GetDefinitionByKey(ctx context.Context, tenantID, key string) (*AgentDefinition, *DefinitionVersion, error)
	ListDefinitions(ctx context.Context, tenantID string, query ListQuery) ([]DefinitionView, *string, bool, error)
	CreateDefinition(ctx context.Context, tenantID string, def *AgentDefinition, ver *DefinitionVersion) error
	UpdateDefinition(ctx context.Context, tenantID string, def *AgentDefinition, ver *DefinitionVersion, expectedRev int64) error
	DeleteDefinition(ctx context.Context, tenantID, id string, expectedRev int64) error
	ListCatalogDefinitions(ctx context.Context, tenantID string, query ListQuery) ([]CatalogDefinition, *string, bool, error)

	// Workflows
	GetWorkflow(ctx context.Context, tenantID, id string) (*AgentWorkflow, *WorkflowVersion, error)
	GetWorkflowByKey(ctx context.Context, tenantID, key string) (*AgentWorkflow, *WorkflowVersion, error)
	ListWorkflows(ctx context.Context, tenantID string, query ListQuery) ([]WorkflowView, *string, bool, error)
	CreateWorkflow(ctx context.Context, tenantID string, wf *AgentWorkflow, ver *WorkflowVersion) error
	UpdateWorkflow(ctx context.Context, tenantID string, wf *AgentWorkflow, ver *WorkflowVersion, expectedRev int64) error
	DeleteWorkflow(ctx context.Context, tenantID, id string, expectedRev int64) error
	ListCatalogWorkflows(ctx context.Context, tenantID string, query ListQuery) ([]CatalogWorkflow, *string, bool, error)

	// Tools
	GetTool(ctx context.Context, tenantID, id string) (*AgentTool, *ToolVersion, error)
	// GetToolVersion resolves the exact frozen version, never the mutable current version.
	GetToolVersion(ctx context.Context, tenantID, versionID string) (*AgentTool, *ToolVersion, error)
	GetToolByKey(ctx context.Context, tenantID, key string) (*AgentTool, *ToolVersion, error)
	ListTools(ctx context.Context, tenantID string, query ListQuery) ([]ToolView, *string, bool, error)
	CreateTool(ctx context.Context, tenantID string, tool *AgentTool, ver *ToolVersion) error
	UpdateTool(ctx context.Context, tenantID string, tool *AgentTool, ver *ToolVersion, expectedRev int64) error
	DeleteTool(ctx context.Context, tenantID, id string, expectedRev int64) error
	IsToolInUse(ctx context.Context, tenantID, id string) (bool, error)
	CheckToolVersionsExist(ctx context.Context, tenantID string, versionIDs []string) (bool, error)

	// MCP Servers
	GetMCPServer(ctx context.Context, tenantID, id string) (*MCPServer, error)
	ListMCPServers(ctx context.Context, tenantID string, query ListQuery) ([]MCPServer, *string, bool, error)
	CreateMCPServer(ctx context.Context, tenantID string, srv *MCPServer) error
	UpdateMCPServer(ctx context.Context, tenantID string, srv *MCPServer, expectedRev int64) error
	DeleteMCPServer(ctx context.Context, tenantID, id string, expectedRev int64) error
	IsMCPServerInUse(ctx context.Context, tenantID, id string) (bool, error)
}

// MCPDeploymentAllowlistPort validates external MCP endpoint URLs and environment references against deployment policies.
type MCPDeploymentAllowlistPort interface {
	ValidateEndpoint(endpoint string) error
	ValidateEnvRef(envRef string) error
}
