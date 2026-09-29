package application

import (
	d "InkLore/internal/agent/domain"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/scope"
	"context"
	"time"

	"github.com/google/uuid"
)

// CreateDefinition creates a new definition and its initial version (v1) within a UoW.
func (s *GovernanceService) CreateDefinition(ctx context.Context, p identity.Principal, input DefinitionInput) (*d.DefinitionView, error) {
	if err := s.AuthorizeAdmin(p, true); err != nil {
		return nil, err
	}
	if err := d.ValidateDefinition(input.DefinitionKey, input.Title, input.SystemInstruction, input.TaskKinds, input.ToolVersionIDs); err != nil {
		return nil, err
	}

	if len(input.ToolVersionIDs) > 0 {
		exists, err := s.repo.CheckToolVersionsExist(ctx, p.TenantID, input.ToolVersionIDs)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, d.ErrInvalidConfiguration(map[string]any{"reason": "one or more tool_version_ids not found in tenant"})
		}
	}

	defID := uuid.NewString()
	verID := uuid.NewString()
	now := time.Now().UTC()

	def := &d.AgentDefinition{
		ID:               defID,
		TenantID:         p.TenantID,
		CreatedAt:        now,
		UpdatedAt:        now,
		Revision:         1,
		DefinitionKey:    input.DefinitionKey,
		Title:            input.Title,
		Enabled:          input.Enabled,
		CurrentVersionID: &verID,
	}

	ver := &d.DefinitionVersion{
		ID:                verID,
		TenantID:          p.TenantID,
		CreatedAt:         now,
		UpdatedAt:         now,
		DefinitionID:      defID,
		Version:           1,
		SystemInstruction: input.SystemInstruction,
		ToolVersionIDs:    input.ToolVersionIDs,
		TaskKinds:         input.TaskKinds,
	}

	err := s.manager.Within(ctx, func(txctx context.Context) error {
		return s.repo.CreateDefinition(txctx, p.TenantID, def, ver)
	})
	if err != nil {
		return nil, err
	}

	pubStatus := d.PublicationStatusDraft
	if def.Enabled {
		pubStatus = d.PublicationStatusPublished
	}

	return &d.DefinitionView{
		ID:                def.ID,
		Revision:          def.Revision,
		CreatedAt:         def.CreatedAt,
		UpdatedAt:         def.UpdatedAt,
		DefinitionKey:     def.DefinitionKey,
		Title:             def.Title,
		Enabled:           def.Enabled,
		SystemInstruction: ver.SystemInstruction,
		ToolVersionIDs:    ver.ToolVersionIDs,
		TaskKinds:         ver.TaskKinds,
		CurrentVersionID:  ver.ID,
		Version:           ver.Version,
		PublicationStatus: pubStatus,
	}, nil
}

// GetDefinition returns the definition and its current version.
func (s *GovernanceService) GetDefinition(ctx context.Context, p identity.Principal, id string) (*d.DefinitionView, error) {
	if err := s.AuthorizeAdmin(p, false); err != nil {
		return nil, err
	}
	if !scope.ValidID(id) {
		return nil, d.ErrValidationFailed(map[string]any{"reason": "invalid definition_id"})
	}

	def, ver, err := s.repo.GetDefinition(ctx, p.TenantID, id)
	if err != nil {
		return nil, err
	}

	pubStatus := d.PublicationStatusDraft
	if def.Enabled {
		pubStatus = d.PublicationStatusPublished
	}

	return &d.DefinitionView{
		ID:                def.ID,
		Revision:          def.Revision,
		CreatedAt:         def.CreatedAt,
		UpdatedAt:         def.UpdatedAt,
		DefinitionKey:     def.DefinitionKey,
		Title:             def.Title,
		Enabled:           def.Enabled,
		SystemInstruction: ver.SystemInstruction,
		ToolVersionIDs:    ver.ToolVersionIDs,
		TaskKinds:         ver.TaskKinds,
		CurrentVersionID:  ver.ID,
		Version:           ver.Version,
		PublicationStatus: pubStatus,
	}, nil
}

// ListDefinitions returns paginated definition views.
func (s *GovernanceService) ListDefinitions(ctx context.Context, p identity.Principal, query d.ListQuery) (*DefinitionPage, error) {
	if err := s.AuthorizeAdmin(p, false); err != nil {
		return nil, err
	}
	items, nextCursor, hasMore, err := s.repo.ListDefinitions(ctx, p.TenantID, query)
	if err != nil {
		return nil, err
	}
	return &DefinitionPage{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

// UpdateDefinition appends an immutable new version and updates the mutable head CAS.
func (s *GovernanceService) UpdateDefinition(ctx context.Context, p identity.Principal, id string, input DefinitionUpdate) (*d.DefinitionView, error) {
	if err := s.AuthorizeAdmin(p, true); err != nil {
		return nil, err
	}
	if !scope.ValidID(id) {
		return nil, d.ErrValidationFailed(map[string]any{"reason": "invalid definition_id"})
	}
	if input.ExpectedRevision < 1 {
		return nil, d.ErrValidationFailed(map[string]any{"reason": "expected_revision must be >= 1"})
	}
	if err := d.ValidateDefinition(input.DefinitionKey, input.Title, input.SystemInstruction, input.TaskKinds, input.ToolVersionIDs); err != nil {
		return nil, err
	}

	if len(input.ToolVersionIDs) > 0 {
		exists, err := s.repo.CheckToolVersionsExist(ctx, p.TenantID, input.ToolVersionIDs)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, d.ErrInvalidConfiguration(map[string]any{"reason": "one or more tool_version_ids not found in tenant"})
		}
	}

	var updatedView *d.DefinitionView

	err := s.manager.Within(ctx, func(txctx context.Context) error {
		currentDef, currentVer, err := s.repo.GetDefinition(txctx, p.TenantID, id)
		if err != nil {
			return err
		}
		if currentDef.Revision != input.ExpectedRevision {
			return d.ErrVersionConflict(map[string]any{"expected": input.ExpectedRevision, "actual": currentDef.Revision})
		}
		if currentDef.DefinitionKey != input.DefinitionKey {
			return d.ErrInvalidConfiguration(map[string]any{"reason": "definition_key cannot be changed"})
		}

		now := time.Now().UTC()
		newVerID := uuid.NewString()
		newVer := &d.DefinitionVersion{
			ID:                newVerID,
			TenantID:          p.TenantID,
			CreatedAt:         now,
			UpdatedAt:         now,
			DefinitionID:      id,
			Version:           currentVer.Version + 1,
			SystemInstruction: input.SystemInstruction,
			ToolVersionIDs:    input.ToolVersionIDs,
			TaskKinds:         input.TaskKinds,
		}

		currentDef.Title = input.Title
		currentDef.Enabled = input.Enabled
		currentDef.CurrentVersionID = &newVerID
		currentDef.UpdatedAt = now

		if err := s.repo.UpdateDefinition(txctx, p.TenantID, currentDef, newVer, input.ExpectedRevision); err != nil {
			return err
		}

		pubStatus := d.PublicationStatusDraft
		if currentDef.Enabled {
			pubStatus = d.PublicationStatusPublished
		}

		updatedView = &d.DefinitionView{
			ID:                currentDef.ID,
			Revision:          currentDef.Revision + 1,
			CreatedAt:         currentDef.CreatedAt,
			UpdatedAt:         now,
			DefinitionKey:     currentDef.DefinitionKey,
			Title:             currentDef.Title,
			Enabled:           currentDef.Enabled,
			SystemInstruction: newVer.SystemInstruction,
			ToolVersionIDs:    newVer.ToolVersionIDs,
			TaskKinds:         newVer.TaskKinds,
			CurrentVersionID:  newVer.ID,
			Version:           newVer.Version,
			PublicationStatus: pubStatus,
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return updatedView, nil
}

// DeleteDefinition hides a definition from governance and creator catalogs without
// removing immutable versions referenced by existing runs or novel guides.
func (s *GovernanceService) DeleteDefinition(ctx context.Context, p identity.Principal, id string, expectedRevision int64) (*d.Deleted, error) {
	if err := s.AuthorizeAdmin(p, true); err != nil {
		return nil, err
	}
	if !scope.ValidID(id) {
		return nil, d.ErrValidationFailed(map[string]any{"reason": "invalid definition_id"})
	}
	if expectedRevision < 1 {
		return nil, d.ErrValidationFailed(map[string]any{"reason": "expected_revision must be >= 1"})
	}

	err := s.manager.Within(ctx, func(txctx context.Context) error {
		return s.repo.DeleteDefinition(txctx, p.TenantID, id, expectedRevision)
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

// ListCatalogDefinitions returns creator-accessible definitions.
func (s *GovernanceService) ListCatalogDefinitions(ctx context.Context, p identity.Principal, query d.ListQuery) (*CatalogDefinitionPage, error) {
	if err := s.AuthorizeCreator(p); err != nil {
		return nil, err
	}
	items, nextCursor, hasMore, err := s.repo.ListCatalogDefinitions(ctx, p.TenantID, query)
	if err != nil {
		return nil, err
	}
	return &CatalogDefinitionPage{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}
