package application

import (
	d "InkLore/internal/agent/domain"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/scope"
	"context"
	"regexp"
	"time"

	"github.com/google/uuid"
)

var workflowKeyPattern = regexp.MustCompile("^[a-z][a-z0-9_]*$")

// CreateWorkflow validates the DAG, checks version references, and persists head + version 1.
func (s *GovernanceService) CreateWorkflow(ctx context.Context, p identity.Principal, input WorkflowInput) (*d.WorkflowView, error) {
	if err := s.AuthorizeAdmin(p, true); err != nil {
		return nil, err
	}
	if len(input.WorkflowKey) < 1 || len(input.WorkflowKey) > 64 || !workflowKeyPattern.MatchString(input.WorkflowKey) {
		return nil, d.ErrInvalidConfiguration(map[string]any{"reason": "invalid workflow_key"})
	}
	if len(input.Title) < 1 || len(input.Title) > 120 {
		return nil, d.ErrInvalidConfiguration(map[string]any{"reason": "invalid title length"})
	}
	if err := d.ValidateDAG(input.Manifest); err != nil {
		return nil, err
	}

	manifestHash, err := d.ComputeManifestHash(input.Manifest)
	if err != nil {
		return nil, d.ErrInvalidConfiguration(map[string]any{"reason": "failed to compute manifest hash", "error": err.Error()})
	}

	// Verify tool version references in nodes
	var toolVerIDs []string
	for _, node := range input.Manifest.Nodes {
		if node.ToolVersionID != nil {
			toolVerIDs = append(toolVerIDs, *node.ToolVersionID)
		}
	}
	if len(toolVerIDs) > 0 {
		exists, err := s.repo.CheckToolVersionsExist(ctx, p.TenantID, toolVerIDs)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, d.ErrInvalidConfiguration(map[string]any{"reason": "one or more tool_version_ids in workflow not found in tenant"})
		}
	}

	wfID := uuid.NewString()
	verID := uuid.NewString()
	now := time.Now().UTC()

	wf := &d.AgentWorkflow{
		ID:               wfID,
		TenantID:         p.TenantID,
		CreatedAt:        now,
		UpdatedAt:        now,
		Revision:         1,
		WorkflowKey:      input.WorkflowKey,
		Title:            input.Title,
		Enabled:          input.Enabled,
		CurrentVersionID: &verID,
	}

	ver := &d.WorkflowVersion{
		ID:           verID,
		TenantID:     p.TenantID,
		CreatedAt:    now,
		UpdatedAt:    now,
		WorkflowID:   wfID,
		Version:      1,
		Manifest:     input.Manifest,
		ManifestHash: manifestHash,
	}

	err = s.manager.Within(ctx, func(txctx context.Context) error {
		return s.repo.CreateWorkflow(txctx, p.TenantID, wf, ver)
	})
	if err != nil {
		return nil, err
	}

	return &d.WorkflowView{
		ID:               wf.ID,
		Revision:         wf.Revision,
		CreatedAt:        wf.CreatedAt,
		UpdatedAt:        wf.UpdatedAt,
		WorkflowKey:      wf.WorkflowKey,
		Title:            wf.Title,
		Enabled:          wf.Enabled,
		Manifest:         ver.Manifest,
		CurrentVersionID: ver.ID,
		Version:          ver.Version,
		ManifestHash:     ver.ManifestHash,
	}, nil
}

// GetWorkflow returns the workflow and its current version.
func (s *GovernanceService) GetWorkflow(ctx context.Context, p identity.Principal, id string) (*d.WorkflowView, error) {
	if err := s.AuthorizeAdmin(p, false); err != nil {
		return nil, err
	}
	if !scope.ValidID(id) {
		return nil, d.ErrValidationFailed(map[string]any{"reason": "invalid workflow_id"})
	}

	wf, ver, err := s.repo.GetWorkflow(ctx, p.TenantID, id)
	if err != nil {
		return nil, err
	}

	return &d.WorkflowView{
		ID:               wf.ID,
		Revision:         wf.Revision,
		CreatedAt:        wf.CreatedAt,
		UpdatedAt:        wf.UpdatedAt,
		WorkflowKey:      wf.WorkflowKey,
		Title:            wf.Title,
		Enabled:          wf.Enabled,
		Manifest:         ver.Manifest,
		CurrentVersionID: ver.ID,
		Version:          ver.Version,
		ManifestHash:     ver.ManifestHash,
	}, nil
}

// ListWorkflows returns paginated workflow views.
func (s *GovernanceService) ListWorkflows(ctx context.Context, p identity.Principal, query d.ListQuery) (*WorkflowPage, error) {
	if err := s.AuthorizeAdmin(p, false); err != nil {
		return nil, err
	}
	items, nextCursor, hasMore, err := s.repo.ListWorkflows(ctx, p.TenantID, query)
	if err != nil {
		return nil, err
	}
	return &WorkflowPage{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

// UpdateWorkflow appends an immutable new version and updates the mutable head CAS.
func (s *GovernanceService) UpdateWorkflow(ctx context.Context, p identity.Principal, id string, input WorkflowUpdate) (*d.WorkflowView, error) {
	if err := s.AuthorizeAdmin(p, true); err != nil {
		return nil, err
	}
	if !scope.ValidID(id) {
		return nil, d.ErrValidationFailed(map[string]any{"reason": "invalid workflow_id"})
	}
	if input.ExpectedRevision < 1 {
		return nil, d.ErrValidationFailed(map[string]any{"reason": "expected_revision must be >= 1"})
	}
	if len(input.WorkflowKey) < 1 || len(input.WorkflowKey) > 64 || !workflowKeyPattern.MatchString(input.WorkflowKey) {
		return nil, d.ErrInvalidConfiguration(map[string]any{"reason": "invalid workflow_key"})
	}
	if len(input.Title) < 1 || len(input.Title) > 120 {
		return nil, d.ErrInvalidConfiguration(map[string]any{"reason": "invalid title length"})
	}
	if err := d.ValidateDAG(input.Manifest); err != nil {
		return nil, err
	}

	manifestHash, err := d.ComputeManifestHash(input.Manifest)
	if err != nil {
		return nil, d.ErrInvalidConfiguration(map[string]any{"reason": "failed to compute manifest hash", "error": err.Error()})
	}

	var toolVerIDs []string
	for _, node := range input.Manifest.Nodes {
		if node.ToolVersionID != nil {
			toolVerIDs = append(toolVerIDs, *node.ToolVersionID)
		}
	}
	if len(toolVerIDs) > 0 {
		exists, err := s.repo.CheckToolVersionsExist(ctx, p.TenantID, toolVerIDs)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, d.ErrInvalidConfiguration(map[string]any{"reason": "one or more tool_version_ids in workflow not found in tenant"})
		}
	}

	var updatedView *d.WorkflowView

	err = s.manager.Within(ctx, func(txctx context.Context) error {
		currentWF, currentVer, err := s.repo.GetWorkflow(txctx, p.TenantID, id)
		if err != nil {
			return err
		}
		if currentWF.Revision != input.ExpectedRevision {
			return d.ErrVersionConflict(map[string]any{"expected": input.ExpectedRevision, "actual": currentWF.Revision})
		}
		if currentWF.WorkflowKey != input.WorkflowKey {
			return d.ErrInvalidConfiguration(map[string]any{"reason": "workflow_key cannot be changed"})
		}

		now := time.Now().UTC()
		newVerID := uuid.NewString()
		newVer := &d.WorkflowVersion{
			ID:           newVerID,
			TenantID:     p.TenantID,
			CreatedAt:    now,
			UpdatedAt:    now,
			WorkflowID:   id,
			Version:      currentVer.Version + 1,
			Manifest:     input.Manifest,
			ManifestHash: manifestHash,
		}

		currentWF.Title = input.Title
		currentWF.Enabled = input.Enabled
		currentWF.CurrentVersionID = &newVerID
		currentWF.UpdatedAt = now

		if err := s.repo.UpdateWorkflow(txctx, p.TenantID, currentWF, newVer, input.ExpectedRevision); err != nil {
			return err
		}

		updatedView = &d.WorkflowView{
			ID:               currentWF.ID,
			Revision:         currentWF.Revision + 1,
			CreatedAt:        currentWF.CreatedAt,
			UpdatedAt:        now,
			WorkflowKey:      currentWF.WorkflowKey,
			Title:            currentWF.Title,
			Enabled:          currentWF.Enabled,
			Manifest:         newVer.Manifest,
			CurrentVersionID: newVer.ID,
			Version:          newVer.Version,
			ManifestHash:     newVer.ManifestHash,
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return updatedView, nil
}

// DeleteWorkflow hides a workflow while retaining versions used by existing runs.
func (s *GovernanceService) DeleteWorkflow(ctx context.Context, p identity.Principal, id string, expectedRevision int64) (*d.Deleted, error) {
	if err := s.AuthorizeAdmin(p, true); err != nil {
		return nil, err
	}
	if !scope.ValidID(id) {
		return nil, d.ErrValidationFailed(map[string]any{"reason": "invalid workflow_id"})
	}
	if expectedRevision < 1 {
		return nil, d.ErrValidationFailed(map[string]any{"reason": "expected_revision must be >= 1"})
	}

	err := s.manager.Within(ctx, func(txctx context.Context) error {
		return s.repo.DeleteWorkflow(txctx, p.TenantID, id, expectedRevision)
	})
	if err != nil {
		return nil, err
	}

	return &d.Deleted{
		ID:       id,
		Revision: expectedRevision + 1,
		Deleted:  true,
	}, nil
}

// ListCatalogWorkflows returns creator-accessible workflows.
func (s *GovernanceService) ListCatalogWorkflows(ctx context.Context, p identity.Principal, query d.ListQuery) (*CatalogWorkflowPage, error) {
	if err := s.AuthorizeCreator(p); err != nil {
		return nil, err
	}
	items, nextCursor, hasMore, err := s.repo.ListCatalogWorkflows(ctx, p.TenantID, query)
	if err != nil {
		return nil, err
	}
	return &CatalogWorkflowPage{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}
