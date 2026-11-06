package domain

import (
	"encoding/json"
	"time"
)

// RunStatus defines the lifecycle status of an AgentRun.
type RunStatus string

const (
	RunStatusQueued               RunStatus = "queued"
	RunStatusRunning              RunStatus = "running"
	RunStatusWaitingApproval      RunStatus = "waiting_approval"
	RunStatusWaitingClarification RunStatus = "waiting_clarification"
	RunStatusCancelling           RunStatus = "cancelling"
	RunStatusSucceeded            RunStatus = "succeeded"
	RunStatusFailed               RunStatus = "failed"
	RunStatusCancelled            RunStatus = "cancelled"
)

// IsTerminal returns whether the run status is terminal.
func (s RunStatus) IsTerminal() bool {
	return s == RunStatusSucceeded || s == RunStatusFailed || s == RunStatusCancelled
}

// NodeStatus defines the execution state of an individual workflow node.
type NodeStatus string

const (
	NodeStatusPending              NodeStatus = "pending"
	NodeStatusReady                NodeStatus = "ready"
	NodeStatusRunning              NodeStatus = "running"
	NodeStatusWaitingApproval      NodeStatus = "waiting_approval"
	NodeStatusWaitingClarification NodeStatus = "waiting_clarification"
	NodeStatusRetryWait            NodeStatus = "retry_wait"
	NodeStatusSucceeded            NodeStatus = "succeeded"
	NodeStatusFailed               NodeStatus = "failed"
	NodeStatusCancelled            NodeStatus = "cancelled"
	NodeStatusSkipped              NodeStatus = "skipped"
)

func (s NodeStatus) IsTerminal() bool {
	return s == NodeStatusSucceeded || s == NodeStatusFailed || s == NodeStatusCancelled || s == NodeStatusSkipped
}

// NodePhase defines the current TAO phase of a running node.
type NodePhase string

const (
	NodePhaseThink   NodePhase = "think"
	NodePhaseAct     NodePhase = "act"
	NodePhaseObserve NodePhase = "observe"
)

// ExecutionStatus defines the state of a tool invocation.
type ExecutionStatus string

const (
	ExecutionStatusPrepared    ExecutionStatus = "prepared"
	ExecutionStatusDispatching ExecutionStatus = "dispatching"
	ExecutionStatusSucceeded   ExecutionStatus = "succeeded"
	ExecutionStatusFailed      ExecutionStatus = "failed"
	ExecutionStatusUnknown     ExecutionStatus = "unknown"
)

// ApprovalStatus defines the state of a human approval gate.
type ApprovalStatus string

const (
	ApprovalStatusPending   ApprovalStatus = "pending"
	ApprovalStatusConsumed  ApprovalStatus = "consumed"
	ApprovalStatusRejected  ApprovalStatus = "rejected"
	ApprovalStatusExpired   ApprovalStatus = "expired"
	ApprovalStatusCancelled ApprovalStatus = "cancelled"
)

// ArtifactKind identifies the suggestion output category.
type ArtifactKind string

const (
	ArtifactKindChapterDraft      ArtifactKind = "chapter_draft"
	ArtifactKindRewriteSuggestion ArtifactKind = "rewrite_suggestion"
	ArtifactKindOutlineSuggestion ArtifactKind = "outline_suggestion"
	ArtifactKindWorldviewSuggest  ArtifactKind = "worldview_suggestion"
	ArtifactKindAnalysisResult    ArtifactKind = "analysis_result"
	ArtifactKindAuditResult       ArtifactKind = "audit_result"
)

// MemoryKind defines context source types.
type MemoryKind string

const (
	MemoryKindConfirmedFact      MemoryKind = "confirmed_fact"
	MemoryKindOwnerSnapshot      MemoryKind = "owner_snapshot"
	MemoryKindDerivedSummary     MemoryKind = "derived_summary"
	MemoryKindExternalSuggestion MemoryKind = "external_suggestion"
	MemoryKindToolObservation    MemoryKind = "tool_observation"
)

// Disposition defines how a context source was treated in budget assembly.
type Disposition string

const (
	DispositionIncluded   Disposition = "included"
	DispositionCompressed Disposition = "compressed"
	DispositionDropped    Disposition = "dropped"
)

// Selection represents a Unicode code-point slice of source text.
type Selection struct {
	StartOffset int    `json:"start_offset"`
	EndOffset   int    `json:"end_offset"`
	SourceHash  string `json:"source_hash"`
}

// ResourceRef represents target resource binding for an agent run.
type ResourceRef struct {
	ResourceID     string     `json:"resource_id"`
	ResourceType   string     `json:"resource_type"`
	SourceRevision int64      `json:"source_revision"`
	NovelID        *string    `json:"novel_id,omitempty"`
	SourceHash     *string    `json:"source_hash,omitempty"`
	Selection      *Selection `json:"selection,omitempty"`
}

// AgentRun is the aggregate root for an agent run instance.
type AgentRun struct {
	ID                  string     `json:"id"`
	TenantID            string     `json:"tenant_id"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	Revision            int64      `json:"revision"`
	OwnerUserID         string     `json:"owner_user_id"`
	PlatformJobID       string     `json:"platform_job_id"`
	Kind                string     `json:"kind"` // creator | diagnostic
	TaskKind            string     `json:"task_kind"`
	DiagnosticProbeKey  *string    `json:"diagnostic_probe_key,omitempty"`
	TaskKey             string     `json:"task_key"`
	ResourceType        *string    `json:"resource_type,omitempty"`
	ResourceID          *string    `json:"resource_id,omitempty"`
	NovelID             *string    `json:"novel_id,omitempty"`
	ChapterID           *string    `json:"chapter_id,omitempty"`
	SourceRevision      *int64     `json:"source_revision,omitempty"`
	SourceHash          *string    `json:"source_hash,omitempty"`
	Instruction         string     `json:"instruction"`
	DefinitionVersionID *string    `json:"definition_version_id,omitempty"`
	WorkflowVersionID   *string    `json:"workflow_version_id,omitempty"`
	PolicyVersionID     string     `json:"policy_version_id"`
	Status              RunStatus  `json:"status"`
	NextSeq             int64      `json:"next_seq"`
	RetainedFromSeq     int64      `json:"retained_from_seq"`
	DeadlineAt          time.Time  `json:"deadline_at"`
	StartedAt           *time.Time `json:"started_at,omitempty"`
	FinishedAt          *time.Time `json:"finished_at,omitempty"`
	ErrorCode           *string    `json:"error_code,omitempty"`
	TerminalReason      *string    `json:"terminal_reason,omitempty"`
	Retryable           bool       `json:"retryable"`
	UsageReceiptIDs     []string   `json:"usage_receipt_ids"`
	DeletedAt           *time.Time `json:"deleted_at,omitempty"`
}

// AgentRunNode represents a single executable node of a run.
type AgentRunNode struct {
	ID                  string          `json:"id"`
	TenantID            string          `json:"tenant_id"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
	Revision            int64           `json:"revision"`
	RunID               string          `json:"run_id"`
	NodeKey             string          `json:"node_key"`
	Role                NodeRole        `json:"role"`
	DependsOn           []string        `json:"depends_on"`
	Required            bool            `json:"required"`
	DefinitionVersionID *string         `json:"definition_version_id,omitempty"`
	ToolVersionID       *string         `json:"tool_version_id,omitempty"`
	Status              NodeStatus      `json:"status"`
	Phase               NodePhase       `json:"phase"`
	Iteration           int             `json:"iteration"`
	Attempt             int             `json:"attempt"`
	LeaseOwner          *string         `json:"lease_owner,omitempty"`
	LeaseUntil          *time.Time      `json:"lease_until,omitempty"`
	FencingToken        int64           `json:"fencing_token"`
	JobEpoch            int64           `json:"job_epoch"`
	NextAttemptAt       time.Time       `json:"next_attempt_at"`
	Checkpoint          json.RawMessage `json:"checkpoint"`
	ErrorCode           *string         `json:"error_code,omitempty"`
	StartedAt           *time.Time      `json:"started_at,omitempty"`
	FinishedAt          *time.Time      `json:"finished_at,omitempty"`
}

// AgentExecution records an invocation of a tool.
type AgentExecution struct {
	ID              string          `json:"id"`
	TenantID        string          `json:"tenant_id"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	Revision        int64           `json:"revision"`
	RunID           string          `json:"run_id"`
	NodeID          string          `json:"node_id"`
	ToolVersionID   string          `json:"tool_version_id"`
	ExecutionKey    string          `json:"execution_key"`
	Iteration       int             `json:"iteration"`
	CallIndex       int             `json:"call_index"`
	ArgumentsHash   string          `json:"arguments_hash"`
	FrozenArguments json.RawMessage `json:"frozen_arguments"`
	Status          ExecutionStatus `json:"status"`
	FencingToken    int64           `json:"fencing_token"`
	JobEpoch        int64           `json:"job_epoch"`
	ResultFileID    *string         `json:"result_file_id,omitempty"`
	ResultHash      *string         `json:"result_hash,omitempty"`
	UpstreamReceipt *string         `json:"upstream_receipt,omitempty"`
	ErrorCode       *string         `json:"error_code,omitempty"`
	DispatchedAt    *time.Time      `json:"dispatched_at,omitempty"`
	FinishedAt      *time.Time      `json:"finished_at,omitempty"`
}

// AgentApproval represents a pending or resolved human approval.
type AgentApproval struct {
	ID             string         `json:"id"`
	TenantID       string         `json:"tenant_id"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	Revision       int64          `json:"revision"`
	OwnerUserID    string         `json:"owner_user_id"`
	RunID          string         `json:"run_id"`
	NodeID         string         `json:"node_id"`
	ExecutionID    string         `json:"execution_id"`
	ToolVersionID  string         `json:"tool_version_id"`
	ArgumentsHash  string         `json:"arguments_hash"`
	SourceRevision int64          `json:"source_revision"`
	Status         ApprovalStatus `json:"status"`
	ExpiresAt      time.Time      `json:"expires_at"`
	DecidedAt      *time.Time     `json:"decided_at,omitempty"`
	DecisionReason *string        `json:"decision_reason,omitempty"`
}

// AgentArtifact represents an immutable suggestion artifact.
type AgentArtifact struct {
	ID               string       `json:"id"`
	TenantID         string       `json:"tenant_id"`
	CreatedAt        time.Time    `json:"created_at"`
	UpdatedAt        time.Time    `json:"updated_at"`
	RunID            string       `json:"run_id"`
	NodeID           string       `json:"node_id"`
	Kind             ArtifactKind `json:"kind"`
	TargetResourceID *string      `json:"target_resource_id,omitempty"`
	SourceRevision   int64        `json:"source_revision"`
	ContentHash      string       `json:"content_hash"`
	TextContent      *string      `json:"text_content,omitempty"`
	FileID           *string      `json:"file_id,omitempty"`
	IsPartial        bool         `json:"is_partial"`
}

// AgentContextSource documents provenance and budget impact of context chunks.
type AgentContextSource struct {
	ID              string      `json:"id"`
	TenantID        string      `json:"tenant_id"`
	CreatedAt       time.Time   `json:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at"`
	RunID           string      `json:"run_id"`
	SourceDomain    string      `json:"source_domain"`
	SourceType      string      `json:"source_type"`
	SourceID        string      `json:"source_id"`
	SourceRevision  int64       `json:"source_revision"`
	ContentHash     string      `json:"content_hash"`
	MemoryKind      MemoryKind  `json:"memory_kind"`
	TokenCount      int         `json:"token_count"`
	Estimated       bool        `json:"estimated"`
	Disposition     Disposition `json:"disposition"`
	MemoryParentIDs []string    `json:"memory_parent_ids"`
}

// AgentRunEvent represents an append-only event persisted for SSE and audit.
type AgentRunEvent struct {
	ID        string          `json:"id"`
	TenantID  string          `json:"tenant_id"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	RunID     string          `json:"run_id"`
	Seq       int64           `json:"seq"`
	EventName string          `json:"event_name"`
	Payload   json.RawMessage `json:"payload"`
	ExpiresAt time.Time       `json:"expires_at"`
}

// SSEEnvelope matches the standard SSE data envelope contract.
type SSEEnvelope struct {
	RunID      string          `json:"run_id"`
	Seq        int64           `json:"seq"`
	OccurredAt time.Time       `json:"occurred_at"`
	Payload    json.RawMessage `json:"payload"`
}

// Event payload structures conforming to api/events.json

type RunStartedPayload struct {
	ModelID  *string `json:"model_id"`
	TaskKind string  `json:"task_kind"`
}

type TextDeltaPayload struct {
	ArtifactID string `json:"artifact_id"`
	Delta      string `json:"delta"`
}

type ReasoningSummaryPayload struct {
	Summary string `json:"summary"`
}

type ToolStartedPayload struct {
	NodeID       string `json:"node_id"`
	ToolID       string `json:"tool_id"`
	InvocationID string `json:"invocation_id"`
}

type ToolCompletedPayload struct {
	NodeID       string  `json:"node_id"`
	InvocationID string  `json:"invocation_id"`
	Success      bool    `json:"success"`
	ResultRef    *string `json:"result_ref"`
	ErrorCode    *string `json:"error_code"`
}

type ApprovalRequiredPayload struct {
	ApprovalID string    `json:"approval_id"`
	NodeID     string    `json:"node_id"`
	ExpiresAt  time.Time `json:"expires_at"`
}

type NodeUpdatedPayload struct {
	NodeID string     `json:"node_id"`
	Status NodeStatus `json:"status"`
}

type UsageUpdatedPayload struct {
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	CostMicros   int64  `json:"cost_micros"`
	Currency     string `json:"currency"`
	Estimated    bool   `json:"estimated"`
}

type RunSucceededPayload struct {
	ArtifactIDs []string `json:"artifact_ids"`
	Revision    int64    `json:"revision"`
}

type RunFailedPayload struct {
	Code      string `json:"code"`
	Retryable bool   `json:"retryable"`
	Revision  int64  `json:"revision"`
}

type RunCancelledPayload struct {
	Reason   string `json:"reason"`
	Revision int64  `json:"revision"`
}

// Public API Views

// RunView matches the api/agent-core-schemas.json#/components/schemas/Run schema.
type RunView struct {
	PendingQuestions    []Question   `json:"pending_questions"`
	ID                  string       `json:"id"`
	Revision            int64        `json:"revision"`
	CreatedAt           time.Time    `json:"created_at"`
	UpdatedAt           time.Time    `json:"updated_at"`
	OwnerUserID         string       `json:"owner_user_id"`
	Kind                string       `json:"kind"`
	TaskKind            string       `json:"task_kind"`
	TaskKey             string       `json:"task_key"`
	Resource            *ResourceRef `json:"resource"`
	Instruction         string       `json:"instruction"`
	Status              RunStatus    `json:"status"`
	PolicyVersionID     string       `json:"policy_version_id"`
	DefinitionVersionID *string      `json:"definition_version_id"`
	WorkflowVersionID   *string      `json:"workflow_version_id"`
	Nodes               []NodeView   `json:"nodes"`
	ErrorCode           *string      `json:"error_code"`
	DeadlineAt          time.Time    `json:"deadline_at"`
	StartedAt           *time.Time   `json:"started_at"`
	FinishedAt          *time.Time   `json:"finished_at"`
	TerminalReason      *string      `json:"terminal_reason"`
	Retryable           bool         `json:"retryable"`
}

// NodeView matches the api/agent-core-schemas.json#/components/schemas/Node schema.
type NodeView struct {
	ID         string     `json:"id"`
	Revision   int64      `json:"revision"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	RunID      string     `json:"run_id"`
	NodeKey    string     `json:"node_key"`
	Role       NodeRole   `json:"role"`
	DependsOn  []string   `json:"depends_on"`
	Status     NodeStatus `json:"status"`
	Phase      NodePhase  `json:"phase"`
	Iteration  int        `json:"iteration"`
	Attempt    int        `json:"attempt"`
	ErrorCode  *string    `json:"error_code"`
	StartedAt  *time.Time `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
}

// RunSummaryView matches RunSummary in agent-core-schemas.json.
type RunSummaryView struct {
	ID        string       `json:"id"`
	Revision  int64        `json:"revision"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
	TaskKind  string       `json:"task_kind"`
	Status    RunStatus    `json:"status"`
	Resource  *ResourceRef `json:"resource"`
	ErrorCode *string      `json:"error_code"`
}

// ApprovalView matches Approval in agent-core-schemas.json.
type ApprovalView struct {
	ID             string         `json:"id"`
	Revision       int64          `json:"revision"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	RunID          string         `json:"run_id"`
	NodeID         string         `json:"node_id"`
	OwnerUserID    string         `json:"owner_user_id"`
	ToolVersionID  string         `json:"tool_version_id"`
	ArgumentsHash  string         `json:"arguments_hash"`
	ArgumentsJSON  string         `json:"arguments_json"`
	SourceRevision int64          `json:"source_revision"`
	Status         ApprovalStatus `json:"status"`
	ExpiresAt      time.Time      `json:"expires_at"`
	DecidedAt      *time.Time     `json:"decided_at"`
	DecisionReason *string        `json:"decision_reason"`
}

// ArtifactView matches Artifact in agent-core-schemas.json.
type ArtifactView struct {
	ID               string       `json:"id"`
	CreatedAt        time.Time    `json:"created_at"`
	RunID            string       `json:"run_id"`
	NodeID           string       `json:"node_id"`
	Kind             ArtifactKind `json:"kind"`
	TargetResourceID *string      `json:"target_resource_id"`
	SourceRevision   int64        `json:"source_revision"`
	ContentHash      string       `json:"content_hash"`
	TextContent      *string      `json:"text_content"`
	FileID           *string      `json:"file_id"`
	IsPartial        bool         `json:"is_partial"`
}

// Page response views

type RunSummaryPage struct {
	Items      []RunSummaryView `json:"items"`
	NextCursor *string          `json:"next_cursor"`
	HasMore    bool             `json:"has_more"`
}

type ApprovalPage struct {
	Items      []ApprovalView `json:"items"`
	NextCursor *string        `json:"next_cursor"`
	HasMore    bool           `json:"has_more"`
}

type ArtifactPage struct {
	Items      []ArtifactView `json:"items"`
	NextCursor *string        `json:"next_cursor"`
	HasMore    bool           `json:"has_more"`
}

// Input DTOs

type CreateRunInput struct {
	TaskKind     string       `json:"task_kind"`
	TaskKey      string       `json:"task_key"`
	Resource     *ResourceRef `json:"resource"`
	Instruction  string       `json:"instruction"`
	DefinitionID string       `json:"definition_id"`
	WorkflowID   *string      `json:"workflow_id,omitempty"`
	ModelID      *string      `json:"model_id,omitempty"`
	ModelKey     *string      `json:"model_key,omitempty"`
}

type CancelRunInput struct {
	ExpectedRevision int64 `json:"expected_revision"`
}

type ApprovalDecisionInput struct {
	ExpectedRevision int64  `json:"expected_revision"`
	ArgumentsHash    string `json:"arguments_hash"`
	SourceRevision   int64  `json:"source_revision"`
	Reason           string `json:"reason,omitempty"`
}
