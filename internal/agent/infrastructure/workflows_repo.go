package infrastructure

import (
	d "InkLore/internal/agent/domain"
	"context"
	"encoding/json"
	"errors"
	"time"
)

type workflowRow struct {
	ID               string     `gorm:"column:id"`
	TenantID         string     `gorm:"column:tenant_id"`
	CreatedAt        time.Time  `gorm:"column:created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at"`
	Revision         int64      `gorm:"column:revision"`
	WorkflowKey      string     `gorm:"column:workflow_key"`
	Title            string     `gorm:"column:title"`
	Enabled          bool       `gorm:"column:enabled"`
	CurrentVersionID *string    `gorm:"column:current_version_id"`
	DeletedAt        *time.Time `gorm:"column:deleted_at"`
	Version          int        `gorm:"column:version"`
	ManifestJSON     JSONB      `gorm:"column:manifest"`
	ManifestHash     string     `gorm:"column:manifest_hash"`
}

func (r *Repository) GetWorkflow(ctx context.Context, tenantID, id string) (*d.AgentWorkflow, *d.WorkflowVersion, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, nil, err
	}

	var row workflowRow
	query := `
		SELECT w.id, w.tenant_id, w.created_at, w.updated_at, w.revision, w.workflow_key, w.title, w.enabled, w.current_version_id, w.deleted_at,
		       v.version, v.manifest, v.manifest_hash
		FROM agent_workflows w
		JOIN agent_workflow_versions v ON v.tenant_id = w.tenant_id AND v.id = w.current_version_id
		WHERE w.tenant_id = ? AND w.id = ? AND w.deleted_at IS NULL
	`
	if err := db.Raw(query, tenantID, id).Take(&row).Error; err != nil {
		return nil, nil, handleNotFound(err)
	}

	wf := &d.AgentWorkflow{
		ID:               row.ID,
		TenantID:         row.TenantID,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
		Revision:         row.Revision,
		WorkflowKey:      row.WorkflowKey,
		Title:            row.Title,
		Enabled:          row.Enabled,
		CurrentVersionID: row.CurrentVersionID,
		DeletedAt:        row.DeletedAt,
	}

	var manifest d.WorkflowManifest
	if len(row.ManifestJSON) == 0 {
		return nil, nil, errors.New("agent workflow manifest is empty")
	}
	if err := json.Unmarshal(row.ManifestJSON, &manifest); err != nil {
		return nil, nil, err
	}
	if err := d.ValidateDAG(manifest); err != nil {
		return nil, nil, err
	}

	curVerID := ""
	if row.CurrentVersionID != nil {
		curVerID = *row.CurrentVersionID
	}
	ver := &d.WorkflowVersion{
		ID:           curVerID,
		TenantID:     row.TenantID,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
		WorkflowID:   row.ID,
		Version:      row.Version,
		Manifest:     manifest,
		ManifestHash: row.ManifestHash,
	}

	return wf, ver, nil
}

func (r *Repository) GetWorkflowByKey(ctx context.Context, tenantID, key string) (*d.AgentWorkflow, *d.WorkflowVersion, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, nil, err
	}

	var row workflowRow
	query := `
		SELECT w.id, w.tenant_id, w.created_at, w.updated_at, w.revision, w.workflow_key, w.title, w.enabled, w.current_version_id, w.deleted_at,
		       v.version, v.manifest, v.manifest_hash
		FROM agent_workflows w
		JOIN agent_workflow_versions v ON v.tenant_id = w.tenant_id AND v.id = w.current_version_id
		WHERE w.tenant_id = ? AND w.workflow_key = ? AND w.deleted_at IS NULL
	`
	if err := db.Raw(query, tenantID, key).Take(&row).Error; err != nil {
		return nil, nil, handleNotFound(err)
	}

	wf := &d.AgentWorkflow{
		ID:               row.ID,
		TenantID:         row.TenantID,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
		Revision:         row.Revision,
		WorkflowKey:      row.WorkflowKey,
		Title:            row.Title,
		Enabled:          row.Enabled,
		CurrentVersionID: row.CurrentVersionID,
		DeletedAt:        row.DeletedAt,
	}

	var manifest d.WorkflowManifest
	if len(row.ManifestJSON) == 0 {
		return nil, nil, errors.New("agent workflow manifest is empty")
	}
	if err := json.Unmarshal(row.ManifestJSON, &manifest); err != nil {
		return nil, nil, err
	}
	if err := d.ValidateDAG(manifest); err != nil {
		return nil, nil, err
	}

	curVerID := ""
	if row.CurrentVersionID != nil {
		curVerID = *row.CurrentVersionID
	}
	ver := &d.WorkflowVersion{
		ID:           curVerID,
		TenantID:     row.TenantID,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
		WorkflowID:   row.ID,
		Version:      row.Version,
		Manifest:     manifest,
		ManifestHash: row.ManifestHash,
	}

	return wf, ver, nil
}

func (r *Repository) ListWorkflows(ctx context.Context, tenantID string, query d.ListQuery) ([]d.WorkflowView, *string, bool, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, nil, false, err
	}

	limit := query.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	sql := `
		SELECT w.id, w.tenant_id, w.created_at, w.updated_at, w.revision, w.workflow_key, w.title, w.enabled, w.current_version_id, w.deleted_at,
		       v.version, v.manifest, v.manifest_hash
		FROM agent_workflows w
		JOIN agent_workflow_versions v ON v.tenant_id = w.tenant_id AND v.id = w.current_version_id
		WHERE w.tenant_id = ? AND w.deleted_at IS NULL
	`
	args := []any{tenantID}

	if beforeTime, beforeID, hasCursor, err := parseListCursor(query); err != nil {
		return nil, nil, false, err
	} else if hasCursor {
		sql += " AND (w.updated_at < ? OR (w.updated_at = ? AND w.id < ?))"
		args = append(args, beforeTime, beforeTime, beforeID)
	}

	sql += " ORDER BY w.updated_at DESC, w.id DESC LIMIT ?"
	args = append(args, limit+1)

	var rows []workflowRow
	if err := db.Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, nil, false, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	views := make([]d.WorkflowView, len(rows))
	for i, r := range rows {
		var manifest d.WorkflowManifest
		if len(r.ManifestJSON) == 0 {
			return nil, nil, false, errors.New("agent workflow manifest is empty")
		}
		if err := json.Unmarshal(r.ManifestJSON, &manifest); err != nil {
			return nil, nil, false, err
		}
		if err := d.ValidateDAG(manifest); err != nil {
			return nil, nil, false, err
		}
		curVerID := ""
		if r.CurrentVersionID != nil {
			curVerID = *r.CurrentVersionID
		}
		views[i] = d.WorkflowView{
			ID:               r.ID,
			Revision:         r.Revision,
			CreatedAt:        r.CreatedAt,
			UpdatedAt:        r.UpdatedAt,
			WorkflowKey:      r.WorkflowKey,
			Title:            r.Title,
			Enabled:          r.Enabled,
			Manifest:         manifest,
			CurrentVersionID: curVerID,
			Version:          r.Version,
			ManifestHash:     r.ManifestHash,
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

func (r *Repository) CreateWorkflow(ctx context.Context, tenantID string, wf *d.AgentWorkflow, ver *d.WorkflowVersion) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}

	manifestRaw, err := json.Marshal(ver.Manifest)
	if err != nil {
		return err
	}

	insertWfSQL := `
		INSERT INTO agent_workflows (id, tenant_id, created_at, updated_at, revision, workflow_key, title, enabled, current_version_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	if err := db.Exec(insertWfSQL, wf.ID, tenantID, wf.CreatedAt, wf.UpdatedAt, wf.Revision, wf.WorkflowKey, wf.Title, wf.Enabled, wf.CurrentVersionID).Error; err != nil {
		return err
	}

	insertVerSQL := `
		INSERT INTO agent_workflow_versions (id, tenant_id, created_at, updated_at, workflow_id, version, manifest, manifest_hash)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	return db.Exec(insertVerSQL, ver.ID, tenantID, ver.CreatedAt, ver.UpdatedAt, wf.ID, ver.Version, JSONB(manifestRaw), ver.ManifestHash).Error
}

func (r *Repository) UpdateWorkflow(ctx context.Context, tenantID string, wf *d.AgentWorkflow, ver *d.WorkflowVersion, expectedRev int64) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}

	manifestRaw, err := json.Marshal(ver.Manifest)
	if err != nil {
		return err
	}

	insertVerSQL := `
		INSERT INTO agent_workflow_versions (id, tenant_id, created_at, updated_at, workflow_id, version, manifest, manifest_hash)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	if err := db.Exec(insertVerSQL, ver.ID, tenantID, ver.CreatedAt, ver.UpdatedAt, wf.ID, ver.Version, JSONB(manifestRaw), ver.ManifestHash).Error; err != nil {
		return err
	}

	updateWfSQL := `
		UPDATE agent_workflows
		SET title = ?, enabled = ?, current_version_id = ?, revision = revision + 1, updated_at = ?
		WHERE tenant_id = ? AND id = ? AND revision = ? AND deleted_at IS NULL
	`
	res := db.Exec(updateWfSQL, wf.Title, wf.Enabled, ver.ID, wf.UpdatedAt, tenantID, wf.ID, expectedRev)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return d.ErrVersionConflict(map[string]any{"expected": expectedRev})
	}
	return nil
}

// DeleteWorkflow retires the mutable head via CAS without touching immutable
// manifests or the runs bound to their workflow versions.
func (r *Repository) DeleteWorkflow(ctx context.Context, tenantID, id string, expectedRev int64) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	res := db.Exec(`UPDATE agent_workflows
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

func (r *Repository) ListCatalogWorkflows(ctx context.Context, tenantID string, query d.ListQuery) ([]d.CatalogWorkflow, *string, bool, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, nil, false, err
	}

	limit := query.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	sql := `
		SELECT id, title, current_version_id, updated_at
		FROM agent_workflows
		WHERE tenant_id = ? AND enabled = true AND deleted_at IS NULL
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

	type catalogWfRow struct {
		ID               string    `gorm:"column:id"`
		Title            string    `gorm:"column:title"`
		CurrentVersionID string    `gorm:"column:current_version_id"`
		UpdatedAt        time.Time `gorm:"column:updated_at"`
	}

	var rows []catalogWfRow
	if err := db.Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, nil, false, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	items := make([]d.CatalogWorkflow, len(rows))
	for i, r := range rows {
		items[i] = d.CatalogWorkflow{
			ID:        r.ID,
			Title:     r.Title,
			VersionID: r.CurrentVersionID,
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
