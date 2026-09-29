package enginetooling

import (
	agentdomain "InkLore/internal/agent/domain"
)

const (
	PermissionAlwaysAsk  = "always_ask"
	PermissionFullAccess = "full_access"

	permissionAlwaysAsk  = PermissionAlwaysAsk
	permissionFullAccess = PermissionFullAccess
)

type ToolOutputStatus = agentdomain.ToolOutputStatus

const (
	ToolOutputStatusOK    = agentdomain.ToolOutputStatusOK
	ToolOutputStatusError = agentdomain.ToolOutputStatusError
)

type ToolOutput = agentdomain.ToolOutput
type ContentBlock = agentdomain.ContentBlock
type ArtifactBlock = agentdomain.ArtifactBlock
type ToolEntry = agentdomain.ToolEntry
type Tool = agentdomain.Tool
