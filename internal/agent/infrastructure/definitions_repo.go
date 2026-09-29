package infrastructure

import (
	d "InkLore/internal/agent/domain"
	"context"
	"time"
)

type definitionRow struct {
	ID                string     `gorm:"column:id"`
	TenantID          string     `gorm:"column:tenant_id"`
	CreatedAt         time.Time  `gorm:"column:created_at"`
	UpdatedAt         time.Time  `gorm:"column:updated_at"`
	Revision          int64      `gorm:"column:revision"`
	DefinitionKey     string     `gorm:"column:definition_key"`
	Title             string     `gorm:"column:title"`
	Enabled           bool       `gorm:"column:enabled"`
	CurrentVersionID  *string    `gorm:"column:current_version_id"`
	DeletedAt         *time.Time `gorm:"column:deleted_at"`
	Version           int        `gorm:"column:version"`
	SystemInstruction string     `gorm:"column:system_instruction"`
	ToolVersionIDs    UUIDArray  `gorm:"column:tool_version_ids"`
	TaskKinds         TextArray  `gorm:"column:task_kinds"`
}

func (r *Repository) GetDefinition(ctx context.Context, tenantID, id string) (*d.AgentDefinition, *d.DefinitionVersion, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, nil, err
	}

	var row definitionRow
	query := `
		SELECT d.id, d.tenant_id, d.created_at, d.updated_at, d.revision, d.definition_key, d.title, d.enabled, d.current_version_id, d.deleted_at,
		       v.version, v.system_instruction, v.tool_version_ids, v.task_kinds
		FROM agent_definitions d
		JOIN agent_definition_versions v ON v.tenant_id = d.tenant_id AND v.id = d.current_version_id
		WHERE d.tenant_id = ? AND d.id = ? AND d.deleted_at IS NULL
	`
	if err := db.Raw(query, tenantID, id).Take(&row).Error; err != nil {
		return nil, nil, handleNotFound(err)
	}

	def := &d.AgentDefinition{
		ID:               row.ID,
		TenantID:         row.TenantID,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
		Revision:         row.Revision,
		DefinitionKey:    row.DefinitionKey,
		Title:            row.Title,
		Enabled:          row.Enabled,
		CurrentVersionID: row.CurrentVersionID,
		DeletedAt:        row.DeletedAt,
	}

	curVerID := ""
	if row.CurrentVersionID != nil {
		curVerID = *row.CurrentVersionID
	}
	ver := &d.DefinitionVersion{
		ID:                curVerID,
		TenantID:          row.TenantID,
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
		DefinitionID:      row.ID,
		Version:           row.Version,
		SystemInstruction: row.SystemInstruction,
		ToolVersionIDs:    row.ToolVersionIDs,
		TaskKinds:         row.TaskKinds,
	}

	return def, ver, nil
}

// GetDefinitionVersion resolves a run's frozen version, never the current head.
// Retiring a definition only stops new selection; it must not break running jobs.
func (r *Repository) GetDefinitionVersion(ctx context.Context, tenantID, versionID string) (*d.DefinitionVersion, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, err
	}

	var row struct {
		ID                string    `gorm:"column:id"`
		TenantID          string    `gorm:"column:tenant_id"`
		CreatedAt         time.Time `gorm:"column:created_at"`
		UpdatedAt         time.Time `gorm:"column:updated_at"`
		DefinitionID      string    `gorm:"column:definition_id"`
		Version           int       `gorm:"column:version"`
		SystemInstruction string    `gorm:"column:system_instruction"`
		ToolVersionIDs    UUIDArray `gorm:"column:tool_version_ids"`
		TaskKinds         TextArray `gorm:"column:task_kinds"`
	}
	if err := db.Raw(`SELECT id, tenant_id, created_at, updated_at, definition_id, version,
		system_instruction, tool_version_ids, task_kinds
		FROM agent_definition_versions WHERE tenant_id = ? AND id = ?`, tenantID, versionID).Take(&row).Error; err != nil {
		return nil, handleNotFound(err)
	}
	return &d.DefinitionVersion{
		ID:                row.ID,
		TenantID:          row.TenantID,
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
		DefinitionID:      row.DefinitionID,
		Version:           row.Version,
		SystemInstruction: row.SystemInstruction,
		ToolVersionIDs:    row.ToolVersionIDs,
		TaskKinds:         row.TaskKinds,
	}, nil
}

func (r *Repository) GetDefinitionByKey(ctx context.Context, tenantID, key string) (*d.AgentDefinition, *d.DefinitionVersion, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, nil, err
	}

	var row definitionRow
	query := `
		SELECT d.id, d.tenant_id, d.created_at, d.updated_at, d.revision, d.definition_key, d.title, d.enabled, d.current_version_id, d.deleted_at,
		       v.version, v.system_instruction, v.tool_version_ids, v.task_kinds
		FROM agent_definitions d
		JOIN agent_definition_versions v ON v.tenant_id = d.tenant_id AND v.id = d.current_version_id
		WHERE d.tenant_id = ? AND d.definition_key = ? AND d.deleted_at IS NULL
	`
	if err := db.Raw(query, tenantID, key).Take(&row).Error; err != nil {
		return nil, nil, handleNotFound(err)
	}

	def := &d.AgentDefinition{
		ID:               row.ID,
		TenantID:         row.TenantID,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
		Revision:         row.Revision,
		DefinitionKey:    row.DefinitionKey,
		Title:            row.Title,
		Enabled:          row.Enabled,
		CurrentVersionID: row.CurrentVersionID,
		DeletedAt:        row.DeletedAt,
	}

	curVerID := ""
	if row.CurrentVersionID != nil {
		curVerID = *row.CurrentVersionID
	}
	ver := &d.DefinitionVersion{
		ID:                curVerID,
		TenantID:          row.TenantID,
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
		DefinitionID:      row.ID,
		Version:           row.Version,
		SystemInstruction: row.SystemInstruction,
		ToolVersionIDs:    row.ToolVersionIDs,
		TaskKinds:         row.TaskKinds,
	}

	return def, ver, nil
}

func (r *Repository) ListDefinitions(ctx context.Context, tenantID string, query d.ListQuery) ([]d.DefinitionView, *string, bool, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, nil, false, err
	}

	limit := query.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	sql := `
		SELECT d.id, d.tenant_id, d.created_at, d.updated_at, d.revision, d.definition_key, d.title, d.enabled, d.current_version_id, d.deleted_at,
		       v.version, v.system_instruction, v.tool_version_ids, v.task_kinds
		FROM agent_definitions d
		JOIN agent_definition_versions v ON v.tenant_id = d.tenant_id AND v.id = d.current_version_id
		WHERE d.tenant_id = ? AND d.deleted_at IS NULL
	`
	args := []any{tenantID}

	if beforeTime, beforeID, hasCursor, err := parseListCursor(query); err != nil {
		return nil, nil, false, err
	} else if hasCursor {
		sql += " AND (d.updated_at < ? OR (d.updated_at = ? AND d.id < ?))"
		args = append(args, beforeTime, beforeTime, beforeID)
	}

	sql += " ORDER BY d.updated_at DESC, d.id DESC LIMIT ?"
	args = append(args, limit+1)

	var rows []definitionRow
	if err := db.Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, nil, false, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	views := make([]d.DefinitionView, len(rows))
	for i, r := range rows {
		curVerID := ""
		if r.CurrentVersionID != nil {
			curVerID = *r.CurrentVersionID
		}
		pubStatus := d.PublicationStatusDraft
		if r.Enabled {
			pubStatus = d.PublicationStatusPublished
		}
		views[i] = d.DefinitionView{
			ID:                r.ID,
			Revision:          r.Revision,
			CreatedAt:         r.CreatedAt,
			UpdatedAt:         r.UpdatedAt,
			DefinitionKey:     r.DefinitionKey,
			Title:             r.Title,
			Enabled:           r.Enabled,
			SystemInstruction: r.SystemInstruction,
			ToolVersionIDs:    r.ToolVersionIDs,
			TaskKinds:         r.TaskKinds,
			CurrentVersionID:  curVerID,
			Version:           r.Version,
			PublicationStatus: pubStatus,
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

func (r *Repository) CreateDefinition(ctx context.Context, tenantID string, def *d.AgentDefinition, ver *d.DefinitionVersion) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}

	// First insert definition head with current_version_id deferred or inserted first
	// Notice FK on agent_definitions(current_version_id) is DEFERRABLE INITIALLY DEFERRED
	// or we can insert definition head, insert version, then set current_version_id.
	insertDefSQL := `
		INSERT INTO agent_definitions (id, tenant_id, created_at, updated_at, revision, definition_key, title, enabled, current_version_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	if err := db.Exec(insertDefSQL, def.ID, tenantID, def.CreatedAt, def.UpdatedAt, def.Revision, def.DefinitionKey, def.Title, def.Enabled, def.CurrentVersionID).Error; err != nil {
		return err
	}

	insertVerSQL := `
		INSERT INTO agent_definition_versions (id, tenant_id, created_at, updated_at, definition_id, version, system_instruction, tool_version_ids, task_kinds)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	return db.Exec(insertVerSQL, ver.ID, tenantID, ver.CreatedAt, ver.UpdatedAt, def.ID, ver.Version, ver.SystemInstruction, UUIDArray(ver.ToolVersionIDs), TextArray(ver.TaskKinds)).Error
}

func (r *Repository) UpdateDefinition(ctx context.Context, tenantID string, def *d.AgentDefinition, ver *d.DefinitionVersion, expectedRev int64) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}

	// Insert immutable version
	insertVerSQL := `
		INSERT INTO agent_definition_versions (id, tenant_id, created_at, updated_at, definition_id, version, system_instruction, tool_version_ids, task_kinds)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	if err := db.Exec(insertVerSQL, ver.ID, tenantID, ver.CreatedAt, ver.UpdatedAt, def.ID, ver.Version, ver.SystemInstruction, UUIDArray(ver.ToolVersionIDs), TextArray(ver.TaskKinds)).Error; err != nil {
		return err
	}

	// CAS update on mutable head
	updateDefSQL := `
		UPDATE agent_definitions
		SET title = ?, enabled = ?, current_version_id = ?, revision = revision + 1, updated_at = ?
		WHERE tenant_id = ? AND id = ? AND revision = ? AND deleted_at IS NULL
	`
	res := db.Exec(updateDefSQL, def.Title, def.Enabled, ver.ID, def.UpdatedAt, tenantID, def.ID, expectedRev)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return d.ErrVersionConflict(map[string]any{"expected": expectedRev})
	}
	return nil
}

// DeleteDefinition retires the mutable head via CAS. Immutable versions and
// their run/node/guide references remain intact for replay and in-flight work.
func (r *Repository) DeleteDefinition(ctx context.Context, tenantID, id string, expectedRev int64) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	res := db.Exec(`UPDATE agent_definitions
		SET enabled = false, deleted_at = ?, updated_at = ?, revision = revision + 1
		WHERE tenant_id = ? AND id = ? AND revision = ? AND deleted_at IS NULL`,
		now, now, tenantID, id, expectedRev)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return d.ErrVersionConflict(map[string]any{"expected": expectedRev})
	}
	return nil
}

func (r *Repository) ListCatalogDefinitions(ctx context.Context, tenantID string, query d.ListQuery) ([]d.CatalogDefinition, *string, bool, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, nil, false, err
	}

	limit := query.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	sql := `
		SELECT d.id, d.title, d.current_version_id, v.task_kinds, d.updated_at
		FROM agent_definitions d
		JOIN agent_definition_versions v ON v.tenant_id = d.tenant_id AND v.id = d.current_version_id
		WHERE d.tenant_id = ? AND d.enabled = true AND d.deleted_at IS NULL
	`
	args := []any{tenantID}

	if beforeTime, beforeID, hasCursor, err := parseListCursor(query); err != nil {
		return nil, nil, false, err
	} else if hasCursor {
		sql += " AND (d.updated_at < ? OR (d.updated_at = ? AND d.id < ?))"
		args = append(args, beforeTime, beforeTime, beforeID)
	}

	sql += " ORDER BY d.updated_at DESC, d.id DESC LIMIT ?"
	args = append(args, limit+1)

	type catalogRow struct {
		ID               string    `gorm:"column:id"`
		Title            string    `gorm:"column:title"`
		CurrentVersionID string    `gorm:"column:current_version_id"`
		TaskKinds        TextArray `gorm:"column:task_kinds"`
		UpdatedAt        time.Time `gorm:"column:updated_at"`
	}

	var rows []catalogRow
	if err := db.Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, nil, false, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	items := make([]d.CatalogDefinition, len(rows))
	for i, r := range rows {
		items[i] = d.CatalogDefinition{
			ID:        r.ID,
			Title:     r.Title,
			VersionID: r.CurrentVersionID,
			TaskKinds: r.TaskKinds,
		}
	}

	var nextCursor *string
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		c := encodeCursor(last.UpdatedAt, last.ID)
		nextCursor = &c
	}

	return items, nextCursor, hasMore, nil
}
