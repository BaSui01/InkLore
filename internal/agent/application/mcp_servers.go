package application

import (
	d "InkLore/internal/agent/domain"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/scope"
	"context"
	"time"

	"github.com/google/uuid"
)

// CreateMCPServer validates endpoint and env_ref allowlists and registers an MCP server.
func (s *GovernanceService) CreateMCPServer(ctx context.Context, p identity.Principal, input MCPServerInput) (*d.MCPServerView, error) {
	if err := s.AuthorizeAdmin(p, true); err != nil {
		return nil, err
	}
	if err := d.ValidateMCPServer(input.Title, input.Endpoint, input.EnvRef, input.AllowedMethods); err != nil {
		return nil, err
	}
	if err := s.allowlist.ValidateEndpoint(input.Endpoint); err != nil {
		return nil, d.ErrInvalidConfiguration(map[string]any{"reason": "endpoint is not in deployment HTTPS allowlist", "error": err.Error()})
	}
	if err := s.allowlist.ValidateEnvRef(input.EnvRef); err != nil {
		return nil, d.ErrInvalidConfiguration(map[string]any{"reason": "env_ref is not in deployment allowlist", "error": err.Error()})
	}

	serverID := uuid.NewString()
	now := time.Now().UTC()

	srv := &d.MCPServer{
		ID:             serverID,
		TenantID:       p.TenantID,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
		Title:          input.Title,
		Endpoint:       input.Endpoint,
		EnvRef:         input.EnvRef,
		AllowedMethods: input.AllowedMethods,
		Enabled:        input.Enabled,
	}

	err := s.manager.Within(ctx, func(txctx context.Context) error {
		return s.repo.CreateMCPServer(txctx, p.TenantID, srv)
	})
	if err != nil {
		return nil, err
	}

	return &d.MCPServerView{
		ID:             srv.ID,
		Revision:       srv.Revision,
		CreatedAt:      srv.CreatedAt,
		UpdatedAt:      srv.UpdatedAt,
		Title:          srv.Title,
		Endpoint:       srv.Endpoint,
		EnvRef:         srv.EnvRef,
		AllowedMethods: srv.AllowedMethods,
		Enabled:        srv.Enabled,
	}, nil
}

// GetMCPServer returns the registered MCP server.
func (s *GovernanceService) GetMCPServer(ctx context.Context, p identity.Principal, id string) (*d.MCPServerView, error) {
	if err := s.AuthorizeAdmin(p, false); err != nil {
		return nil, err
	}
	if !scope.ValidID(id) {
		return nil, d.ErrValidationFailed(map[string]any{"reason": "invalid mcp_server_id"})
	}

	srv, err := s.repo.GetMCPServer(ctx, p.TenantID, id)
	if err != nil {
		return nil, err
	}

	return &d.MCPServerView{
		ID:             srv.ID,
		Revision:       srv.Revision,
		CreatedAt:      srv.CreatedAt,
		UpdatedAt:      srv.UpdatedAt,
		Title:          srv.Title,
		Endpoint:       srv.Endpoint,
		EnvRef:         srv.EnvRef,
		AllowedMethods: srv.AllowedMethods,
		Enabled:        srv.Enabled,
	}, nil
}

// ListMCPServers returns paginated MCP server views.
func (s *GovernanceService) ListMCPServers(ctx context.Context, p identity.Principal, query d.ListQuery) (*MCPServerPage, error) {
	if err := s.AuthorizeAdmin(p, false); err != nil {
		return nil, err
	}
	items, nextCursor, hasMore, err := s.repo.ListMCPServers(ctx, p.TenantID, query)
	if err != nil {
		return nil, err
	}

	views := make([]d.MCPServerView, 0, len(items))
	for _, srv := range items {
		views = append(views, d.MCPServerView{
			ID:             srv.ID,
			Revision:       srv.Revision,
			CreatedAt:      srv.CreatedAt,
			UpdatedAt:      srv.UpdatedAt,
			Title:          srv.Title,
			Endpoint:       srv.Endpoint,
			EnvRef:         srv.EnvRef,
			AllowedMethods: srv.AllowedMethods,
			Enabled:        srv.Enabled,
		})
	}

	return &MCPServerPage{
		Items:      views,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

// UpdateMCPServer validates allowlists and updates an MCP server CAS.
func (s *GovernanceService) UpdateMCPServer(ctx context.Context, p identity.Principal, id string, input MCPServerUpdate) (*d.MCPServerView, error) {
	if err := s.AuthorizeAdmin(p, true); err != nil {
		return nil, err
	}
	if !scope.ValidID(id) {
		return nil, d.ErrValidationFailed(map[string]any{"reason": "invalid mcp_server_id"})
	}
	if input.ExpectedRevision < 1 {
		return nil, d.ErrValidationFailed(map[string]any{"reason": "expected_revision must be >= 1"})
	}
	if err := d.ValidateMCPServer(input.Title, input.Endpoint, input.EnvRef, input.AllowedMethods); err != nil {
		return nil, err
	}
	if err := s.allowlist.ValidateEndpoint(input.Endpoint); err != nil {
		return nil, d.ErrInvalidConfiguration(map[string]any{"reason": "endpoint is not in deployment HTTPS allowlist", "error": err.Error()})
	}
	if err := s.allowlist.ValidateEnvRef(input.EnvRef); err != nil {
		return nil, d.ErrInvalidConfiguration(map[string]any{"reason": "env_ref is not in deployment allowlist", "error": err.Error()})
	}

	var updatedView *d.MCPServerView

	err := s.manager.Within(ctx, func(txctx context.Context) error {
		currentSrv, err := s.repo.GetMCPServer(txctx, p.TenantID, id)
		if err != nil {
			return err
		}
		if currentSrv.Revision != input.ExpectedRevision {
			return d.ErrVersionConflict(map[string]any{"expected": input.ExpectedRevision, "actual": currentSrv.Revision})
		}

		now := time.Now().UTC()
		currentSrv.Title = input.Title
		currentSrv.Endpoint = input.Endpoint
		currentSrv.EnvRef = input.EnvRef
		currentSrv.AllowedMethods = input.AllowedMethods
		currentSrv.Enabled = input.Enabled
		currentSrv.UpdatedAt = now

		if err := s.repo.UpdateMCPServer(txctx, p.TenantID, currentSrv, input.ExpectedRevision); err != nil {
			return err
		}

		updatedView = &d.MCPServerView{
			ID:             currentSrv.ID,
			Revision:       currentSrv.Revision + 1,
			CreatedAt:      currentSrv.CreatedAt,
			UpdatedAt:      now,
			Title:          currentSrv.Title,
			Endpoint:       currentSrv.Endpoint,
			EnvRef:         currentSrv.EnvRef,
			AllowedMethods: currentSrv.AllowedMethods,
			Enabled:        currentSrv.Enabled,
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return updatedView, nil
}

// DeleteMCPServer soft-deletes an MCP server after verifying it is not in use by tools.
func (s *GovernanceService) DeleteMCPServer(ctx context.Context, p identity.Principal, id string, expectedRevision int64) (*d.Deleted, error) {
	if err := s.AuthorizeAdmin(p, true); err != nil {
		return nil, err
	}
	if !scope.ValidID(id) {
		return nil, d.ErrValidationFailed(map[string]any{"reason": "invalid mcp_server_id"})
	}
	if expectedRevision < 1 {
		return nil, d.ErrValidationFailed(map[string]any{"reason": "expected_revision must be >= 1"})
	}

	inUse, err := s.repo.IsMCPServerInUse(ctx, p.TenantID, id)
	if err != nil {
		return nil, err
	}
	if inUse {
		return nil, d.ErrResourceInUse(map[string]any{"reason": "mcp server is currently referenced by tools"})
	}

	err = s.manager.Within(ctx, func(txctx context.Context) error {
		return s.repo.DeleteMCPServer(txctx, p.TenantID, id, expectedRevision)
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
