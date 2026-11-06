package infrastructure

import (
	"context"
	"errors"
	"time"

	d "InkLore/internal/agent/domain"

	"gorm.io/gorm"
)

type auditRunRow struct {
	ID              string
	Revision        int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Kind            string
	TaskKind        string
	Status          d.RunStatus
	NodeCount       int
	DurationMS      int64
	ErrorCode       *string
	UsageReceiptIDs TextArray `gorm:"column:usage_receipt_ids;type:text[]"`
}

func (row auditRunRow) record() d.AuditRunRecord {
	return d.AuditRunRecord{View: d.AuditRunView{
		ID: row.ID, Revision: row.Revision, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		Kind: row.Kind, TaskKind: row.TaskKind, Status: row.Status, NodeCount: row.NodeCount,
		DurationMS: row.DurationMS, ErrorCode: row.ErrorCode,
	}, ReceiptIDs: []string(row.UsageReceiptIDs)}
}

// Select only diagnostic metadata, including for the detail endpoint. Do not
// load a full run and rely on later JSON omission for the privacy boundary.
func auditRunQuery(db *gorm.DB, tenantID string) *gorm.DB {
	return db.Table("agent_runs AS r").Select(`r.id, r.revision, r.created_at, r.updated_at, r.kind, r.task_kind, r.status, r.error_code, r.usage_receipt_ids,
		(SELECT count(*) FROM agent_run_nodes AS n WHERE n.tenant_id = r.tenant_id AND n.run_id = r.id) AS node_count,
		CASE WHEN r.started_at IS NULL THEN 0 ELSE LEAST(2147483647, GREATEST(0, floor(EXTRACT(EPOCH FROM (COALESCE(r.finished_at, CURRENT_TIMESTAMP) - r.started_at)) * 1000))) END::bigint AS duration_ms`).
		Where("r.tenant_id = ? AND r.deleted_at IS NULL", tenantID)
}

func (r *RunRepositoryPostgres) ListAuditRuns(ctx context.Context, tenantID string, query d.ListQuery) ([]d.AuditRunRecord, bool, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, false, err
	}
	limit := query.Limit
	if limit < 1 || limit > 100 {
		return nil, false, d.ErrValidationFailed(nil)
	}
	q := auditRunQuery(db, tenantID)
	if query.BeforeTime != "" && query.BeforeID != "" {
		q = q.Where("(r.updated_at, r.id) < (?, ?)", query.BeforeTime, query.BeforeID)
	}
	var rows []auditRunRow
	if err := q.Order("r.updated_at DESC, r.id DESC").Limit(limit + 1).Scan(&rows).Error; err != nil {
		return nil, false, err
	}
	more := len(rows) > limit
	if more {
		rows = rows[:limit]
	}
	records := make([]d.AuditRunRecord, 0, len(rows))
	for _, row := range rows {
		records = append(records, row.record())
	}
	return records, more, nil
}

func (r *RunRepositoryPostgres) GetAuditRun(ctx context.Context, tenantID, id string) (*d.AuditRunRecord, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, err
	}
	var row auditRunRow
	err = auditRunQuery(db, tenantID).Where("r.id = ?", id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, d.ErrNotFound()
	}
	if err != nil {
		return nil, err
	}
	record := row.record()
	return &record, nil
}
