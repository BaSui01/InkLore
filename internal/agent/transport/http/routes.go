package http

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers all agent governance and catalog endpoints on the router.
func (h *Handler) RegisterRoutes(r gin.IRoutes) {
	r.POST("/v1/agents/novel-guide", h.startNovelGuide)
	r.GET("/v1/agents/novel-guide/:session_id", h.getNovelGuide)
	r.POST("/v1/agents/novel-guide/:session_id/answers", h.answerNovelGuide)
	// Definitions
	r.GET("/v1/agents/definitions", h.listDefinitions)
	r.POST("/v1/agents/definitions", h.createDefinition)
	r.GET("/v1/agents/definitions/:definition_id", h.getDefinition)
	r.PUT("/v1/agents/definitions/:definition_id", h.updateDefinition)
	r.DELETE("/v1/agents/definitions/:definition_id", h.deleteDefinition)

	// Workflows
	r.GET("/v1/agents/workflows", h.listWorkflows)
	r.POST("/v1/agents/workflows", h.createWorkflow)
	r.GET("/v1/agents/workflows/:workflow_id", h.getWorkflow)
	r.PUT("/v1/agents/workflows/:workflow_id", h.updateWorkflow)
	r.DELETE("/v1/agents/workflows/:workflow_id", h.deleteWorkflow)

	// Tools: the built-in catalog is distinct from tenant-scoped frozen versions.
	r.GET("/v1/agents/builtin-tools", h.listBuiltinTools)
	r.GET("/v1/agents/tools", h.listTools)
	r.GET("/v1/agents/tools/:tool_id", h.getTool)

	// MCP Servers
	r.GET("/v1/agents/mcp-servers", h.listMCPServers)
	r.POST("/v1/agents/mcp-servers", h.createMCPServer)
	r.GET("/v1/agents/mcp-servers/:mcp_server_id", h.getMCPServer)
	r.PUT("/v1/agents/mcp-servers/:mcp_server_id", h.updateMCPServer)
	r.DELETE("/v1/agents/mcp-servers/:mcp_server_id", h.deleteMCPServer)

	// Creator Catalog
	r.GET("/v1/agents/catalog/definitions", h.catalogDefinitions)
	r.GET("/v1/agents/catalog/workflows", h.catalogWorkflows)
	r.GET("/v1/agents/catalog/task-policies", h.catalogTaskPolicies)
	r.GET("/v1/agents/audit/runs", h.listAuditRuns)
	r.GET("/v1/agents/audit/runs/:run_id", h.getAuditRun)

	// Runs
	r.POST("/v1/agents/runs", h.createRun)
	r.GET("/v1/agents/runs", h.listRuns)
	r.GET("/v1/agents/runs/:run_id", h.getRun)
	r.GET("/v1/agents/runs/:run_id/questions/:question_id", h.getQuestion)
	r.POST("/v1/agents/runs/:run_id/questions/:question_id/answer", h.answerQuestion)
	r.POST("/v1/agents/runs/:run_id/cancel", h.cancelRun)
	r.GET("/v1/agents/runs/:run_id/events", h.subscribeEvents)
	r.GET("/v1/agents/runs/:run_id/artifacts", h.listArtifacts)

	// Artifacts
	r.GET("/v1/agents/artifacts/:artifact_id", h.getArtifact)

	// Approvals
	r.GET("/v1/agents/approvals", h.listApprovals)
	r.GET("/v1/agents/approvals/:approval_id", h.getApproval)
	r.POST("/v1/agents/approvals/:approval_id/approve", h.approveApproval)
	r.POST("/v1/agents/approvals/:approval_id/reject", h.rejectApproval)
}
