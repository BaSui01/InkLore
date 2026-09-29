package application

import (
	"context"
	"errors"
	"sync"

	d "InkLore/internal/agent/domain"
	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/scope"
)

var toolExecutionStatusBindings sync.Map // *GovernanceService -> func(context.Context, string, d.ToolView) string

// SetToolExecutionStatus binds a live worker's capability projection to Admin
// responses. An unbound service always reports not_bound. Attach only after the
// Scheduler and its tool executor are successfully assembled in the app root.
func (s *GovernanceService) SetToolExecutionStatus(status func(context.Context, string, d.ToolView) string) {
	if s == nil {
		return
	}
	if status == nil {
		toolExecutionStatusBindings.Delete(s)
		return
	}
	toolExecutionStatusBindings.Store(s, status)
}

func (s *GovernanceService) executionStatus(ctx context.Context, tenantID string, view d.ToolView) string {
	if s != nil {
		if bound, ok := toolExecutionStatusBindings.Load(s); ok {
			if status := bound.(func(context.Context, string, d.ToolView) string)(ctx, tenantID, view); status == d.ToolExecutionAvailable {
				return status
			}
		}
	}
	return d.ToolExecutionNotBound
}

// ListBuiltinTools projects only registered Go handlers. Availability additionally
// requires a compatible enabled current version for this Admin tenant; a missing
// registration is not_bound, while storage failures fail the entire response.
func (s *GovernanceService) ListBuiltinTools(ctx context.Context, p identity.Principal) ([]d.BuiltinTool, error) {
	if err := s.AuthorizeAdmin(p, false); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.builtinTools == nil {
		return []d.BuiltinTool{}, nil
	}
	tools := s.builtinTools.BuiltinTools()
	for i := range tools {
		tools[i].ExecutionStatus = d.ToolExecutionNotBound
		tool, version, err := s.repo.GetToolByKey(ctx, p.TenantID, tools[i].ToolKey)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}
			var contractErr *pe.AppError
			if errors.As(err, &contractErr) && contractErr.HTTPStatus == 404 && contractErr.Code == "platform.resource_not_found" {
				continue
			}
			return nil, err
		}
		if tool == nil || version == nil || tool.DeletedAt != nil || tool.CurrentVersionID == nil ||
			tool.TenantID != p.TenantID || version.TenantID != p.TenantID ||
			tool.ToolKey != tools[i].ToolKey || version.ToolID != tool.ID || version.ID != *tool.CurrentVersionID {
			continue
		}
		tools[i].ExecutionStatus = s.executionStatus(ctx, p.TenantID, d.ToolView{
			ToolKey: tool.ToolKey, Enabled: tool.Enabled, CurrentVersionID: version.ID,
			InputSchema: version.InputSchema, OutputSchema: version.OutputSchema, SideEffect: version.SideEffect,
			ApprovalRequired: version.ApprovalRequired, TimeoutSeconds: version.TimeoutSeconds,
			MCPServerID: version.MCPServerID, RemoteToolName: version.RemoteToolName,
		})
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return tools, nil
}

// GetTool returns the tool and its current version.
func (s *GovernanceService) GetTool(ctx context.Context, p identity.Principal, id string) (*d.ToolView, error) {
	if err := s.AuthorizeAdmin(p, false); err != nil {
		return nil, err
	}
	if !scope.ValidID(id) {
		return nil, d.ErrValidationFailed(map[string]any{"reason": "invalid tool_id"})
	}

	tool, ver, err := s.repo.GetTool(ctx, p.TenantID, id)
	if err != nil {
		return nil, err
	}

	return &d.ToolView{
		ID:               tool.ID,
		Revision:         tool.Revision,
		CreatedAt:        tool.CreatedAt,
		UpdatedAt:        tool.UpdatedAt,
		ToolKey:          tool.ToolKey,
		Title:            tool.Title,
		Enabled:          tool.Enabled,
		ExecutionStatus:  s.executionStatus(ctx, p.TenantID, d.ToolView{ToolKey: tool.ToolKey, Enabled: tool.Enabled, CurrentVersionID: ver.ID, InputSchema: ver.InputSchema, OutputSchema: ver.OutputSchema, SideEffect: ver.SideEffect, ApprovalRequired: ver.ApprovalRequired, TimeoutSeconds: ver.TimeoutSeconds, MCPServerID: ver.MCPServerID, RemoteToolName: ver.RemoteToolName}),
		InputSchema:      ver.InputSchema,
		OutputSchema:     ver.OutputSchema,
		SideEffect:       ver.SideEffect,
		ApprovalRequired: ver.ApprovalRequired,
		TimeoutSeconds:   ver.TimeoutSeconds,
		MCPServerID:      ver.MCPServerID,
		RemoteToolName:   ver.RemoteToolName,
		CurrentVersionID: ver.ID,
		Version:          ver.Version,
	}, nil
}

// ListTools returns paginated tool views.
func (s *GovernanceService) ListTools(ctx context.Context, p identity.Principal, query d.ListQuery) (*ToolPage, error) {
	if err := s.AuthorizeAdmin(p, false); err != nil {
		return nil, err
	}
	items, nextCursor, hasMore, err := s.repo.ListTools(ctx, p.TenantID, query)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].ExecutionStatus = s.executionStatus(ctx, p.TenantID, items[i])
	}
	return &ToolPage{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}
