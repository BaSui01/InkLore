package application

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	d "InkLore/internal/agent/domain"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/scope"
	"InkLore/internal/platform/uow"

	"github.com/google/uuid"
)

// ApprovalService manages human-in-the-loop approvals for agent tool invocations.
type ApprovalService struct {
	repo      d.RunRepository
	runs      *RunService
	jobs      d.JobsPort
	resources d.TaskResourcePort
	manager   *uow.Manager
}

// NewApprovalService constructs an ApprovalService.
func NewApprovalService(
	repo d.RunRepository,
	runs *RunService,
	jobs d.JobsPort,
	resources d.TaskResourcePort,
	manager *uow.Manager,
) (*ApprovalService, error) {
	if repo == nil {
		return nil, errors.New("agent approval service: repository required")
	}
	if runs == nil {
		return nil, errors.New("agent approval service: run service required")
	}
	if manager == nil {
		return nil, errors.New("agent approval service: uow manager required")
	}
	return &ApprovalService{
		repo:      repo,
		runs:      runs,
		jobs:      jobs,
		resources: resources,
		manager:   manager,
	}, nil
}

// AuthorizeCreator verifies that the principal is an authenticated creator.
func (s *ApprovalService) AuthorizeCreator(p identity.Principal) error {
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

// ListApprovals lists pending approvals for the creator.
func (s *ApprovalService) ListApprovals(ctx context.Context, p identity.Principal, query d.ListQuery) ([]d.ApprovalView, *string, bool, error) {
	if err := s.AuthorizeCreator(p); err != nil {
		return nil, nil, false, err
	}
	items, cursor, more, err := s.repo.ListPendingApprovals(ctx, p.TenantID, p.UserID, query)
	if err != nil {
		return nil, nil, false, err
	}
	visible := make([]d.ApprovalView, 0, len(items))
	for _, item := range items {
		if err := s.runs.AuthorizeRun(ctx, p, item.RunID); err != nil {
			if hiddenResource(err) {
				continue
			}
			return nil, nil, false, err
		}
		visible = append(visible, item)
	}
	return visible, cursor, more, nil
}

// GetApproval retrieves a single approval by ID.
func (s *ApprovalService) GetApproval(ctx context.Context, p identity.Principal, approvalID string) (*d.ApprovalView, error) {
	if err := s.AuthorizeCreator(p); err != nil {
		return nil, err
	}
	if !scope.ValidID(approvalID) {
		return nil, d.ErrValidationFailed(map[string]any{"reason": "invalid approval_id"})
	}

	app, exec, err := s.repo.GetApproval(ctx, p.TenantID, approvalID)
	if err != nil {
		return nil, err
	}
	if app.OwnerUserID != p.UserID && p.Audience != "admin" {
		return nil, d.ErrNotFound()
	}

	if err := s.runs.AuthorizeRun(ctx, p, app.RunID); err != nil {
		return nil, err
	}
	argsJSON := ""
	if exec != nil && len(exec.FrozenArguments) > 0 {
		argsJSON = string(exec.FrozenArguments)
	}

	return &d.ApprovalView{
		ID:             app.ID,
		Revision:       app.Revision,
		CreatedAt:      app.CreatedAt,
		UpdatedAt:      app.UpdatedAt,
		RunID:          app.RunID,
		NodeID:         app.NodeID,
		OwnerUserID:    app.OwnerUserID,
		ToolVersionID:  app.ToolVersionID,
		ArgumentsHash:  app.ArgumentsHash,
		ArgumentsJSON:  argsJSON,
		SourceRevision: app.SourceRevision,
		Status:         app.Status,
		ExpiresAt:      app.ExpiresAt,
		DecidedAt:      app.DecidedAt,
		DecisionReason: app.DecisionReason,
	}, nil
}

// Approve consumes a pending approval in a single CAS transaction and resumes the run.
func (s *ApprovalService) Approve(
	ctx context.Context,
	p identity.Principal,
	approvalID string,
	in d.ApprovalDecisionInput,
	idempotencyKey string,
) (*d.ApprovalView, error) {
	if err := s.AuthorizeCreator(p); err != nil {
		return nil, err
	}
	if !scope.ValidID(approvalID) {
		return nil, d.ErrValidationFailed(map[string]any{"reason": "invalid approval_id"})
	}

	app, exec, err := s.repo.GetApproval(ctx, p.TenantID, approvalID)
	if err != nil {
		return nil, err
	}
	if app.OwnerUserID != p.UserID {
		return nil, d.ErrNotFound()
	}

	// Invariant: once not pending, cannot be approved
	if app.Status != d.ApprovalStatusPending {
		if app.Status == d.ApprovalStatusConsumed {
			return nil, d.ErrApprovalConsumed(map[string]any{"status": app.Status})
		}
		if app.Status == d.ApprovalStatusExpired {
			return nil, d.ErrApprovalExpired(map[string]any{"status": app.Status})
		}
		return nil, d.ErrApprovalConsumed(map[string]any{"status": app.Status})
	}

	// Expiry check (PG now() / server time)
	now := time.Now().UTC()
	if now.After(app.ExpiresAt) {
		_ = s.repo.CancelApproval(ctx, p.TenantID, approvalID, app.Revision, "expired")
		return nil, d.ErrApprovalExpired(map[string]any{"expires_at": app.ExpiresAt})
	}

	// Revision check
	if in.ExpectedRevision > 0 && in.ExpectedRevision != app.Revision {
		return nil, d.ErrVersionConflict(map[string]any{
			"current_revision":  app.Revision,
			"expected_revision": in.ExpectedRevision,
		})
	}

	// Arguments hash check
	if in.ArgumentsHash != "" && in.ArgumentsHash != app.ArgumentsHash {
		return nil, d.ErrApprovalBindingMismatch(map[string]any{
			"expected_hash": app.ArgumentsHash,
			"provided_hash": in.ArgumentsHash,
		})
	}

	// Source revision check & verification
	run, err := s.repo.GetRunOnly(ctx, p.TenantID, app.RunID)
	if err != nil {
		return nil, err
	}

	sourceUnchanged := true
	if s.resources != nil && run.ResourceID != nil && run.ResourceType != nil {
		resRef := &d.ResourceRef{
			ResourceID:     *run.ResourceID,
			ResourceType:   *run.ResourceType,
			SourceRevision: app.SourceRevision,
			NovelID:        run.NovelID,
			SourceHash:     run.SourceHash,
		}
		expectedHash := ""
		if run.SourceHash != nil {
			expectedHash = *run.SourceHash
		}
		unchanged, err := s.resources.VerifySourceUnchanged(ctx, p, run.TaskKind, resRef, app.SourceRevision, expectedHash)
		if err != nil || !unchanged {
			sourceUnchanged = false
		}
	} else if in.SourceRevision > 0 && in.SourceRevision != app.SourceRevision {
		sourceUnchanged = false
	}

	// If source changed, execute committed error: cancel ticket, fail node and run, return 409
	if !sourceUnchanged {
		_ = s.manager.Within(ctx, func(tx context.Context) error {
			_ = s.repo.CancelApproval(tx, p.TenantID, approvalID, app.Revision, "source_changed")
			node, err := s.repo.GetNodeByID(tx, p.TenantID, app.RunID, app.NodeID)
			if err == nil {
				_ = s.repo.UpdateNodeStatus(tx, p.TenantID, node.ID, node.Revision, d.NodeStatusFailed)
			}
			_, _ = s.runs.Terminate(tx, p.TenantID, app.RunID, run.Revision, d.RunStatusFailed, "", "agent.approval_source_changed", false, nil)
			return nil
		})
		return nil, d.ErrApprovalSourceChanged(map[string]any{
			"approval_id":     approvalID,
			"source_revision": app.SourceRevision,
		})
	}

	// Single CAS transaction:
	// 1. Consume approval (pending -> consumed)
	// 2. Resume node (waiting_approval -> ready with continuation 'act')
	// 3. Emit node.updated event
	// 4. If run is waiting_approval, restore run to running
	// 5. Wake platform job
	err = s.manager.Within(ctx, func(tx context.Context) error {
		if err := s.repo.ConsumeApproval(tx, p.TenantID, approvalID, app.Revision, in.Reason); err != nil {
			return err
		}

		node, err := s.repo.GetNodeByID(tx, p.TenantID, app.RunID, app.NodeID)
		if err != nil {
			return err
		}
		if err := s.repo.UpdateNodeStatus(tx, p.TenantID, node.ID, node.Revision, d.NodeStatusReady); err != nil {
			return err
		}

		// Emit node.updated event
		seq, err := s.repo.AllocateNextSeq(tx, p.TenantID, app.RunID)
		if err != nil {
			return err
		}
		nodePayload, _ := json.Marshal(d.NodeUpdatedPayload{
			NodeID: node.ID,
			Status: d.NodeStatusReady,
		})
		_ = s.repo.AppendEvent(tx, p.TenantID, &d.AgentRunEvent{
			ID:        uuid.NewString(),
			TenantID:  p.TenantID,
			CreatedAt: now,
			UpdatedAt: now,
			RunID:     app.RunID,
			Seq:       seq,
			EventName: "node.updated",
			Payload:   nodePayload,
			ExpiresAt: now.Add(7 * 24 * time.Hour),
		})

		// Check if run was waiting_approval
		if run.Status == d.RunStatusWaitingApproval {
			if _, err := s.repo.UpdateRunStatus(tx, p.TenantID, run.ID, run.Revision, d.RunStatusRunning); err != nil {
				return err
			}
		}

		// Wake the platform job so a worker can claim it and execute Act phase
		if s.jobs != nil && run.PlatformJobID != "" {
			_ = s.jobs.WakeRunJob(tx, p.TenantID, run.PlatformJobID)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	if err := s.runs.AuthorizeRun(ctx, p, app.RunID); err != nil {
		return nil, err
	}
	argsJSON := ""
	if exec != nil && len(exec.FrozenArguments) > 0 {
		argsJSON = string(exec.FrozenArguments)
	}

	return &d.ApprovalView{
		ID:             app.ID,
		Revision:       app.Revision + 1,
		CreatedAt:      app.CreatedAt,
		UpdatedAt:      now,
		RunID:          app.RunID,
		NodeID:         app.NodeID,
		OwnerUserID:    app.OwnerUserID,
		ToolVersionID:  app.ToolVersionID,
		ArgumentsHash:  app.ArgumentsHash,
		ArgumentsJSON:  argsJSON,
		SourceRevision: app.SourceRevision,
		Status:         d.ApprovalStatusConsumed,
		ExpiresAt:      app.ExpiresAt,
		DecidedAt:      &now,
		DecisionReason: &in.Reason,
	}, nil
}

// Reject rejects the approval ticket and cancels the entire run.
func (s *ApprovalService) Reject(
	ctx context.Context,
	p identity.Principal,
	approvalID string,
	in d.ApprovalDecisionInput,
	idempotencyKey string,
) (*d.ApprovalView, error) {
	if err := s.AuthorizeCreator(p); err != nil {
		return nil, err
	}
	if !scope.ValidID(approvalID) {
		return nil, d.ErrValidationFailed(map[string]any{"reason": "invalid approval_id"})
	}

	app, exec, err := s.repo.GetApproval(ctx, p.TenantID, approvalID)
	if err != nil {
		return nil, err
	}
	if app.OwnerUserID != p.UserID {
		return nil, d.ErrNotFound()
	}

	if app.Status != d.ApprovalStatusPending {
		return nil, d.ErrApprovalConsumed(map[string]any{"status": app.Status})
	}

	now := time.Now().UTC()
	if now.After(app.ExpiresAt) {
		return nil, d.ErrApprovalExpired(map[string]any{"expires_at": app.ExpiresAt})
	}

	run, err := s.repo.GetRunOnly(ctx, p.TenantID, app.RunID)
	if err != nil {
		return nil, err
	}

	err = s.manager.Within(ctx, func(tx context.Context) error {
		if err := s.repo.RejectApproval(tx, p.TenantID, approvalID, app.Revision, in.Reason); err != nil {
			return err
		}

		// Reject cancels the entire run with reason "approval_rejected"
		_, err := s.runs.Terminate(tx, p.TenantID, app.RunID, run.Revision, d.RunStatusCancelled, "approval_rejected", "", false, nil)
		return err
	})
	if err != nil {
		return nil, err
	}

	if err := s.runs.AuthorizeRun(ctx, p, app.RunID); err != nil {
		return nil, err
	}
	argsJSON := ""
	if exec != nil && len(exec.FrozenArguments) > 0 {
		argsJSON = string(exec.FrozenArguments)
	}

	return &d.ApprovalView{
		ID:             app.ID,
		Revision:       app.Revision + 1,
		CreatedAt:      app.CreatedAt,
		UpdatedAt:      now,
		RunID:          app.RunID,
		NodeID:         app.NodeID,
		OwnerUserID:    app.OwnerUserID,
		ToolVersionID:  app.ToolVersionID,
		ArgumentsHash:  app.ArgumentsHash,
		ArgumentsJSON:  argsJSON,
		SourceRevision: app.SourceRevision,
		Status:         d.ApprovalStatusRejected,
		ExpiresAt:      app.ExpiresAt,
		DecidedAt:      &now,
		DecisionReason: &in.Reason,
	}, nil
}
