package infrastructure

import (
	d "InkLore/internal/agent/domain"
	"context"
	"time"
)

type mcpServerRow struct {
	ID             string     `gorm:"column:id"`
	TenantID       string     `gorm:"column:tenant_id"`
	CreatedAt      time.Time  `gorm:"column:created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at"`
	Revision       int64      `gorm:"column:revision"`
	Title          string     `gorm:"column:title"`
	Endpoint       string     `gorm:"column:endpoint"`
	EnvRef         string     `gorm:"column:env_ref"`
	AllowedMethods TextArray  `gorm:"column:allowed_methods"`
	Enabled        bool       `gorm:"column:enabled"`
	DeletedAt      *time.Time `gorm:"column:deleted_at"`
}

func (r *Repository) GetMCPServer(ctx context.Context, tenantID, id string) (*d.MCPServer, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, err
	}

	var row mcpServerRow
	query := `
		SELECT id, tenant_id, created_at, updated_at, revision, title, endpoint, env_ref, allowed_methods, enabled, deleted_at
		FROM agent_mcp_servers
		WHERE tenant_id = ? AND id = ? AND deleted_at IS NULL
	`
	if err := db.Raw(query, tenantID, id).Take(&row).Error; err != nil {
		return nil, handleNotFound(err)
	}

	return &d.MCPServer{
		ID:             row.ID,
		TenantID:       row.TenantID,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		Revision:       row.Revision,
		Title:          row.Title,
		Endpoint:       row.Endpoint,
		EnvRef:         row.EnvRef,
		AllowedMethods: row.AllowedMethods,
		Enabled:        row.Enabled,
		DeletedAt:      row.DeletedAt,
	}, nil
}

func (r *Repository) ListMCPServers(ctx context.Context, tenantID string, query d.ListQuery) ([]d.MCPServer, *string, bool, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, nil, false, err
	}

	limit := query.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	sql := `
		SELECT id, tenant_id, created_at, updated_at, revision, title, endpoint, env_ref, allowed_methods, enabled, deleted_at
		FROM agent_mcp_servers
		WHERE tenant_id = ? AND deleted_at IS NULL
	`
	args := []any{tenantID}

	if beforeTime, beforeID, hasCursor, err := parseListCursor(query); err != nil {
		return nil, nil, false, err
	} else if hasCursor {
		sql += " AND (updated_at < ? OR (updated_at = ? AND id < ?))"
		args = append(args, beforeTime, beforeTime, beforeID)
	}

	sql += " ORDER BY updated_at DESC, id DESC LIMIT ?"
	args = append(args, limit+1)

	var rows []mcpServerRow
	if err := db.Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, nil, false, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	servers := make([]d.MCPServer, len(rows))
	for i, r := range rows {
		servers[i] = d.MCPServer{
			ID:             r.ID,
			TenantID:       r.TenantID,
			CreatedAt:      r.CreatedAt,
			UpdatedAt:      r.UpdatedAt,
			Revision:       r.Revision,
			Title:          r.Title,
			Endpoint:       r.Endpoint,
			EnvRef:         r.EnvRef,
			AllowedMethods: r.AllowedMethods,
			Enabled:        r.Enabled,
			DeletedAt:      r.DeletedAt,
		}
	}

	var nextCursor *string
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		c := encodeCursor(last.UpdatedAt, last.ID)
		nextCursor = &c
	}

	return servers, nextCursor, hasMore, nil
}

func (r *Repository) CreateMCPServer(ctx context.Context, tenantID string, srv *d.MCPServer) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}

	sql := `
		INSERT INTO agent_mcp_servers (id, tenant_id, created_at, updated_at, revision, title, endpoint, env_ref, allowed_methods, enabled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	return db.Exec(sql, srv.ID, tenantID, srv.CreatedAt, srv.UpdatedAt, srv.Revision, srv.Title, srv.Endpoint, srv.EnvRef, TextArray(srv.AllowedMethods), srv.Enabled).Error
}

func (r *Repository) UpdateMCPServer(ctx context.Context, tenantID string, srv *d.MCPServer, expectedRev int64) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}

	sql := `
		UPDATE agent_mcp_servers
		SET title = ?, endpoint = ?, env_ref = ?, allowed_methods = ?, enabled = ?, revision = revision + 1, updated_at = ?
		WHERE tenant_id = ? AND id = ? AND revision = ? AND deleted_at IS NULL
	`
	res := db.Exec(sql, srv.Title, srv.Endpoint, srv.EnvRef, TextArray(srv.AllowedMethods), srv.Enabled, srv.UpdatedAt, tenantID, srv.ID, expectedRev)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return d.ErrVersionConflict(map[string]any{"expected": expectedRev})
	}
	return nil
}

func (r *Repository) DeleteMCPServer(ctx context.Context, tenantID, id string, expectedRev int64) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}

	sql := `
		UPDATE agent_mcp_servers
		SET deleted_at = now(), revision = revision + 1, updated_at = now()
		WHERE tenant_id = ? AND id = ? AND revision = ? AND deleted_at IS NULL
	`
	res := db.Exec(sql, tenantID, id, expectedRev)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return d.ErrVersionConflict(map[string]any{"expected": expectedRev})
	}
	return nil
}

func (r *Repository) IsMCPServerInUse(ctx context.Context, tenantID, id string) (bool, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return false, err
	}

	var count int64
	query := `
		SELECT count(*)
		FROM agent_tool_versions tv
		JOIN agent_tools t ON t.tenant_id = tv.tenant_id AND t.id = tv.tool_id
		WHERE tv.tenant_id = ? AND tv.mcp_server_id = ? AND t.deleted_at IS NULL
	`
	if err := db.Raw(query, tenantID, id).Scan(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
