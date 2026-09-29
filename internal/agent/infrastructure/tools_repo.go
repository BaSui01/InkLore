package infrastructure

import (
	d "InkLore/internal/agent/domain"
	"context"
	"encoding/json"
	"strings"
	"time"
)

type toolRow struct {
	CreatedAt        time.Time  `gorm:"column:created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at"`
	CurrentVersionID *string    `gorm:"column:current_version_id"`
	DeletedAt        *time.Time `gorm:"column:deleted_at"`
	MCPServerID      *string    `gorm:"column:mcp_server_id"`
	RemoteToolName   *string    `gorm:"column:remote_tool_name"`
	ID               string     `gorm:"column:id"`
	TenantID         string     `gorm:"column:tenant_id"`
	ToolKey          string     `gorm:"column:tool_key"`
	Title            string     `gorm:"column:title"`
	SideEffect       string     `gorm:"column:side_effect"`
	InputSchema      JSONB      `gorm:"column:input_schema"`
	OutputSchema     JSONB      `gorm:"column:output_schema"`
	Revision         int64      `gorm:"column:revision"`
	Version          int        `gorm:"column:version"`
	TimeoutSeconds   int        `gorm:"column:timeout_seconds"`
	Enabled          bool       `gorm:"column:enabled"`
	ApprovalRequired bool       `gorm:"column:approval_required"`
}

func (r *Repository) GetTool(ctx context.Context, tenantID, id string) (*d.AgentTool, *d.ToolVersion, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, nil, err
	}

	var row toolRow
	query := `
		SELECT t.id, t.tenant_id, t.created_at, t.updated_at, t.revision, t.tool_key, t.title, t.enabled, t.current_version_id, t.deleted_at,
		       v.version, v.input_schema, v.output_schema, v.side_effect, v.approval_required, v.timeout_seconds, v.mcp_server_id, v.remote_tool_name
		FROM agent_tools t
		JOIN agent_tool_versions v ON v.tenant_id = t.tenant_id AND v.id = t.current_version_id
		WHERE t.tenant_id = ? AND t.id = ? AND t.deleted_at IS NULL
	`
	if err := db.Raw(query, tenantID, id).Take(&row).Error; err != nil {
		return nil, nil, handleNotFound(err)
	}

	tool := &d.AgentTool{
		ID:               row.ID,
		TenantID:         row.TenantID,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
		Revision:         row.Revision,
		ToolKey:          row.ToolKey,
		Title:            row.Title,
		Enabled:          row.Enabled,
		CurrentVersionID: row.CurrentVersionID,
		DeletedAt:        row.DeletedAt,
	}

	curVerID := ""
	if row.CurrentVersionID != nil {
		curVerID = *row.CurrentVersionID
	}
	ver := &d.ToolVersion{
		ID:               curVerID,
		TenantID:         row.TenantID,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
		ToolID:           row.ID,
		Version:          row.Version,
		InputSchema:      json.RawMessage(row.InputSchema),
		OutputSchema:     json.RawMessage(row.OutputSchema),
		SideEffect:       row.SideEffect,
		ApprovalRequired: row.ApprovalRequired,
		TimeoutSeconds:   row.TimeoutSeconds,
		MCPServerID:      row.MCPServerID,
		RemoteToolName:   row.RemoteToolName,
	}

	return tool, ver, nil
}

func (r *Repository) GetToolVersion(ctx context.Context, tenantID, versionID string) (*d.AgentTool, *d.ToolVersion, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, nil, err
	}
	var row struct {
		ToolID           string     `gorm:"column:tool_id"`
		ToolKey          string     `gorm:"column:tool_key"`
		Title            string     `gorm:"column:title"`
		CurrentVersionID *string    `gorm:"column:current_version_id"`
		MCPServerID      *string    `gorm:"column:mcp_server_id"`
		RemoteToolName   *string    `gorm:"column:remote_tool_name"`
		InputSchema      JSONB      `gorm:"column:input_schema"`
		OutputSchema     JSONB      `gorm:"column:output_schema"`
		SideEffect       string     `gorm:"column:side_effect"`
		Version          int        `gorm:"column:version"`
		TimeoutSeconds   int        `gorm:"column:timeout_seconds"`
		ApprovalRequired bool       `gorm:"column:approval_required"`
		Enabled          bool       `gorm:"column:enabled"`
		DeletedAt        *time.Time `gorm:"column:deleted_at"`
	}
	if err := db.Raw(`SELECT v.tool_id, t.tool_key, t.title, t.current_version_id, t.enabled, t.deleted_at,
		v.version, v.input_schema, v.output_schema, v.side_effect, v.approval_required,
		v.timeout_seconds, v.mcp_server_id, v.remote_tool_name
		FROM agent_tool_versions v JOIN agent_tools t ON t.id = v.tool_id AND t.tenant_id = v.tenant_id
		WHERE v.tenant_id = ? AND v.id = ?`, tenantID, versionID).Take(&row).Error; err != nil {
		return nil, nil, handleNotFound(err)
	}
	return &d.AgentTool{ID: row.ToolID, TenantID: tenantID, ToolKey: row.ToolKey, Title: row.Title,
			Enabled: row.Enabled, DeletedAt: row.DeletedAt, CurrentVersionID: row.CurrentVersionID},
		&d.ToolVersion{ID: versionID, TenantID: tenantID, ToolID: row.ToolID, Version: row.Version,
			InputSchema: json.RawMessage(row.InputSchema), OutputSchema: json.RawMessage(row.OutputSchema),
			SideEffect: row.SideEffect, ApprovalRequired: row.ApprovalRequired,
			TimeoutSeconds: row.TimeoutSeconds, MCPServerID: row.MCPServerID, RemoteToolName: row.RemoteToolName}, nil
}

func (r *Repository) GetToolByKey(ctx context.Context, tenantID, key string) (*d.AgentTool, *d.ToolVersion, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, nil, err
	}

	var row toolRow
	query := `
		SELECT t.id, t.tenant_id, t.created_at, t.updated_at, t.revision, t.tool_key, t.title, t.enabled, t.current_version_id, t.deleted_at,
		       v.version, v.input_schema, v.output_schema, v.side_effect, v.approval_required, v.timeout_seconds, v.mcp_server_id, v.remote_tool_name
		FROM agent_tools t
		JOIN agent_tool_versions v ON v.tenant_id = t.tenant_id AND v.id = t.current_version_id
		WHERE t.tenant_id = ? AND t.tool_key = ? AND t.deleted_at IS NULL
	`
	if err := db.Raw(query, tenantID, key).Take(&row).Error; err != nil {
		return nil, nil, handleNotFound(err)
	}

	tool := &d.AgentTool{
		ID:               row.ID,
		TenantID:         row.TenantID,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
		Revision:         row.Revision,
		ToolKey:          row.ToolKey,
		Title:            row.Title,
		Enabled:          row.Enabled,
		CurrentVersionID: row.CurrentVersionID,
		DeletedAt:        row.DeletedAt,
	}

	curVerID := ""
	if row.CurrentVersionID != nil {
		curVerID = *row.CurrentVersionID
	}
	ver := &d.ToolVersion{
		ID:               curVerID,
		TenantID:         row.TenantID,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
		ToolID:           row.ID,
		Version:          row.Version,
		InputSchema:      json.RawMessage(row.InputSchema),
		OutputSchema:     json.RawMessage(row.OutputSchema),
		SideEffect:       row.SideEffect,
		ApprovalRequired: row.ApprovalRequired,
		TimeoutSeconds:   row.TimeoutSeconds,
		MCPServerID:      row.MCPServerID,
		RemoteToolName:   row.RemoteToolName,
	}

	return tool, ver, nil
}

func (r *Repository) ListTools(ctx context.Context, tenantID string, query d.ListQuery) ([]d.ToolView, *string, bool, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, nil, false, err
	}

	limit := query.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	sql := `
		SELECT t.id, t.tenant_id, t.created_at, t.updated_at, t.revision, t.tool_key, t.title, t.enabled, t.current_version_id, t.deleted_at,
		       v.version, v.input_schema, v.output_schema, v.side_effect, v.approval_required, v.timeout_seconds, v.mcp_server_id, v.remote_tool_name
		FROM agent_tools t
		JOIN agent_tool_versions v ON v.tenant_id = t.tenant_id AND v.id = t.current_version_id
		WHERE t.tenant_id = ? AND t.deleted_at IS NULL
	`
	args := []any{tenantID}

	if beforeTime, beforeID, hasCursor, err := parseListCursor(query); err != nil {
		return nil, nil, false, err
	} else if hasCursor {
		sql += " AND (t.updated_at < ? OR (t.updated_at = ? AND t.id < ?))"
		args = append(args, beforeTime, beforeTime, beforeID)
	}

	sql += " ORDER BY t.updated_at DESC, t.id DESC LIMIT ?"
	args = append(args, limit+1)

	var rows []toolRow
	if err := db.Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, nil, false, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	views := make([]d.ToolView, len(rows))
	for i, r := range rows {
		curVerID := ""
		if r.CurrentVersionID != nil {
			curVerID = *r.CurrentVersionID
		}
		views[i] = d.ToolView{
			ID:               r.ID,
			Revision:         r.Revision,
			CreatedAt:        r.CreatedAt,
			UpdatedAt:        r.UpdatedAt,
			ToolKey:          r.ToolKey,
			Title:            r.Title,
			Enabled:          r.Enabled,
			InputSchema:      json.RawMessage(r.InputSchema),
			OutputSchema:     json.RawMessage(r.OutputSchema),
			SideEffect:       r.SideEffect,
			ApprovalRequired: r.ApprovalRequired,
			TimeoutSeconds:   r.TimeoutSeconds,
			MCPServerID:      r.MCPServerID,
			RemoteToolName:   r.RemoteToolName,
			CurrentVersionID: curVerID,
			Version:          r.Version,
		}
	}

	var nextCursor *string
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		c := encodeCursor(last.UpdatedAt, last.ID)
		nextCursor = &c
	}

	return views, nextCursor, hasMore, nil
}

func (r *Repository) CreateTool(ctx context.Context, tenantID string, tool *d.AgentTool, ver *d.ToolVersion) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}

	insertToolSQL := `
		INSERT INTO agent_tools (id, tenant_id, created_at, updated_at, revision, tool_key, title, enabled, current_version_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	if err := db.Exec(insertToolSQL, tool.ID, tenantID, tool.CreatedAt, tool.UpdatedAt, tool.Revision, tool.ToolKey, tool.Title, tool.Enabled, tool.CurrentVersionID).Error; err != nil {
		return err
	}

	insertVerSQL := `
		INSERT INTO agent_tool_versions (id, tenant_id, created_at, updated_at, tool_id, version, input_schema, output_schema, side_effect, approval_required, timeout_seconds, mcp_server_id, remote_tool_name)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	return db.Exec(insertVerSQL, ver.ID, tenantID, ver.CreatedAt, ver.UpdatedAt, tool.ID, ver.Version, JSONB(ver.InputSchema), JSONB(ver.OutputSchema), ver.SideEffect, ver.ApprovalRequired, ver.TimeoutSeconds, ver.MCPServerID, ver.RemoteToolName).Error
}

func (r *Repository) UpdateTool(ctx context.Context, tenantID string, tool *d.AgentTool, ver *d.ToolVersion, expectedRev int64) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}

	insertVerSQL := `
		INSERT INTO agent_tool_versions (id, tenant_id, created_at, updated_at, tool_id, version, input_schema, output_schema, side_effect, approval_required, timeout_seconds, mcp_server_id, remote_tool_name)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	if err := db.Exec(insertVerSQL, ver.ID, tenantID, ver.CreatedAt, ver.UpdatedAt, tool.ID, ver.Version, JSONB(ver.InputSchema), JSONB(ver.OutputSchema), ver.SideEffect, ver.ApprovalRequired, ver.TimeoutSeconds, ver.MCPServerID, ver.RemoteToolName).Error; err != nil {
		return err
	}

	updateToolSQL := `
		UPDATE agent_tools
		SET title = ?, enabled = ?, current_version_id = ?, revision = revision + 1, updated_at = ?
		WHERE tenant_id = ? AND id = ? AND revision = ? AND deleted_at IS NULL
	`
	res := db.Exec(updateToolSQL, tool.Title, tool.Enabled, ver.ID, tool.UpdatedAt, tenantID, tool.ID, expectedRev)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return d.ErrVersionConflict(map[string]any{"expected": expectedRev})
	}
	return nil
}

func (r *Repository) DeleteTool(ctx context.Context, tenantID, id string, expectedRev int64) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}

	if err := db.Exec(`DELETE FROM agent_tool_versions WHERE tenant_id = ? AND tool_id = ?`, tenantID, id).Error; err != nil {
		if strings.Contains(err.Error(), "23503") || strings.Contains(err.Error(), "violates foreign key constraint") {
			return d.ErrResourceInUse(map[string]any{"reason": "tool is currently in use by definitions, workflows, or active runs"})
		}
		return err
	}
	res := db.Exec(`DELETE FROM agent_tools WHERE tenant_id = ? AND id = ? AND revision = ?`, tenantID, id, expectedRev)
	if res.Error != nil {
		if strings.Contains(res.Error.Error(), "23503") || strings.Contains(res.Error.Error(), "violates foreign key constraint") {
			return d.ErrResourceInUse(map[string]any{"reason": "tool is currently in use by definitions, workflows, or active runs"})
		}
		return res.Error
	}
	if res.RowsAffected == 0 {
		return d.ErrVersionConflict(map[string]any{"expected": expectedRev})
	}
	return nil
}

func (r *Repository) IsToolInUse(ctx context.Context, tenantID, id string) (bool, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return false, err
	}

	// 1. Check if referenced in active definitions
	var defCount int64
	defQuery := `
		SELECT count(*)
		FROM agent_definition_versions dv
		JOIN agent_definitions d ON d.tenant_id = dv.tenant_id AND d.id = dv.definition_id
		WHERE dv.tenant_id = ? AND d.deleted_at IS NULL
		  AND EXISTS (
		      SELECT 1 FROM agent_tool_versions tv
		      WHERE tv.tenant_id = dv.tenant_id AND tv.tool_id = ? AND tv.id = ANY(dv.tool_version_ids)
		  )
	`
	if err := db.Raw(defQuery, tenantID, id).Scan(&defCount).Error; err == nil && defCount > 0 {
		return true, nil
	}

	// 2. Keep all run history intact before physically deleting tool versions.
	var nodeCount int64
	nodeQuery := `
		SELECT count(*)
		FROM agent_run_nodes rn
		JOIN agent_runs r ON r.tenant_id = rn.tenant_id AND r.id = rn.run_id
		WHERE rn.tenant_id = ?
		  AND rn.tool_version_id IN (SELECT id FROM agent_tool_versions WHERE tenant_id = ? AND tool_id = ?)
	`
	if err := db.Raw(nodeQuery, tenantID, tenantID, id).Scan(&nodeCount).Error; err == nil && nodeCount > 0 {
		return true, nil
	}

	return false, nil
}

func (r *Repository) CheckToolVersionsExist(ctx context.Context, tenantID string, versionIDs []string) (bool, error) {
	if len(versionIDs) == 0 {
		return true, nil
	}
	db, err := r.db(ctx, false)
	if err != nil {
		return false, err
	}

	unique := make(map[string]bool, len(versionIDs))
	var uniqueIDs []string
	for _, id := range versionIDs {
		if !unique[id] {
			unique[id] = true
			uniqueIDs = append(uniqueIDs, id)
		}
	}

	var count int64
	query := `
		SELECT count(DISTINCT id)
		FROM agent_tool_versions
		WHERE tenant_id = ? AND id IN (?)
	`
	if err := db.Raw(query, tenantID, uniqueIDs).Scan(&count).Error; err != nil {
		return false, err
	}
	return int(count) == len(uniqueIDs), nil
}
