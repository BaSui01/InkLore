package application

import (
	"context"
	"errors"

	d "InkLore/internal/agent/domain"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/scope"
)

// ArtifactService provides read access to generated suggestion artifacts.
type ArtifactService struct {
	repo       d.RunRepository
	authorizer RunAuthorizer
}

// NewArtifactService constructs an ArtifactService.
func NewArtifactService(repo d.RunRepository, authorizer RunAuthorizer) (*ArtifactService, error) {
	if repo == nil || authorizer == nil {
		return nil, errors.New("agent artifact service: repository required")
	}
	return &ArtifactService{repo: repo, authorizer: authorizer}, nil
}

// AuthorizeCreator verifies that the principal has access to read artifacts.
func (s *ArtifactService) AuthorizeCreator(p identity.Principal) error {
	if p.Audience != "creator" && p.Audience != "admin" {
		return d.ErrForbidden()
	}
	return nil
}

// GetArtifact retrieves a single suggestion artifact by ID.
func (s *ArtifactService) GetArtifact(ctx context.Context, p identity.Principal, artifactID string) (*d.ArtifactView, error) {
	if err := s.AuthorizeCreator(p); err != nil {
		return nil, err
	}
	if !scope.ValidID(artifactID) {
		return nil, d.ErrValidationFailed(map[string]any{"reason": "invalid artifact_id"})
	}

	art, err := s.repo.GetArtifact(ctx, p.TenantID, artifactID)
	if err != nil {
		return nil, err
	}

	if err := s.authorizer.AuthorizeRun(ctx, p, art.RunID); err != nil {
		return nil, err
	}
	// Verify run ownership
	run, err := s.repo.GetRunOnly(ctx, p.TenantID, art.RunID)
	if err != nil {
		return nil, err
	}
	if run.OwnerUserID != p.UserID && p.Audience != "admin" {
		return nil, d.ErrNotFound()
	}

	return &d.ArtifactView{
		ID:               art.ID,
		CreatedAt:        art.CreatedAt,
		RunID:            art.RunID,
		NodeID:           art.NodeID,
		Kind:             art.Kind,
		TargetResourceID: art.TargetResourceID,
		SourceRevision:   art.SourceRevision,
		ContentHash:      art.ContentHash,
		TextContent:      art.TextContent,
		FileID:           art.FileID,
		IsPartial:        art.IsPartial,
	}, nil
}

// ListArtifacts retrieves suggestion artifacts for a given run with keyset pagination.
func (s *ArtifactService) ListArtifacts(ctx context.Context, p identity.Principal, runID string, query d.ListQuery) ([]d.ArtifactView, *string, bool, error) {
	if err := s.AuthorizeCreator(p); err != nil {
		return nil, nil, false, err
	}
	if !scope.ValidID(runID) {
		return nil, nil, false, d.ErrValidationFailed(map[string]any{"reason": "invalid run_id"})
	}

	if err := s.authorizer.AuthorizeRun(ctx, p, runID); err != nil {
		return nil, nil, false, err
	}
	// Verify run ownership
	run, err := s.repo.GetRunOnly(ctx, p.TenantID, runID)
	if err != nil {
		return nil, nil, false, err
	}
	if run.OwnerUserID != p.UserID && p.Audience != "admin" {
		return nil, nil, false, d.ErrNotFound()
	}

	return s.repo.ListArtifacts(ctx, p.TenantID, runID, query)
}
