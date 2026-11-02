package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	d "InkLore/internal/agent/domain"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/scope"
	"InkLore/internal/platform/uow"

	"github.com/google/uuid"
)

const (
	MaxActiveRunsPerOwner = 3
	MaxInstructionLen     = 8000
	DefaultRunDeadline    = 30 * time.Minute
)

// RunService coordinates agent run lifecycles, execution, and termination.
type RunService struct {
	repo      d.RunRepository
	govRepo   d.GovernanceRepository
	jobs      d.JobsPort
	gateway   d.ModelGatewayPort
	resources d.TaskResourcePort
	manager   *uow.Manager
}

// NewRunService constructs a RunService instance.
func NewRunService(
	repo d.RunRepository,
	govRepo d.GovernanceRepository,
	jobs d.JobsPort,
	gateway d.ModelGatewayPort,
	resources d.TaskResourcePort,
	manager *uow.Manager,
) (*RunService, error) {
	if repo == nil {
		return nil, errors.New("agent run service: run repository required")
	}
	if jobs == nil {
		return nil, errors.New("agent run service: jobs port required")
	}
	if manager == nil {
		return nil, errors.New("agent run service: uow manager required")
	}
	return &RunService{
		repo:      repo,
		govRepo:   govRepo,
		jobs:      jobs,
		gateway:   gateway,
		resources: resources,
		manager:   manager,
	}, nil
}

// AuthorizeCreator ensures the principal is an authenticated creator.
func (s *RunService) AuthorizeCreator(p identity.Principal) error {
	if p.Audience != "creator" {
		return d.ErrForbidden()
	}
	for _, r := range p.Roles {
		if r == "creator" {
			return nil
		}
	}
	return d.ErrForbidden()
}

// AuthorizeCreateRun checks the current creator can still access a run source.
// It intentionally does not inspect any optimistic revision: replays must not
// be rejected by a CAS value before the idempotency record is consulted.
func (s *RunService) AuthorizeCreateRun(ctx context.Context, p identity.Principal, in d.CreateRunInput) error {
	if err := s.AuthorizeCreator(p); err != nil {
		return err
	}
	if in.Resource == nil || !scope.ValidID(in.Resource.ResourceID) {
		return d.ErrValidationFailed(map[string]any{"reason": "valid resource reference is required"})
	}
	if in.TaskKind != string(d.TaskKindInspirationAnalyze) && (in.Resource.NovelID == nil || !scope.ValidID(*in.Resource.NovelID)) {
		return d.ErrValidationFailed(map[string]any{"reason": "novel_id is required for this task_kind"})
	}
	if s.resources == nil {
		return d.ErrInvalidConfiguration(map[string]any{"reason": "task resource authorizer is required"})
	}
	return s.resources.AuthorizeResource(ctx, p, in.Resource)
}

// CreateRun validates prerequisites, freezes policies/DAG, and enqueues a new run.
func (s *RunService) CreateRun(ctx context.Context, p identity.Principal, in d.CreateRunInput, idempotencyKey string) (*d.RunView, error) {
	if err := s.AuthorizeCreator(p); err != nil {
		return nil, err
	}

	// Validate instruction
	if length := utf8.RuneCountInString(in.Instruction); length < 1 || length > MaxInstructionLen {
		return nil, d.ErrValidationFailed(map[string]any{"reason": "instruction must be between 1 and 8000 codepoints"})
	}

	// Validate task_kind and task_key
	if in.TaskKind == "" || in.TaskKey == "" {
		return nil, d.ErrValidationFailed(map[string]any{"reason": "task_kind and task_key are required"})
	}

	// Resource validation
	if in.Resource == nil {
		return nil, d.ErrValidationFailed(map[string]any{"reason": "resource reference is required"})
	}
	if !scope.ValidID(in.Resource.ResourceID) {
		return nil, d.ErrValidationFailed(map[string]any{"reason": "invalid resource_id"})
	}
	if in.TaskKind != string(d.TaskKindInspirationAnalyze) {
		if in.Resource.NovelID == nil || !scope.ValidID(*in.Resource.NovelID) {
			return nil, d.ErrValidationFailed(map[string]any{"reason": "novel_id is required for this task_kind"})
		}
	}

	// Check active run capacity (max 3 non-terminal runs per creator)
	activeCount, err := s.repo.CountActiveRunsByOwner(ctx, p.TenantID, p.UserID)
	if err != nil {
		return nil, err
	}
	if activeCount >= MaxActiveRunsPerOwner {
		return nil, d.ErrRunCapacityExceeded(map[string]any{
			"active_runs": activeCount,
			"limit":       MaxActiveRunsPerOwner,
		})
	}

	// Verify and prepare source snapshot
	sourceRevision := in.Resource.SourceRevision
	sourceHash := ""
	if s.resources != nil {
		rev, hash, err := s.resources.PrepareExecution(ctx, p, in.TaskKind, in.Resource)
		if err != nil {
			return nil, err
		}
		sourceRevision = rev
		sourceHash = hash
	} else if in.Resource.SourceHash != nil {
		sourceHash = *in.Resource.SourceHash
	}

	// Validate definition
	if !scope.ValidID(in.DefinitionID) {
		return nil, d.ErrValidationFailed(map[string]any{"reason": "invalid definition_id"})
	}
	if s.govRepo == nil {
		return nil, d.ErrInvalidConfiguration(map[string]any{"reason": "agent governance repository is required"})
	}
	def, defVer, err := s.govRepo.GetDefinition(ctx, p.TenantID, in.DefinitionID)
	if err != nil {
		return nil, err
	}
	if def == nil || !def.Enabled || def.CurrentVersionID == nil || defVer == nil || defVer.ID != *def.CurrentVersionID {
		return nil, d.ErrInvalidConfiguration(map[string]any{"reason": "definition is not enabled or published"})
	}
	allowedTaskKind := false
	for _, taskKind := range defVer.TaskKinds {
		if taskKind == in.TaskKind {
			allowedTaskKind = true
			break
		}
	}
	if !allowedTaskKind {
		return nil, d.ErrInvalidConfiguration(map[string]any{"reason": "task_kind is not enabled for definition"})
	}
	defVerID := def.CurrentVersionID

	// Prepare workflow & nodes
	var wfVerID *string
	var manifestNodes []d.ManifestNode
	if in.WorkflowID != nil && *in.WorkflowID != "" {
		if !scope.ValidID(*in.WorkflowID) {
			return nil, d.ErrValidationFailed(map[string]any{"reason": "invalid workflow_id"})
		}
		if s.govRepo != nil {
			wf, wfVer, err := s.govRepo.GetWorkflow(ctx, p.TenantID, *in.WorkflowID)
			if err != nil {
				return nil, err
			}
			if wf == nil || !wf.Enabled || wf.CurrentVersionID == nil || wfVer == nil || wfVer.ID != *wf.CurrentVersionID {
				return nil, d.ErrInvalidConfiguration(map[string]any{"reason": "workflow is not enabled or published"})
			}
			if err := d.ValidateDAG(wfVer.Manifest); err != nil {
				return nil, err
			}
			wfVerID = wf.CurrentVersionID
			manifestNodes = wfVer.Manifest.Nodes
		}
	}

	// A workflow omitted by the R1 request contract uses one definition-bound root node.
	if len(manifestNodes) == 0 {
		manifestNodes = []d.ManifestNode{
			{
				NodeKey:             "root",
				Role:                string(d.RoleWriter),
				DependsOn:           []string{},
				Required:            true,
				DefinitionVersionID: defVerID,
			},
		}
	}

	// Freeze the actual published LLM policy version into the run. A random
	// placeholder would violate the policy foreign key and make replay opaque.
	if s.gateway == nil {
		return nil, d.ErrInvalidConfiguration(map[string]any{"reason": "model gateway is not configured"})
	}
	runID := uuid.NewString()
	now := time.Now().UTC()
	deadLine := now.Add(DefaultRunDeadline)

	run := &d.AgentRun{
		ID:                  runID,
		TenantID:            p.TenantID,
		CreatedAt:           now,
		UpdatedAt:           now,
		Revision:            1,
		OwnerUserID:         p.UserID,
		Kind:                "creator",
		TaskKind:            in.TaskKind,
		TaskKey:             in.TaskKey,
		ResourceType:        &in.Resource.ResourceType,
		ResourceID:          &in.Resource.ResourceID,
		NovelID:             in.Resource.NovelID,
		SourceRevision:      &sourceRevision,
		SourceHash:          &sourceHash,
		Instruction:         in.Instruction,
		DefinitionVersionID: defVerID,
		WorkflowVersionID:   wfVerID,
		PolicyVersionID:     "",
		Status:              d.RunStatusQueued,
		NextSeq:             2, // Seq 1 is run.started
		RetainedFromSeq:     1,
		DeadlineAt:          deadLine,
		Retryable:           false,
		UsageReceiptIDs:     []string{},
	}

	// Build nodes
	userModel := ""
	if in.ModelID != nil && strings.TrimSpace(*in.ModelID) != "" {
		userModel = strings.TrimSpace(*in.ModelID)
	} else if in.ModelKey != nil && strings.TrimSpace(*in.ModelKey) != "" {
		userModel = strings.TrimSpace(*in.ModelKey)
	}
	checkpointBytes := []byte("{}")
	if userModel != "" {
		cpMap := map[string]string{"user_model": userModel}
		if b, err := json.Marshal(cpMap); err == nil {
			checkpointBytes = b
		}
	}

	nodes := make([]d.AgentRunNode, 0, len(manifestNodes))
	for _, mn := range manifestNodes {
		nodeStatus := d.NodeStatusPending
		if len(mn.DependsOn) == 0 {
			nodeStatus = d.NodeStatusReady
		}
		nodes = append(nodes, d.AgentRunNode{
			ID:                  uuid.NewString(),
			TenantID:            p.TenantID,
			CreatedAt:           now,
			UpdatedAt:           now,
			Revision:            1,
			RunID:               runID,
			NodeKey:             mn.NodeKey,
			Role:                d.NodeRole(mn.Role),
			DependsOn:           mn.DependsOn,
			Required:            mn.Required,
			DefinitionVersionID: mn.DefinitionVersionID,
			ToolVersionID:       mn.ToolVersionID,
			Status:              nodeStatus,
			Phase:               d.NodePhaseThink,
			Iteration:           0,
			Attempt:             0,
			FencingToken:        0,
			JobEpoch:            0,
			NextAttemptAt:       now,
			Checkpoint:          checkpointBytes,
		})
	}

	// In single UoW transaction: enqueue job, create run + nodes + run.started event
	err = s.manager.Within(ctx, func(tx context.Context) error {
		policyVersionID, err := s.gateway.ResolvePolicy(tx, p.TenantID, in.TaskKey)
		if err != nil {
			return err
		}
		if policyVersionID == "" {
			return d.ErrInvalidConfiguration(map[string]any{"reason": "enabled llm policy is required for task_key"})
		}
		run.PolicyVersionID = policyVersionID
		jobID, err := s.jobs.EnqueueRunJob(tx, p, runID)
		if err != nil {
			return err
		}
		run.PlatformJobID = jobID

		if err := s.repo.CreateRun(tx, p.TenantID, run, nodes); err != nil {
			return err
		}

		// Initial event: run.started
		payload, _ := json.Marshal(d.RunStartedPayload{
			ModelID:  nil,
			TaskKind: in.TaskKind,
		})
		startEvent := &d.AgentRunEvent{
			ID:        uuid.NewString(),
			TenantID:  p.TenantID,
			CreatedAt: now,
			UpdatedAt: now,
			RunID:     runID,
			Seq:       1,
			EventName: "run.started",
			Payload:   payload,
			ExpiresAt: now.Add(7 * 24 * time.Hour),
		}
		return s.repo.AppendEvent(tx, p.TenantID, startEvent)
	})
	if err != nil {
		return nil, err
	}

	return s.buildRunView(run, nodes), nil
}

// GetRun fetches a run and its nodes for the owner.
func (s *RunService) GetRun(ctx context.Context, p identity.Principal, runID string) (*d.RunView, error) {
	if err := s.AuthorizeCreator(p); err != nil {
		return nil, err
	}
	if !scope.ValidID(runID) {
		return nil, d.ErrValidationFailed(map[string]any{"reason": "invalid run_id"})
	}

	run, nodes, err := s.repo.GetRun(ctx, p.TenantID, runID)
	if err != nil {
		return nil, err
	}
	if run.OwnerUserID != p.UserID && p.Audience != "admin" {
		return nil, d.ErrNotFound()
	}

	view := s.buildRunView(run, nodes)
	if s.resources == nil {
		return nil, d.ErrNotFound()
	}
	if err := s.resources.AuthorizeResource(ctx, p, view.Resource); err != nil {
		return nil, err
	}
	return view, nil
}

// ListRuns lists runs belonging to the creator.
func (s *RunService) ListRuns(ctx context.Context, p identity.Principal, status string, query d.ListQuery) ([]d.RunSummaryView, *string, bool, error) {
	if err := s.AuthorizeCreator(p); err != nil {
		return nil, nil, false, err
	}
	items, cursor, more, err := s.repo.ListRunsByOwner(ctx, p.TenantID, p.UserID, status, query)
	if err != nil {
		return nil, nil, false, err
	}
	visible := make([]d.RunSummaryView, 0, len(items))
	for _, item := range items {
		if err := s.AuthorizeRun(ctx, p, item.ID); err != nil {
			if hiddenResource(err) {
				continue
			}
			return nil, nil, false, err
		}
		visible = append(visible, item)
	}
	return visible, cursor, more, nil
}

// CancelRun handles user cancellation of a run.
func (s *RunService) CancelRun(ctx context.Context, p identity.Principal, runID string, in d.CancelRunInput, idempotencyKey string) (*d.RunView, error) {
	if err := s.AuthorizeCreator(p); err != nil {
		return nil, err
	}
	if !scope.ValidID(runID) {
		return nil, d.ErrValidationFailed(map[string]any{"reason": "invalid run_id"})
	}

	run, nodes, err := s.repo.GetRun(ctx, p.TenantID, runID)
	if err != nil {
		return nil, err
	}
	if run.OwnerUserID != p.UserID {
		return nil, d.ErrNotFound()
	}

	// Idempotent completion if already in terminal state
	if run.Status.IsTerminal() {
		return s.buildRunView(run, nodes), nil
	}

	// Check expected revision
	if in.ExpectedRevision > 0 && in.ExpectedRevision != run.Revision {
		return nil, d.ErrVersionConflict(map[string]any{
			"current_revision":  run.Revision,
			"expected_revision": in.ExpectedRevision,
		})
	}

	// If queued or waiting_approval, terminate immediately
	if run.Status == d.RunStatusQueued || run.Status == d.RunStatusWaitingApproval {
		terminatedRun, err := s.Terminate(ctx, p.TenantID, runID, run.Revision, d.RunStatusCancelled, "user_requested", "", false, nil)
		if err != nil {
			return nil, err
		}
		nodes, _ := s.repo.ListNodes(ctx, p.TenantID, runID)
		return s.buildRunView(terminatedRun, nodes), nil
	}

	// If running, move to cancelling first, then terminate
	if run.Status == d.RunStatusRunning {
		err = s.manager.Within(ctx, func(tx context.Context) error {
			updated, err := s.repo.UpdateRunStatus(tx, p.TenantID, runID, run.Revision, d.RunStatusCancelling)
			if err != nil {
				return err
			}
			run = updated
			return nil
		})
		if err != nil {
			return nil, err
		}
		// Now terminate to cancelled
		terminatedRun, err := s.Terminate(ctx, p.TenantID, runID, run.Revision, d.RunStatusCancelled, "user_requested", "", false, nil)
		if err != nil {
			return nil, err
		}
		nodes, _ := s.repo.ListNodes(ctx, p.TenantID, runID)
		return s.buildRunView(terminatedRun, nodes), nil
	}

	return s.buildRunView(run, nodes), nil
}

// Terminate is the single CAS entry point for transitioning a run to a terminal state (succeeded, failed, cancelled).
func (s *RunService) Terminate(
	ctx context.Context,
	tenantID, runID string,
	expectedRev int64,
	targetStatus d.RunStatus,
	reason, errorCode string,
	retryable bool,
	receiptIDs []string,
) (*d.AgentRun, error) {
	if !targetStatus.IsTerminal() {
		return nil, fmt.Errorf("target status %s is not terminal", targetStatus)
	}

	var terminatedRun *d.AgentRun
	err := s.manager.Within(ctx, func(tx context.Context) error {
		// 1. CAS update run status to terminal
		var err error
		terminatedRun, err = s.repo.TerminateRun(tx, tenantID, runID, expectedRev, targetStatus, reason, errorCode, retryable, receiptIDs)
		if err != nil {
			return err
		}

		// 2. Cascade cancel incomplete nodes
		nodes, err := s.repo.ListNodes(tx, tenantID, runID)
		if err != nil {
			return err
		}
		for _, node := range nodes {
			if !node.Status.IsTerminal() {
				_ = s.repo.UpdateNodeStatus(tx, tenantID, node.ID, node.Revision, d.NodeStatusCancelled)
			}
		}

		// 3. Complete platform job
		jobCode := errorCode
		if jobCode == "" {
			jobCode = reason
		}
		if s.jobs != nil && terminatedRun.PlatformJobID != "" {
			_ = s.jobs.CancelRunJobCommand(tx, tenantID, terminatedRun.PlatformJobID, jobCode)
		}

		// 4. Allocate next seq and write the unique terminal event
		seq, err := s.repo.AllocateNextSeq(tx, tenantID, runID)
		if err != nil {
			return err
		}

		now := time.Now().UTC()
		var eventName string
		var payloadBytes []byte

		switch targetStatus {
		case d.RunStatusSucceeded:
			eventName = "run.succeeded"
			payloadBytes, _ = json.Marshal(d.RunSucceededPayload{
				ArtifactIDs: []string{},
				Revision:    terminatedRun.Revision,
			})
		case d.RunStatusFailed:
			eventName = "run.failed"
			payloadBytes, _ = json.Marshal(d.RunFailedPayload{
				Code:      errorCode,
				Retryable: retryable,
				Revision:  terminatedRun.Revision,
			})
		case d.RunStatusCancelled:
			eventName = "run.cancelled"
			cancelReason := reason
			if cancelReason == "" {
				cancelReason = "user_requested"
			}
			payloadBytes, _ = json.Marshal(d.RunCancelledPayload{
				Reason:   cancelReason,
				Revision: terminatedRun.Revision,
			})
		}

		terminalEvent := &d.AgentRunEvent{
			ID:        uuid.NewString(),
			TenantID:  tenantID,
			CreatedAt: now,
			UpdatedAt: now,
			RunID:     runID,
			Seq:       seq,
			EventName: eventName,
			Payload:   payloadBytes,
			ExpiresAt: now.Add(7 * 24 * time.Hour),
		}

		return s.repo.AppendEvent(tx, tenantID, terminalEvent)
	})
	if err != nil {
		return nil, err
	}
	return terminatedRun, nil
}

func (s *RunService) buildRunView(run *d.AgentRun, nodes []d.AgentRunNode) *d.RunView {
	nodeViews := make([]d.NodeView, 0, len(nodes))
	for _, n := range nodes {
		nodeViews = append(nodeViews, d.NodeView{
			ID:         n.ID,
			Revision:   n.Revision,
			CreatedAt:  n.CreatedAt,
			UpdatedAt:  n.UpdatedAt,
			RunID:      n.RunID,
			NodeKey:    n.NodeKey,
			Role:       n.Role,
			DependsOn:  n.DependsOn,
			Status:     n.Status,
			Phase:      n.Phase,
			Iteration:  n.Iteration,
			Attempt:    n.Attempt,
			ErrorCode:  n.ErrorCode,
			StartedAt:  n.StartedAt,
			FinishedAt: n.FinishedAt,
		})
	}

	var resRef *d.ResourceRef
	if run.ResourceID != nil && run.ResourceType != nil && run.SourceRevision != nil {
		resRef = &d.ResourceRef{
			ResourceID:     *run.ResourceID,
			ResourceType:   *run.ResourceType,
			SourceRevision: *run.SourceRevision,
			NovelID:        run.NovelID,
			SourceHash:     run.SourceHash,
		}
	}

	return &d.RunView{
		ID:                  run.ID,
		Revision:            run.Revision,
		CreatedAt:           run.CreatedAt,
		UpdatedAt:           run.UpdatedAt,
		OwnerUserID:         run.OwnerUserID,
		Kind:                run.Kind,
		TaskKind:            run.TaskKind,
		TaskKey:             run.TaskKey,
		Resource:            resRef,
		Instruction:         run.Instruction,
		Status:              run.Status,
		PolicyVersionID:     run.PolicyVersionID,
		DefinitionVersionID: run.DefinitionVersionID,
		WorkflowVersionID:   run.WorkflowVersionID,
		Nodes:               nodeViews,
		ErrorCode:           run.ErrorCode,
		DeadlineAt:          run.DeadlineAt,
		StartedAt:           run.StartedAt,
		FinishedAt:          run.FinishedAt,
		TerminalReason:      run.TerminalReason,
		Retryable:           run.Retryable,
	}
}

func HashString(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
