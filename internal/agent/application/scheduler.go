package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	d "InkLore/internal/agent/domain"
	"InkLore/internal/platform/jobs"
	"InkLore/internal/platform/uow"

	"github.com/google/uuid"
)

const (
	MaxConcurrentNodesPerRun     = 4
	MaxTAOIterations             = 15
	MaxConsecutiveDuplicateCalls = 3
	HeartbeatTick                = 10 * time.Second
)

// Scheduler drives worker execution for agent runs via platform_jobs and TAO loop.
type Scheduler struct {
	repo    d.RunRepository
	govRepo d.GovernanceRepository
	jobs    d.JobsPort
	gateway d.ModelGatewayPort
	tools   d.ToolExecutionPort
	runs    *RunService
	manager *uow.Manager
}

// NewScheduler constructs a Scheduler.
func NewScheduler(
	repo d.RunRepository,
	govRepo d.GovernanceRepository,
	jobs d.JobsPort,
	gateway d.ModelGatewayPort,
	tools d.ToolExecutionPort,
	runs *RunService,
	manager *uow.Manager,
) (*Scheduler, error) {
	if repo == nil {
		return nil, errors.New("agent scheduler: repo required")
	}
	if govRepo == nil || gateway == nil || tools == nil {
		return nil, errors.New("agent scheduler: governance, model gateway and tool executor required")
	}
	if jobs == nil {
		return nil, errors.New("agent scheduler: jobs port required")
	}
	if runs == nil {
		return nil, errors.New("agent scheduler: run service required")
	}
	if manager == nil {
		return nil, errors.New("agent scheduler: uow manager required")
	}
	return &Scheduler{
		repo:    &schedulerRunRepository{RunRepository: repo, manager: manager},
		govRepo: govRepo,
		jobs:    jobs,
		gateway: gateway,
		tools:   tools,
		runs:    runs,
		manager: manager,
	}, nil
}

// ClaimAndExecute claims a run from platform_jobs and executes its ready nodes.
// Returns true if a job was processed, false if queue was empty.
func (s *Scheduler) ClaimAndExecute(ctx context.Context, workerID string) (processed bool, resultErr error) {
	lease, err := s.jobs.ClaimRunJob(ctx, workerID)
	if errors.Is(err, jobs.ErrNoJob) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if lease == nil {
		return false, nil
	}
	// The worker lease has its own cancellable context so a failed heartbeat
	// stops in-flight model/tool work without cancelling the entire process.
	operationCtx, stopOperation := context.WithCancel(ctx)
	defer stopOperation()
	ctx = operationCtx
	leaseErrors := make(chan error, 1)
	defer func() {
		select {
		case leaseErr := <-leaseErrors:
			resultErr = errors.Join(resultErr, leaseErr)
		default:
		}
	}()

	tenantID := lease.TenantID
	runID := lease.RunID

	// Fetch run
	run, nodes, err := s.repo.GetRun(ctx, tenantID, runID)
	if err != nil {
		return false, err
	}

	// If already terminal, mark platform job complete and return
	if run.Status.IsTerminal() {
		return true, s.jobs.CompleteRunJob(ctx, lease, string(run.Status), "")
	}
	// An expired lease can leave an in-flight model or tool outcome unknown.
	// Never send the same external request again merely because a lease was reclaimed.
	if lease.Recovered {
		for _, n := range nodes {
			if n.Status == d.NodeStatusRunning {
				if err := s.repo.UpdateNodeStatus(ctx, tenantID, n.ID, n.Revision, d.NodeStatusFailed); err != nil {
					return true, err
				}
				return true, s.completeRun(ctx, lease, run.Revision, d.RunStatusFailed, "agent.execution_outcome_unknown")
			}
		}
	}

	// If queued, transition to running
	if run.Status == d.RunStatusQueued {
		updated, err := s.repo.UpdateRunStatus(ctx, tenantID, runID, run.Revision, d.RunStatusRunning)
		if err != nil {
			return false, err
		}
		run = updated
	}

	// Start 10s heartbeat loop in background
	stopHeartbeat := make(chan struct{})
	heartbeatDone := make(chan struct{})
	defer func() {
		close(stopHeartbeat)
		<-heartbeatDone
	}()

	var activeNodeIDs []string
	var activeMu sync.Mutex

	go func() {
		defer close(heartbeatDone)
		ticker := time.NewTicker(HeartbeatTick)
		defer ticker.Stop()

		for {
			select {
			case <-stopHeartbeat:
				return
			case <-ticker.C:
				if err := s.jobs.HeartbeatRunJob(ctx, lease); err != nil {
					leaseErrors <- fmt.Errorf("agent job heartbeat: %w", err)
					stopOperation()
					return
				}

				activeMu.Lock()
				nids := append([]string(nil), activeNodeIDs...)
				activeMu.Unlock()

				if len(nids) > 0 {
					if err := s.repo.HeartbeatNodes(ctx, tenantID, nids, workerID, time.Now().UTC().Add(30*time.Second)); err != nil {
						leaseErrors <- fmt.Errorf("agent node heartbeat: %w", err)
						stopOperation()
						return
					}
				}
			}
		}
	}()

	// Query ready nodes up to concurrency limit (4)
	readyNodes, err := s.repo.ListReadyNodes(ctx, tenantID, runID, MaxConcurrentNodesPerRun)
	if err != nil {
		return false, err
	}

	if len(readyNodes) == 0 {
		// A failed required node prevents downstream pending nodes from ever becoming ready.
		for _, n := range nodes {
			if n.Required && n.Status == d.NodeStatusFailed {
				return true, s.completeRun(ctx, lease, run.Revision, d.RunStatusFailed, "agent.node_failed")
			}
		}
		// Check if there are nodes waiting for approval
		hasWaitingApproval := false
		allFinished := true
		for _, n := range nodes {
			if n.Status == d.NodeStatusWaitingApproval {
				hasWaitingApproval = true
			}
			if !n.Status.IsTerminal() {
				allFinished = false
			}
		}

		if hasWaitingApproval {
			if run.Status != d.RunStatusWaitingApproval {
				if _, err := s.repo.UpdateRunStatus(ctx, tenantID, runID, run.Revision, d.RunStatusWaitingApproval); err != nil {
					return true, err
				}
			}
			return true, s.jobs.SuspendRunJob(ctx, lease, time.Now().UTC().Add(15*time.Minute))
		}

		if allFinished {
			state, code := d.RunStatusSucceeded, ""
			if len(nodes) == 0 {
				state, code = d.RunStatusFailed, "agent.invalid_configuration"
			}
			for _, n := range nodes {
				if n.Required && n.Status != d.NodeStatusSucceeded {
					state, code = d.RunStatusFailed, "agent.node_failed"
					break
				}
			}
			return true, s.completeRun(ctx, lease, run.Revision, state, code)
		}
		return true, s.jobs.SuspendRunJob(ctx, lease, time.Now().UTC().Add(time.Second))
	}

	// Execute ready nodes concurrently (up to MaxConcurrentNodesPerRun)
	var wg sync.WaitGroup
	nodeErrors := make([]error, len(readyNodes))

	for i, node := range readyNodes {
		wg.Add(1)
		go func(idx int, n d.AgentRunNode) {
			defer wg.Done()

			activeMu.Lock()
			activeNodeIDs = append(activeNodeIDs, n.ID)
			activeMu.Unlock()

			defer func() {
				activeMu.Lock()
				filtered := make([]string, 0, len(activeNodeIDs))
				for _, id := range activeNodeIDs {
					if id != n.ID {
						filtered = append(filtered, id)
					}
				}
				activeNodeIDs = filtered
				activeMu.Unlock()
			}()

			err := s.executeNodeTAO(ctx, lease, run, n)
			nodeErrors[idx] = err
		}(i, node)
	}

	wg.Wait()
	if err := errors.Join(nodeErrors...); err != nil {
		return true, err
	}

	// After running nodes, check run status
	allNodes, err := s.repo.ListNodes(ctx, tenantID, runID)
	if err != nil {
		return false, err
	}

	hasFailedRequired := false
	allRequiredSucceeded := true
	hasActiveOrReady := false
	hasWaitingApproval := false

	for _, n := range allNodes {
		if n.Required && n.Status == d.NodeStatusFailed {
			hasFailedRequired = true
		}
		if n.Required && n.Status != d.NodeStatusSucceeded {
			allRequiredSucceeded = false
		}
		if n.Status == d.NodeStatusRunning || n.Status == d.NodeStatusReady {
			hasActiveOrReady = true
		}
		if n.Status == d.NodeStatusWaitingApproval {
			hasWaitingApproval = true
		}
	}

	currentRun, err := s.repo.GetRunOnly(ctx, tenantID, runID)
	if err != nil {
		return true, err
	}
	if currentRun.Status.IsTerminal() {
		return true, s.jobs.CompleteRunJob(ctx, lease, string(currentRun.Status), "")
	}
	if hasFailedRequired {
		return true, s.completeRun(ctx, lease, currentRun.Revision, d.RunStatusFailed, "agent.node_failed")
	}
	allTerminal := true
	for _, n := range allNodes {
		if !n.Status.IsTerminal() {
			allTerminal = false
			break
		}
	}
	if allTerminal {
		state, code := d.RunStatusSucceeded, ""
		if !allRequiredSucceeded {
			state, code = d.RunStatusFailed, "agent.node_failed"
		}
		return true, s.completeRun(ctx, lease, currentRun.Revision, state, code)
	}
	if !hasActiveOrReady && hasWaitingApproval {
		if _, err := s.repo.UpdateRunStatus(ctx, tenantID, runID, currentRun.Revision, d.RunStatusWaitingApproval); err != nil {
			return true, err
		}
		return true, s.jobs.SuspendRunJob(ctx, lease, time.Now().UTC().Add(15*time.Minute))
	}
	return true, s.jobs.SuspendRunJob(ctx, lease, time.Now().UTC().Add(time.Second))
}

// executeNodeTAO executes the Think-Act-Observe cycle for a single node.
func (s *Scheduler) executeNodeTAO(ctx context.Context, lease *d.RunJobLease, run *d.AgentRun, node d.AgentRunNode) error {
	tenantID := lease.TenantID
	runID := run.ID
	workerID := lease.WorkerID

	// Claim node attempt: increments fencing_token, freezes job_epoch
	claimedNode, err := s.repo.ClaimNodeAttempt(ctx, tenantID, node.ID, node.Revision, workerID, time.Now().UTC().Add(30*time.Second), lease.LeaseEpoch)
	if err != nil {
		return err
	}
	node = *claimedNode

	// Persist node.started before producing model or tool output.
	payload, err := json.Marshal(d.NodeUpdatedPayload{NodeID: node.ID, Status: d.NodeStatusRunning})
	if err != nil {
		return err
	}
	if err := s.appendWorkerEvent(ctx, tenantID, runID, "node.updated", payload); err != nil {
		return err
	}

	// Loop history for loop detection
	callHistory := make([]string, 0, MaxTAOIterations)
	observation := ""
	userModel := ""
	if len(node.Checkpoint) > 0 {
		var checkpoint struct {
			UserModel   string `json:"user_model"`
			Observation string `json:"observation"`
		}
		if err := json.Unmarshal(node.Checkpoint, &checkpoint); err != nil {
			return err
		}
		userModel, observation = checkpoint.UserModel, checkpoint.Observation
	}

	// TAO Iterations
	for node.Iteration < MaxTAOIterations {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Check if run was cancelled in the meantime
		curRun, err := s.repo.GetRunOnly(ctx, tenantID, runID)
		if err == nil && (curRun.Status == d.RunStatusCancelling || curRun.Status.IsTerminal()) {
			_ = s.repo.UpdateNodeStatus(ctx, tenantID, node.ID, node.Revision, d.NodeStatusCancelled)
			return nil
		}

		// Phase: Think
		if node.DefinitionVersionID == nil || s.govRepo == nil {
			failCode := "agent.invalid_configuration"
			_ = s.repo.FinishNode(ctx, tenantID, node.ID, node.Revision, lease.LeaseEpoch, node.FencingToken, d.NodeStatusFailed, &failCode)
			return errors.New("agent definition version is unavailable")
		}
		defVer, err := s.govRepo.GetDefinitionVersion(ctx, tenantID, *node.DefinitionVersionID)
		if err != nil || defVer == nil || strings.TrimSpace(defVer.SystemInstruction) == "" {
			failCode := "agent.invalid_configuration"
			_ = s.repo.FinishNode(ctx, tenantID, node.ID, node.Revision, lease.LeaseEpoch, node.FencingToken, d.NodeStatusFailed, &failCode)
			if err != nil {
				return err
			}
			return errors.New("agent definition instruction is unavailable")
		}
		sysInstruction := defVer.SystemInstruction
		allowedTools := make(map[string]d.ToolView, len(defVer.ToolVersionIDs))
		toolStatus, ok := s.tools.(interface {
			ToolExecutionStatus(context.Context, string, d.ToolView) string
		})
		if !ok {
			return errors.New("agent tool executor cannot verify tool availability")
		}
		for _, versionID := range defVer.ToolVersionIDs {
			tool, version, err := s.govRepo.GetToolVersion(ctx, tenantID, versionID)
			if err != nil {
				return err
			}
			if tool == nil || version == nil || tool.ToolKey == "" || version.ID != versionID {
				return errors.New("frozen agent tool version is unavailable")
			}
			view := d.ToolView{
				ID: tool.ID, ToolKey: tool.ToolKey, Title: tool.Title,
				Enabled: tool.Enabled, CurrentVersionID: version.ID,
				InputSchema: version.InputSchema, OutputSchema: version.OutputSchema,
				SideEffect: version.SideEffect, ApprovalRequired: version.ApprovalRequired,
				MCPServerID: version.MCPServerID, RemoteToolName: version.RemoteToolName,
				TimeoutSeconds: version.TimeoutSeconds,
			}
			if toolStatus.ToolExecutionStatus(ctx, tenantID, view) == d.ToolExecutionAvailable {
				allowedTools[versionID] = view
			}
		}

		genInput := d.ModelGenerateInput{
			TenantID:          tenantID,
			RunID:             runID,
			NodeKey:           node.NodeKey,
			TaskKey:           run.TaskKey,
			PolicyVersionID:   run.PolicyVersionID,
			Model:             userModel,
			SystemInstruction: sysInstruction,
			UserInstruction:   run.Instruction,
			ContextText:       observation,
			MaxOutputTokens:   0,
		}
		for _, versionID := range defVer.ToolVersionIDs {
			if tool, ok := allowedTools[versionID]; ok {
				genInput.Tools = append(genInput.Tools, tool)
			}
		}

		var genOutput *d.ModelGenerateOutput
		if s.gateway != nil {
			genOutput, err = s.gateway.Generate(ctx, genInput)
			if err != nil {
				failCode := "agent.execution_outcome_unknown"
				_ = s.repo.FinishNode(ctx, tenantID, node.ID, node.Revision, lease.LeaseEpoch, node.FencingToken, d.NodeStatusFailed, &failCode)
				return err
			}
		} else {
			failCode := "llm_provider_not_configured"
			_ = s.repo.FinishNode(ctx, tenantID, node.ID, node.Revision, lease.LeaseEpoch, node.FencingToken, d.NodeStatusFailed, &failCode)
			return errors.New("agent model gateway is not configured")
		}

		// Record reasoning summary if provided
		if genOutput.ReasoningSummary != nil && *genOutput.ReasoningSummary != "" {
			seq, err := s.repo.AllocateNextSeq(ctx, tenantID, runID)
			if err == nil {
				payload, _ := json.Marshal(d.ReasoningSummaryPayload{Summary: *genOutput.ReasoningSummary})
				_ = s.repo.AppendEvent(ctx, tenantID, &d.AgentRunEvent{
					ID:        uuid.NewString(),
					TenantID:  tenantID,
					CreatedAt: time.Now().UTC(),
					UpdatedAt: time.Now().UTC(),
					RunID:     runID,
					Seq:       seq,
					EventName: "reasoning.summary",
					Payload:   payload,
					ExpiresAt: time.Now().UTC().Add(7 * 24 * time.Hour),
				})
			}
		}

		// Record usage updated
		if genOutput.InputTokens > 0 || genOutput.OutputTokens > 0 {
			seq, err := s.repo.AllocateNextSeq(ctx, tenantID, runID)
			if err == nil {
				payload, _ := json.Marshal(d.UsageUpdatedPayload{
					InputTokens:  genOutput.InputTokens,
					OutputTokens: genOutput.OutputTokens,
					CostMicros:   genOutput.CostMicros,
					Currency:     "CNY",
					Estimated:    false,
				})
				_ = s.repo.AppendEvent(ctx, tenantID, &d.AgentRunEvent{
					ID:        uuid.NewString(),
					TenantID:  tenantID,
					CreatedAt: time.Now().UTC(),
					UpdatedAt: time.Now().UTC(),
					RunID:     runID,
					Seq:       seq,
					EventName: "usage.updated",
					Payload:   payload,
					ExpiresAt: time.Now().UTC().Add(7 * 24 * time.Hour),
				})
			}
		}

		// Reject any model-invented or disabled tool before emitting a tool event,
		// creating an approval, or dispatching a side effect.
		for _, tc := range genOutput.ToolCalls {
			tool, ok := allowedTools[tc.ToolVersion]
			if tc.ToolVersion == "" || !ok || tool.ToolKey != tc.ToolKey {
				failCode := "agent.invalid_configuration"
				if err := s.repo.FinishNode(ctx, tenantID, node.ID, node.Revision, lease.LeaseEpoch, node.FencingToken, d.NodeStatusFailed, &failCode); err != nil {
					return err
				}
				return errors.New("model requested a tool outside the frozen executable set")
			}
		}

		// Case 1: Model returned final text (no tool calls)
		if len(genOutput.ToolCalls) == 0 {
			text := ""
			if genOutput.FinalText != nil {
				text = *genOutput.FinalText
			}

			// Save suggestion artifact
			artID := uuid.NewString()
			cHash := HashString(text)
			sourceRev := int64(1)
			if run.SourceRevision != nil {
				sourceRev = *run.SourceRevision
			}

			artifact := &d.AgentArtifact{
				ID:               artID,
				TenantID:         tenantID,
				CreatedAt:        time.Now().UTC(),
				UpdatedAt:        time.Now().UTC(),
				RunID:            runID,
				NodeID:           node.ID,
				Kind:             d.ArtifactKind(mapTaskKindToArtifact(run.TaskKind)),
				TargetResourceID: run.ResourceID,
				SourceRevision:   sourceRev,
				ContentHash:      cHash,
				TextContent:      &text,
				IsPartial:        false,
			}
			deltaPayload, err := json.Marshal(d.TextDeltaPayload{ArtifactID: artID, Delta: text})
			if err != nil {
				return err
			}
			if err := s.manager.Within(ctx, func(tx context.Context) error {
				if err := s.repo.CreateArtifact(tx, tenantID, artifact); err != nil {
					return err
				}
				seq, err := s.repo.AllocateNextSeq(tx, tenantID, runID)
				if err != nil {
					return err
				}
				now := time.Now().UTC()
				if err := s.repo.AppendEvent(tx, tenantID, &d.AgentRunEvent{
					ID: uuid.NewString(), TenantID: tenantID, RunID: runID,
					CreatedAt: now, UpdatedAt: now, ExpiresAt: now.Add(7 * 24 * time.Hour),
					Seq: seq, EventName: "text.delta", Payload: deltaPayload,
				}); err != nil {
					return err
				}
				return s.repo.FinishNode(tx, tenantID, node.ID, node.Revision, lease.LeaseEpoch, node.FencingToken, d.NodeStatusSucceeded, nil)
			}); err != nil {
				return err
			}
			return s.activateDownstreamNodes(ctx, tenantID, runID, node.NodeKey)
		}

		// Case 2: Model returned tool calls (Phase: Act / Observe)
		requiresApproval := false
		var pendingApprovalTool *d.ToolCall

		for _, tc := range genOutput.ToolCalls {
			// Check loop detection
			callKey := fmt.Sprintf("%s:%s", tc.ToolKey, HashString(string(tc.Arguments)))
			dupCount := 0
			for _, prev := range callHistory {
				if prev == callKey {
					dupCount++
				}
			}
			if dupCount >= MaxConsecutiveDuplicateCalls {
				failCode := "agent.loop_detected"
				_ = s.repo.FinishNode(ctx, tenantID, node.ID, node.Revision, lease.LeaseEpoch, node.FencingToken, d.NodeStatusFailed, &failCode)
				return d.ErrLoopDetected(map[string]any{"tool_key": tc.ToolKey})
			}
			callHistory = append(callHistory, callKey)

			// Approval is determined from the frozen tool version, never from a mutable head.
			tool := allowedTools[tc.ToolVersion]
			if tool.ApprovalRequired || tool.SideEffect == string(d.SideEffectExternal) {
				requiresApproval = true
				tCopy := tc
				pendingApprovalTool = &tCopy
				break
			}
		}

		// If approval is required: gate execution and pause node
		if requiresApproval && pendingApprovalTool != nil {
			execID := uuid.NewString()
			appID := uuid.NewString()
			now := time.Now().UTC()
			expiresAt := now.Add(15 * time.Minute)
			argHash := HashString(string(pendingApprovalTool.Arguments))
			sourceRev := int64(1)
			if run.SourceRevision != nil {
				sourceRev = *run.SourceRevision
			}

			execution := &d.AgentExecution{
				ID:              execID,
				TenantID:        tenantID,
				CreatedAt:       now,
				UpdatedAt:       now,
				Revision:        1,
				RunID:           runID,
				NodeID:          node.ID,
				ToolVersionID:   pendingApprovalTool.ToolVersion,
				ExecutionKey:    uuid.NewString(),
				Iteration:       node.Iteration + 1,
				CallIndex:       0,
				ArgumentsHash:   argHash,
				FrozenArguments: pendingApprovalTool.Arguments,
				Status:          d.ExecutionStatusPrepared,
				FencingToken:    node.FencingToken,
				JobEpoch:        lease.LeaseEpoch,
			}
			_ = s.repo.CreateExecution(ctx, tenantID, execution)

			approval := &d.AgentApproval{
				ID:             appID,
				TenantID:       tenantID,
				CreatedAt:      now,
				UpdatedAt:      now,
				Revision:       1,
				OwnerUserID:    run.OwnerUserID,
				RunID:          runID,
				NodeID:         node.ID,
				ExecutionID:    execID,
				ToolVersionID:  pendingApprovalTool.ToolVersion,
				ArgumentsHash:  argHash,
				SourceRevision: sourceRev,
				Status:         d.ApprovalStatusPending,
				ExpiresAt:      expiresAt,
			}
			_ = s.repo.CreateApproval(ctx, tenantID, approval)

			// Transition node to waiting_approval
			_ = s.repo.SaveNodeCheckpoint(ctx, tenantID, node.ID, node.Revision, lease.LeaseEpoch, node.FencingToken, d.NodePhaseThink, node.Iteration, d.NodeStatusWaitingApproval, json.RawMessage("{}"), nil)

			// Emit approval.required event
			seq, err := s.repo.AllocateNextSeq(ctx, tenantID, runID)
			if err == nil {
				payload, _ := json.Marshal(d.ApprovalRequiredPayload{
					ApprovalID: appID,
					NodeID:     node.ID,
					ExpiresAt:  expiresAt,
				})
				_ = s.repo.AppendEvent(ctx, tenantID, &d.AgentRunEvent{
					ID:        uuid.NewString(),
					TenantID:  tenantID,
					CreatedAt: now,
					UpdatedAt: now,
					RunID:     runID,
					Seq:       seq,
					EventName: "approval.required",
					Payload:   payload,
					ExpiresAt: now.Add(7 * 24 * time.Hour),
				})
			}

			// Node pauses execution until approval is decided
			return nil
		}

		// Phase: Act (Execute automated tools)
		for _, tc := range genOutput.ToolCalls {
			invID := tc.InvocationID
			if invID == "" {
				invID = uuid.NewString()
			}

			// Persist the start event before dispatching the registered handler.
			startedPayload, err := json.Marshal(d.ToolStartedPayload{NodeID: node.ID, ToolID: tc.ToolVersion, InvocationID: invID})
			if err != nil {
				return err
			}
			if err := s.appendWorkerEvent(ctx, tenantID, runID, "tool.started", startedPayload); err != nil {
				return err
			}

			var toolOutput *d.ToolCallOutput
			var toolErr error
			if s.tools == nil {
				toolErr = errors.New("agent tool executor is not configured")
			} else {
				toolOutput, toolErr = s.tools.ExecuteTool(ctx, d.ToolCallInput{
					TenantID:      tenantID,
					RunID:         runID,
					NodeID:        node.ID,
					ExecutionKey:  uuid.NewString(),
					ToolKey:       tc.ToolKey,
					ToolVersionID: tc.ToolVersion,
					Arguments:     tc.Arguments,
				})
			}
			if toolErr != nil || toolOutput == nil {
				failCode := "agent.execution_outcome_unknown"
				_ = s.repo.FinishNode(ctx, tenantID, node.ID, node.Revision, lease.LeaseEpoch, node.FencingToken, d.NodeStatusFailed, &failCode)
				return errors.New("agent tool execution failed or returned no result")
			}

			// Persist the executor's actual result; never send tool contents in SSE.
			completedPayload, err := json.Marshal(d.ToolCompletedPayload{
				NodeID: node.ID, InvocationID: invID, Success: toolOutput.Success,
				ResultRef: toolOutput.ResultFileID, ErrorCode: toolOutput.ErrorCode,
			})
			if err != nil {
				return err
			}
			if err := s.appendWorkerEvent(ctx, tenantID, runID, "tool.completed", completedPayload); err != nil {
				return err
			}
			if !toolOutput.Success {
				failCode := "agent.execution_outcome_unknown"
				if toolOutput.ErrorCode != nil && strings.TrimSpace(*toolOutput.ErrorCode) != "" {
					failCode = *toolOutput.ErrorCode
				}
				if err := s.repo.FinishNode(ctx, tenantID, node.ID, node.Revision, lease.LeaseEpoch, node.FencingToken, d.NodeStatusFailed, &failCode); err != nil {
					return err
				}
				return nil
			}
			// Treat tool output as untrusted data; preserve it for the next Think and
			// crash recovery without placing its contents in SSE events.
			observation = "Untrusted tool result (" + tc.ToolKey + "): " + toolOutput.ResultText
		}

		// Advance iteration and persist the Observe checkpoint before another model call.
		checkpoint, err := json.Marshal(struct {
			UserModel   string `json:"user_model,omitempty"`
			Observation string `json:"observation"`
		}{UserModel: userModel, Observation: observation})
		if err != nil {
			return err
		}
		nextIteration := node.Iteration + 1
		if err := s.repo.SaveNodeCheckpoint(ctx, tenantID, node.ID, node.Revision, lease.LeaseEpoch, node.FencingToken, d.NodePhaseThink, nextIteration, d.NodeStatusRunning, checkpoint, nil); err != nil {
			return err
		}
		node.Iteration = nextIteration
		node.Revision++
		node.Checkpoint = checkpoint
	}

	// Exhausting TAO iterations is a failed node, not a successful answer.
	failCode := "agent.loop_detected"
	if err := s.repo.FinishNode(ctx, tenantID, node.ID, node.Revision, lease.LeaseEpoch, node.FencingToken, d.NodeStatusFailed, &failCode); err != nil {
		return err
	}
	return nil
}

// activateDownstreamNodes unlocks nodes whose dependencies have all succeeded.
func (s *Scheduler) activateDownstreamNodes(ctx context.Context, tenantID, runID, completedNodeKey string) error {
	nodes, err := s.repo.ListNodes(ctx, tenantID, runID)
	if err != nil {
		return err
	}

	succeededKeys := make(map[string]bool)
	for _, n := range nodes {
		if n.Status == d.NodeStatusSucceeded {
			succeededKeys[n.NodeKey] = true
		}
	}
	succeededKeys[completedNodeKey] = true

	for _, n := range nodes {
		if n.Status == d.NodeStatusPending {
			allSatisfied := true
			for _, dep := range n.DependsOn {
				if !succeededKeys[dep] {
					allSatisfied = false
					break
				}
			}
			if allSatisfied {
				if err := s.repo.UpdateNodeStatus(ctx, tenantID, n.ID, n.Revision, d.NodeStatusReady); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func mapTaskKindToArtifact(taskKind string) string {
	switch taskKind {
	case string(d.TaskKindChapterDraft):
		return string(d.ArtifactKindChapterDraft)
	case string(d.TaskKindChapterRewrite), string(d.TaskKindComplianceRewrite):
		return string(d.ArtifactKindRewriteSuggestion)
	case string(d.TaskKindOutlineSuggest):
		return string(d.ArtifactKindOutlineSuggestion)
	case string(d.TaskKindWorldviewSuggest):
		return string(d.ArtifactKindWorldviewSuggest)
	case string(d.TaskKindInspirationAnalyze):
		return string(d.ArtifactKindAnalysisResult)
	case string(d.TaskKindComplianceAudit):
		return string(d.ArtifactKindAuditResult)
	default:
		return string(d.ArtifactKindChapterDraft)
	}
}

// appendWorkerEvent keeps sequence allocation and SSE persistence atomic.
func (s *Scheduler) appendWorkerEvent(ctx context.Context, tenantID, runID, eventName string, payload json.RawMessage) error {
	return s.manager.Within(ctx, func(tx context.Context) error {
		seq, err := s.repo.AllocateNextSeq(tx, tenantID, runID)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		return s.repo.AppendEvent(tx, tenantID, &d.AgentRunEvent{
			ID: uuid.NewString(), TenantID: tenantID, RunID: runID,
			CreatedAt: now, UpdatedAt: now, ExpiresAt: now.Add(7 * 24 * time.Hour),
			Seq: seq, EventName: eventName, Payload: payload,
		})
	})
}

// completeRun atomically closes the claimed platform job and its Agent run.
// RunService.Terminate is for commands and marks the job cancelled; it is not
// suitable for a worker's successful/failed completion.
func (s *Scheduler) completeRun(ctx context.Context, lease *d.RunJobLease, revision int64, status d.RunStatus, code string) error {
	return s.manager.Within(ctx, func(tx context.Context) error {
		if err := s.jobs.CompleteRunJob(tx, lease, string(status), code); err != nil {
			return err
		}
		run, err := s.repo.TerminateRun(tx, lease.TenantID, lease.RunID, revision, status, "", code, false, nil)
		if err != nil {
			return err
		}
		nodes, err := s.repo.ListNodes(tx, lease.TenantID, lease.RunID)
		if err != nil {
			return err
		}
		for _, n := range nodes {
			if !n.Status.IsTerminal() {
				if err := s.repo.UpdateNodeStatus(tx, lease.TenantID, n.ID, n.Revision, d.NodeStatusCancelled); err != nil {
					return err
				}
			}
		}
		seq, err := s.repo.AllocateNextSeq(tx, lease.TenantID, lease.RunID)
		if err != nil {
			return err
		}
		name := "run.succeeded"
		payload, err := json.Marshal(d.RunSucceededPayload{ArtifactIDs: []string{}, Revision: run.Revision})
		if status == d.RunStatusFailed {
			name = "run.failed"
			payload, err = json.Marshal(d.RunFailedPayload{Code: code, Retryable: false, Revision: run.Revision})
		}
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		return s.repo.AppendEvent(tx, lease.TenantID, &d.AgentRunEvent{
			ID: uuid.NewString(), TenantID: lease.TenantID, RunID: lease.RunID,
			CreatedAt: now, UpdatedAt: now, ExpiresAt: now.Add(7 * 24 * time.Hour),
			Seq: seq, EventName: name, Payload: payload,
		})
	})
}

// schedulerRunRepository opens short transactions only for worker writes. Model and
// tool calls remain outside transactions; existing request-scoped repositories keep
// their strict Require(ctx) contract.
type schedulerRunRepository struct {
	d.RunRepository
	manager *uow.Manager
}

func (r *schedulerRunRepository) UpdateRunStatus(ctx context.Context, tenantID, runID string, revision int64, status d.RunStatus) (*d.AgentRun, error) {
	var result *d.AgentRun
	err := r.manager.Within(ctx, func(tx context.Context) (err error) {
		result, err = r.RunRepository.UpdateRunStatus(tx, tenantID, runID, revision, status)
		return err
	})
	return result, err
}
func (r *schedulerRunRepository) ClaimNodeAttempt(ctx context.Context, tenantID, nodeID string, revision int64, workerID string, until time.Time, epoch int64) (*d.AgentRunNode, error) {
	var result *d.AgentRunNode
	err := r.manager.Within(ctx, func(tx context.Context) (err error) {
		result, err = r.RunRepository.ClaimNodeAttempt(tx, tenantID, nodeID, revision, workerID, until, epoch)
		return err
	})
	return result, err
}
func (r *schedulerRunRepository) HeartbeatNodes(ctx context.Context, tenantID string, ids []string, workerID string, until time.Time) error {
	return r.manager.Within(ctx, func(tx context.Context) error {
		return r.RunRepository.HeartbeatNodes(tx, tenantID, ids, workerID, until)
	})
}
func (r *schedulerRunRepository) AllocateNextSeq(ctx context.Context, tenantID, runID string) (int64, error) {
	var seq int64
	err := r.manager.Within(ctx, func(tx context.Context) (err error) {
		seq, err = r.RunRepository.AllocateNextSeq(tx, tenantID, runID)
		return err
	})
	return seq, err
}
func (r *schedulerRunRepository) AppendEvent(ctx context.Context, tenantID string, event *d.AgentRunEvent) error {
	return r.manager.Within(ctx, func(tx context.Context) error {
		return r.RunRepository.AppendEvent(tx, tenantID, event)
	})
}
func (r *schedulerRunRepository) FinishNode(ctx context.Context, tenantID, nodeID string, revision, epoch, fence int64, status d.NodeStatus, code *string) error {
	return r.manager.Within(ctx, func(tx context.Context) error {
		return r.RunRepository.FinishNode(tx, tenantID, nodeID, revision, epoch, fence, status, code)
	})
}
func (r *schedulerRunRepository) SaveNodeCheckpoint(ctx context.Context, tenantID, nodeID string, revision, epoch, fence int64, phase d.NodePhase, iteration int, status d.NodeStatus, checkpoint json.RawMessage, code *string) error {
	return r.manager.Within(ctx, func(tx context.Context) error {
		return r.RunRepository.SaveNodeCheckpoint(tx, tenantID, nodeID, revision, epoch, fence, phase, iteration, status, checkpoint, code)
	})
}
func (r *schedulerRunRepository) UpdateNodeStatus(ctx context.Context, tenantID, nodeID string, revision int64, status d.NodeStatus) error {
	return r.manager.Within(ctx, func(tx context.Context) error {
		return r.RunRepository.UpdateNodeStatus(tx, tenantID, nodeID, revision, status)
	})
}
func (r *schedulerRunRepository) CreateArtifact(ctx context.Context, tenantID string, artifact *d.AgentArtifact) error {
	return r.manager.Within(ctx, func(tx context.Context) error {
		return r.RunRepository.CreateArtifact(tx, tenantID, artifact)
	})
}
func (r *schedulerRunRepository) CreateExecution(ctx context.Context, tenantID string, execution *d.AgentExecution) error {
	return r.manager.Within(ctx, func(tx context.Context) error {
		return r.RunRepository.CreateExecution(tx, tenantID, execution)
	})
}
func (r *schedulerRunRepository) CreateApproval(ctx context.Context, tenantID string, approval *d.AgentApproval) error {
	return r.manager.Within(ctx, func(tx context.Context) error {
		return r.RunRepository.CreateApproval(tx, tenantID, approval)
	})
}
