package engineloop

import (
	enginedomain "InkLore/internal/agent/domain"
	engineapproval "InkLore/internal/agent/runtime/application/approval"
	enginepromptctx "InkLore/internal/agent/runtime/application/promptctx"
	enginetooling "InkLore/internal/agent/runtime/application/tooling"
)

// Approval types
type ToolApprovalService = engineapproval.ToolApprovalService
type ToolApprovalConsumption = engineapproval.ToolApprovalConsumption
type ToolApprovalConsumptionService = engineapproval.ToolApprovalConsumptionService
type ToolApprovalGrant = engineapproval.ToolApprovalGrant
type ToolApprovalCreate = engineapproval.ToolApprovalCreate

var (
	ApprovalConsumptionData = engineapproval.ApprovalConsumptionData
	WithApprovedTool        = engineapproval.WithApprovedTool
)

// Tooling types
type ToolCapability = enginetooling.ToolCapability
type ToolCapabilityRegistry = enginetooling.ToolCapabilityRegistry
type ToolPermissionChecker = enginetooling.ToolPermissionChecker
type ToolOutput = enginedomain.ToolOutput
type ContentBlock = enginedomain.ContentBlock
type ArtifactBlock = enginedomain.ArtifactBlock
type ToolOutputStatus = enginedomain.ToolOutputStatus

const (
	ToolOutputStatusOK    = enginedomain.ToolOutputStatusOK
	ToolOutputStatusError = enginedomain.ToolOutputStatusError

	ApprovalNever   = enginetooling.ApprovalNever
	ApprovalAskRisk = enginetooling.ApprovalAskRisk
	ApprovalAlways  = enginetooling.ApprovalAlways

	CapabilityRiskLow         = enginetooling.CapabilityRiskLow
	CapabilityRiskMedium      = enginetooling.CapabilityRiskMedium
	CapabilityRiskHigh        = enginetooling.CapabilityRiskHigh
	CapabilityRiskDestructive = enginetooling.CapabilityRiskDestructive

	AuditNone    = enginetooling.AuditNone
	AuditSummary = enginetooling.AuditSummary
	AuditFull    = enginetooling.AuditFull

	ToolSideEffectReadonly       = enginetooling.ToolSideEffectReadonly
	ToolSideEffectCreatesTask    = enginetooling.ToolSideEffectCreatesTask
	ToolSideEffectWritesArtifact = enginetooling.ToolSideEffectWritesArtifact
	ToolSideEffectMutatesState   = enginetooling.ToolSideEffectMutatesState
	ToolSideEffectExternal       = enginetooling.ToolSideEffectExternal
)

var (
	DefaultToolCapabilities      = enginetooling.DefaultToolCapabilities
	NewToolCapabilityRegistry    = enginetooling.NewToolCapabilityRegistry
	NormalizeToolOutputWithLimit = enginetooling.NormalizeToolOutputWithLimit
	NormalizeToolOutput          = enginetooling.NormalizeToolOutput
	ValidateToolArguments        = enginetooling.ValidateToolArguments
	NewToolSelector              = enginetooling.NewToolSelector
)

// Promptctx types
type PromptSection = enginepromptctx.PromptSection
type PromptSectionUsage = enginepromptctx.PromptSectionUsage
type PromptBuildOptions = enginepromptctx.PromptBuildOptions
type PromptBuildResult = enginepromptctx.PromptBuildResult
type ContextManager = enginepromptctx.ContextManager
type ContextManagerOptions = enginepromptctx.ContextManagerOptions
type ContextOptimizationResult = enginepromptctx.ContextOptimizationResult

const (
	PromptCacheOptionKey      = enginepromptctx.PromptCacheOptionKey
	PromptCacheBreakpointsKey = enginepromptctx.PromptCacheBreakpointsKey
)

var (
	NewContextManager        = enginepromptctx.NewContextManager
	NewContextCompactor      = enginepromptctx.NewContextCompactor
	FoldOldToolResults       = enginepromptctx.FoldOldToolResults
	EstimateMessagesTokens   = enginepromptctx.EstimateMessagesTokens
	BuildPromptFromSections  = enginepromptctx.BuildPromptFromSections
	BuildPromptCacheMetadata = enginepromptctx.BuildPromptCacheMetadata
)

func toRequestMeta(req *RunRequest) enginepromptctx.RequestMeta {
	if req == nil {
		return enginepromptctx.RequestMeta{}
	}
	return enginepromptctx.RequestMeta{
		Model:            req.Model,
		SystemPrompt:     req.SystemPrompt,
		MaxContextTokens: req.MaxContextTokens,
	}
}
