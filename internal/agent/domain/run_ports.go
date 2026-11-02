package domain

import (
	"context"
	"encoding/json"
	"time"

	"InkLore/internal/platform/identity"
)

// RunJobLease captures the active lease on a platform job for an agent run.
type RunJobLease struct {
	TenantID   string
	JobID      string
	WorkerID   string
	LeaseEpoch int64
	RunID      string
	Recovered  bool
}

// JobsPort defines the boundary to platform_jobs for agent run scheduling.
type JobsPort interface {
	EnqueueRunJob(ctx context.Context, p identity.Principal, runID string) (string, error)
	ClaimRunJob(ctx context.Context, workerID string) (*RunJobLease, error)
	HeartbeatRunJob(ctx context.Context, lease *RunJobLease) error
	SuspendRunJob(ctx context.Context, lease *RunJobLease, wakeAt time.Time) error
	WakeRunJob(ctx context.Context, tenantID, jobID string) error
	CompleteRunJob(ctx context.Context, lease *RunJobLease, state, code string) error
	CancelRunJobCommand(ctx context.Context, tenantID, jobID string, code string) error
}

// ModelGenerateInput is the input for Think phase model generation.
type ModelGenerateInput struct {
	TenantID          string
	RunID             string
	NodeKey           string
	TaskKey           string
	PolicyVersionID   string
	Model             string
	SystemInstruction string
	UserInstruction   string
	ContextText       string
	Tools             []ToolView
	ToolChoice        any
	ParallelToolCalls *bool
	MaxOutputTokens   int
	Temperature       float64
}

// ToolCall represents a requested tool call from the model.
type ToolCall struct {
	InvocationID string
	ToolKey      string
	ToolVersion  string
	Arguments    json.RawMessage
}

// ModelGenerateOutput is the result from the model.
type ModelGenerateOutput struct {
	FinalText        *string
	ReasoningSummary *string
	ToolCalls        []ToolCall
	InputTokens      int64
	OutputTokens     int64
	CostMicros       int64
	UsageReceiptID   *string
}

// ModelGatewayPort defines the boundary for invoking LLM models.
type ModelGatewayPort interface {
	Generate(ctx context.Context, input ModelGenerateInput) (*ModelGenerateOutput, error)
	ResolvePolicy(ctx context.Context, tenantID, taskKey string) (string, error)
}

// ToolCallInput is the input for Act phase tool execution.
type ToolCallInput struct {
	TenantID      string
	RunID         string
	NodeID        string
	ExecutionKey  string
	ToolKey       string
	ToolVersionID string
	Arguments     json.RawMessage
	SideEffect    string
}

// ToolCallOutput is the result from a tool execution.
type ToolCallOutput struct {
	Success         bool
	ResultText      string
	ResultHash      string
	ResultFileID    *string
	UpstreamReceipt *string
	ErrorCode       *string
}

// ToolExecutionPort defines the boundary for executing tools.
type ToolExecutionPort interface {
	ExecuteTool(ctx context.Context, input ToolCallInput) (*ToolCallOutput, error)
}

// TaskResourcePort validates and prepares resource snapshots from content/outline/worldview/etc.
type TaskResourcePort interface {
	AuthorizeResource(context.Context, identity.Principal, *ResourceRef) error
	PrepareExecution(ctx context.Context, p identity.Principal, taskKind string, res *ResourceRef) (sourceRevision int64, sourceHash string, err error)
	VerifySourceUnchanged(ctx context.Context, p identity.Principal, taskKind string, res *ResourceRef, expectedRevision int64, expectedHash string) (bool, error)
}

// RunRepository persists agent runs, nodes, executions, approvals, artifacts, and events.
type RunRepository interface {
	// Runs
	CreateRun(ctx context.Context, tenantID string, run *AgentRun, nodes []AgentRunNode) error
	GetRun(ctx context.Context, tenantID, runID string) (*AgentRun, []AgentRunNode, error)
	GetRunOnly(ctx context.Context, tenantID, runID string) (*AgentRun, error)
	ListRunsByOwner(ctx context.Context, tenantID, ownerID, status string, query ListQuery) ([]RunSummaryView, *string, bool, error)
	CountActiveRunsByOwner(ctx context.Context, tenantID, ownerID string) (int, error)
	UpdateRunStatus(ctx context.Context, tenantID, runID string, expectedRev int64, newStatus RunStatus) (*AgentRun, error)
	TerminateRun(ctx context.Context, tenantID, runID string, expectedRev int64, targetStatus RunStatus, reason, errorCode string, retryable bool, receiptIDs []string) (*AgentRun, error)
	AllocateNextSeq(ctx context.Context, tenantID, runID string) (int64, error)

	// Nodes
	GetNode(ctx context.Context, tenantID, runID, nodeKey string) (*AgentRunNode, error)
	GetNodeByID(ctx context.Context, tenantID, runID, nodeID string) (*AgentRunNode, error)
	ListNodes(ctx context.Context, tenantID, runID string) ([]AgentRunNode, error)
	ListReadyNodes(ctx context.Context, tenantID, runID string, limit int) ([]AgentRunNode, error)
	ClaimNodeAttempt(ctx context.Context, tenantID, nodeID string, expectedRev int64, workerID string, leaseUntil time.Time, jobEpoch int64) (*AgentRunNode, error)
	HeartbeatNodes(ctx context.Context, tenantID string, nodeIDs []string, workerID string, leaseUntil time.Time) error
	SaveNodeCheckpoint(ctx context.Context, tenantID, nodeID string, expectedRev int64, jobEpoch, fence int64, phase NodePhase, iteration int, status NodeStatus, checkpoint json.RawMessage, errorCode *string) error
	FinishNode(ctx context.Context, tenantID, nodeID string, expectedRev int64, jobEpoch, fence int64, status NodeStatus, errorCode *string) error
	UpdateNodeStatus(ctx context.Context, tenantID, nodeID string, expectedRev int64, status NodeStatus) error

	// Executions
	CreateExecution(ctx context.Context, tenantID string, exec *AgentExecution) error
	GetExecutionByKey(ctx context.Context, tenantID, executionKey string) (*AgentExecution, error)
	GetExecutionByID(ctx context.Context, tenantID, executionID string) (*AgentExecution, error)
	UpdateExecution(ctx context.Context, tenantID, execID string, status ExecutionStatus, resultFileID *string, resultHash *string, receipt *string, errCode *string) error

	// Approvals
	CreateApproval(ctx context.Context, tenantID string, app *AgentApproval) error
	GetApproval(ctx context.Context, tenantID, approvalID string) (*AgentApproval, *AgentExecution, error)
	ListPendingApprovals(ctx context.Context, tenantID, ownerID string, query ListQuery) ([]ApprovalView, *string, bool, error)
	ConsumeApproval(ctx context.Context, tenantID, approvalID string, expectedRev int64, reason string) error
	RejectApproval(ctx context.Context, tenantID, approvalID string, expectedRev int64, reason string) error
	CancelApproval(ctx context.Context, tenantID, approvalID string, expectedRev int64, reason string) error

	// Artifacts
	CreateArtifact(ctx context.Context, tenantID string, art *AgentArtifact) error
	GetArtifact(ctx context.Context, tenantID, artifactID string) (*AgentArtifact, error)
	ListArtifacts(ctx context.Context, tenantID, runID string, query ListQuery) ([]ArtifactView, *string, bool, error)

	// Context Sources
	SaveContextSources(ctx context.Context, tenantID string, sources []AgentContextSource) error

	// Events
	AppendEvent(ctx context.Context, tenantID string, event *AgentRunEvent) error
	AppendEvents(ctx context.Context, tenantID string, events []*AgentRunEvent) error
	GetEventsAfter(ctx context.Context, tenantID, runID string, afterSeq int64, limit int) ([]AgentRunEvent, error)
	CheckEventCursorExpired(ctx context.Context, tenantID, runID string, cursorSeq int64) (bool, error)
}
