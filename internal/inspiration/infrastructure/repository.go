// package infrastructure implements inspiration PostgreSQL persistence scoped strictly to inspiration tables.
package infrastructure

import (
	d "InkLore/internal/inspiration/domain"
	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/uow"
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Repository struct {
	manager *uow.Manager
}

func NewRepository(m *uow.Manager) *Repository {
	return &Repository{manager: m}
}

func (r *Repository) db(ctx context.Context, write bool) (*gorm.DB, error) {
	var db *gorm.DB
	var err error
	if write {
		db, err = r.manager.Require(ctx)
	} else {
		db, err = r.manager.DB(ctx)
	}
	if err != nil {
		return nil, err
	}
	return db.Session(&gorm.Session{Logger: logger.Discard}), nil
}

// --- Asset operations ---

func (r *Repository) InsertAssetWithSegments(ctx context.Context, asset d.Asset, spans []d.SegmentSpan) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}
	const assetSQL = `INSERT INTO inspiration_assets (id, tenant_id, created_at, updated_at, revision, owner_user_id, file_id, title, source_kind, author, source_url, source_note, rights_status, content_hash, codepoint_count, state, deleted_at, redacted_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	res := db.Exec(assetSQL, asset.ID, asset.TenantID, asset.CreatedAt, asset.UpdatedAt, asset.Revision, asset.OwnerUserID, asset.FileID, asset.Title, asset.SourceKind, asset.Author, asset.SourceURL, asset.SourceNote, asset.RightsStatus, asset.ContentHash, asset.CodepointCount, asset.State, asset.DeletedAt, asset.RedactedAt)
	if res.Error != nil {
		return res.Error
	}
	const segSQL = `INSERT INTO inspiration_segments (id, tenant_id, created_at, updated_at, asset_id, seq, start_offset, end_offset) VALUES (gen_random_uuid(), ?, ?, ?, ?, ?, ?, ?)`
	for _, span := range spans {
		segRes := db.Exec(segSQL, asset.TenantID, asset.CreatedAt, asset.UpdatedAt, asset.ID, span.Seq, span.StartOffset, span.EndOffset)
		if segRes.Error != nil {
			return segRes.Error
		}
	}
	return nil
}

func (r *Repository) UpdateAsset(ctx context.Context, asset d.Asset, expectedRevision int64) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}
	res := db.Exec(`UPDATE inspiration_assets SET title=?, author=?, source_url=?, source_note=?, rights_status=?, revision=?, updated_at=? WHERE tenant_id=? AND id=? AND revision=? AND state!='deleted'`,
		asset.Title, asset.Author, asset.SourceURL, asset.SourceNote, asset.RightsStatus, asset.Revision, asset.UpdatedAt, asset.TenantID, asset.ID, expectedRevision)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return pe.Contract(409, "inspiration.version_conflict", map[string]any{"current_revision": asset.Revision})
	}
	return nil
}

func (r *Repository) SoftDeleteAsset(ctx context.Context, tenantID, id string, expectedRevision int64, now time.Time) (int64, error) {
	db, err := r.db(ctx, true)
	if err != nil {
		return 0, err
	}
	var newRev int64
	row := db.Raw(`UPDATE inspiration_assets SET state='deleted', deleted_at=?, revision=revision+1, updated_at=? WHERE tenant_id=? AND id=? AND revision=? AND state='ready' RETURNING revision`, now, now, tenantID, id, expectedRevision).Row()
	if err := row.Scan(&newRev); err != nil {
		return 0, pe.Contract(409, "inspiration.version_conflict", nil)
	}
	_ = db.Exec(`UPDATE inspiration_analyses SET state='cancelled', failure_code='inspiration.asset_deleted', updated_at=? WHERE tenant_id=? AND asset_id=? AND state='pending'`, now, tenantID, id)
	return newRev, nil
}

func (r *Repository) RestoreAsset(ctx context.Context, tenantID, id string, expectedRevision int64, now time.Time) (int64, error) {
	db, err := r.db(ctx, true)
	if err != nil {
		return 0, err
	}
	var newRev int64
	row := db.Raw(`UPDATE inspiration_assets SET state='ready', deleted_at=NULL, revision=revision+1, updated_at=? WHERE tenant_id=? AND id=? AND revision=? AND state='deleted' RETURNING revision`, now, tenantID, id, expectedRevision).Row()
	if err := row.Scan(&newRev); err != nil {
		return 0, pe.Contract(409, "inspiration.version_conflict", nil)
	}
	return newRev, nil
}

func (r *Repository) GetAsset(ctx context.Context, tenantID, id string, includeDeleted bool) (d.Asset, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return d.Asset{}, err
	}
	var out d.Asset
	where := "tenant_id=? AND id=?"
	if !includeDeleted {
		where += " AND deleted_at IS NULL"
	}
	res := db.Table("inspiration_assets").Where(where, tenantID, id).Take(&out)
	if errors.Is(res.Error, gorm.ErrRecordNotFound) || res.RowsAffected != 1 {
		return out, pe.Contract(404, "platform.resource_not_found", nil)
	}
	return out, res.Error
}

func (r *Repository) CheckAssetDuplicate(ctx context.Context, tenantID, ownerUserID, contentHash string) (*string, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, err
	}
	var existingID string
	res := db.Table("inspiration_assets").Select("id").Where("tenant_id=? AND owner_user_id=? AND content_hash=? AND deleted_at IS NULL", tenantID, ownerUserID, contentHash).Take(&existingID)
	if errors.Is(res.Error, gorm.ErrRecordNotFound) || res.RowsAffected == 0 {
		return nil, nil
	}
	if res.Error != nil {
		return nil, res.Error
	}
	return &existingID, nil
}

func (r *Repository) ListAssets(ctx context.Context, tenantID, ownerUserID string, limit int, cursorTime *time.Time, cursorID string, includeDeleted bool) ([]d.Asset, bool, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, false, err
	}
	q := db.Table("inspiration_assets").Where("tenant_id=? AND owner_user_id=?", tenantID, ownerUserID)
	if !includeDeleted {
		q = q.Where("deleted_at IS NULL")
	}
	if cursorTime != nil && cursorID != "" {
		q = q.Where("(updated_at < ? OR (updated_at = ? AND id < ?))", *cursorTime, *cursorTime, cursorID)
	}
	var items []d.Asset
	err = q.Order("updated_at DESC, id DESC").Limit(limit + 1).Find(&items).Error
	if err != nil {
		return nil, false, err
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	return items, hasMore, nil
}

func (r *Repository) GetSegments(ctx context.Context, tenantID, assetID string, limit int, cursorSeq int) ([]d.Segment, bool, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, false, err
	}
	q := db.Table("inspiration_segments").Where("tenant_id=? AND asset_id=?", tenantID, assetID)
	if cursorSeq > 0 {
		q = q.Where("seq > ?", cursorSeq)
	}
	var items []d.Segment
	err = q.Order("seq ASC, id ASC").Limit(limit + 1).Find(&items).Error
	if err != nil {
		return nil, false, err
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	return items, hasMore, nil
}

// --- Analysis operations ---

type analysisRow struct {
	ID              string    `gorm:"column:id"`
	TenantID        string    `gorm:"column:tenant_id"`
	CreatedAt       time.Time `gorm:"column:created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at"`
	Revision        int64     `gorm:"column:revision"`
	OwnerUserID     string    `gorm:"column:owner_user_id"`
	NovelID         *string   `gorm:"column:novel_id"`
	AssetID         *string   `gorm:"column:asset_id"`
	AssetRevision   *int64    `gorm:"column:asset_revision"`
	Kind            string    `gorm:"column:kind"`
	Brief           string    `gorm:"column:brief"`
	State           string    `gorm:"column:state"`
	RunID           *string   `gorm:"column:run_id"`
	TotalTokens     int       `gorm:"column:total_tokens"`
	MaxOutputTokens int       `gorm:"column:max_output_tokens"`
	MaxCalls        int       `gorm:"column:max_calls"`
	UsedTokens      int       `gorm:"column:used_tokens"`
	ReservedTokens  int       `gorm:"column:reserved_tokens"`
	CallsUsed       int       `gorm:"column:calls_used"`
	FailureCode     *string   `gorm:"column:failure_code"`
}

func rowToAnalysis(r analysisRow) d.Analysis {
	return d.Analysis{
		ID:            r.ID,
		TenantID:      r.TenantID,
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
		Revision:      r.Revision,
		OwnerUserID:   r.OwnerUserID,
		NovelID:       r.NovelID,
		AssetID:       r.AssetID,
		AssetRevision: r.AssetRevision,
		Kind:          r.Kind,
		Brief:         r.Brief,
		State:         r.State,
		RunID:         r.RunID,
		Budget:        d.Budget{TotalTokens: r.TotalTokens, MaxOutputTokens: r.MaxOutputTokens, MaxCalls: r.MaxCalls},
		Usage:         d.Usage{UsedTokens: r.UsedTokens, ReservedTokens: r.ReservedTokens, CallsUsed: r.CallsUsed},
		FailureCode:   r.FailureCode,
	}
}

func (r *Repository) CreateAnalysis(ctx context.Context, a d.Analysis) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}
	const sql = `INSERT INTO inspiration_analyses (id, tenant_id, created_at, updated_at, revision, owner_user_id, novel_id, asset_id, asset_revision, kind, brief, state, total_tokens, max_output_tokens, max_calls, used_tokens, reserved_tokens, calls_used) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	res := db.Exec(sql, a.ID, a.TenantID, a.CreatedAt, a.UpdatedAt, a.Revision, a.OwnerUserID, a.NovelID, a.AssetID, a.AssetRevision, a.Kind, a.Brief, a.State, a.Budget.TotalTokens, a.Budget.MaxOutputTokens, a.Budget.MaxCalls, a.Usage.UsedTokens, a.Usage.ReservedTokens, a.Usage.CallsUsed)
	return res.Error
}

func (r *Repository) GetAnalysis(ctx context.Context, tenantID, id string) (d.Analysis, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return d.Analysis{}, err
	}
	var row analysisRow
	res := db.Table("inspiration_analyses").Where("tenant_id=? AND id=?", tenantID, id).Take(&row)
	if errors.Is(res.Error, gorm.ErrRecordNotFound) || res.RowsAffected != 1 {
		return d.Analysis{}, pe.Contract(404, "platform.resource_not_found", nil)
	}
	return rowToAnalysis(row), res.Error
}

func (r *Repository) ListAnalyses(ctx context.Context, tenantID, ownerUserID string, novelID *string, state *string, limit int, cursorTime *time.Time, cursorID string) ([]d.Analysis, bool, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, false, err
	}
	q := db.Table("inspiration_analyses").Where("tenant_id=? AND owner_user_id=?", tenantID, ownerUserID)
	if novelID != nil {
		q = q.Where("novel_id=?", *novelID)
	}
	if state != nil {
		q = q.Where("state=?", *state)
	}
	if cursorTime != nil && cursorID != "" {
		q = q.Where("(updated_at < ? OR (updated_at = ? AND id < ?))", *cursorTime, *cursorTime, cursorID)
	}
	var rows []analysisRow
	err = q.Order("updated_at DESC, id DESC").Limit(limit + 1).Find(&rows).Error
	if err != nil {
		return nil, false, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	items := make([]d.Analysis, len(rows))
	for i, row := range rows {
		items[i] = rowToAnalysis(row)
	}
	return items, hasMore, nil
}

func (r *Repository) CountInFlightAnalyses(ctx context.Context, tenantID, ownerUserID string) (int, int, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return 0, 0, err
	}
	var tenantCount int64
	var authorCount int64
	if err := db.Table("inspiration_analyses").Where("tenant_id=? AND state IN ('pending', 'running')", tenantID).Count(&tenantCount).Error; err != nil {
		return 0, 0, err
	}
	if err := db.Table("inspiration_analyses").Where("tenant_id=? AND owner_user_id=? AND state IN ('pending', 'running')", tenantID, ownerUserID).Count(&authorCount).Error; err != nil {
		return 0, 0, err
	}
	return int(tenantCount), int(authorCount), nil
}

func (r *Repository) ClaimAnalysis(ctx context.Context, tenantID, id string, runID, executionKey string, fence int64, expectedRevision int64) (d.Analysis, error) {
	db, err := r.db(ctx, true)
	if err != nil {
		return d.Analysis{}, err
	}
	now := time.Now().UTC()
	res := db.Exec(`UPDATE inspiration_analyses SET state='running', run_id=?, execution_key=?, execution_fence=?, revision=revision+1, updated_at=? WHERE tenant_id=? AND id=? AND revision=? AND (state='pending' OR (state='running' AND execution_key=? AND execution_fence<=?))`,
		runID, executionKey, fence, now, tenantID, id, expectedRevision, executionKey, fence)
	if res.Error != nil {
		return d.Analysis{}, res.Error
	}
	if res.RowsAffected != 1 {
		return d.Analysis{}, pe.Contract(409, "inspiration.execution_conflict", nil)
	}
	return r.GetAnalysis(ctx, tenantID, id)
}

func (r *Repository) CancelAnalysis(ctx context.Context, tenantID, id string, expectedRevision int64, reason string) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	res := db.Exec(`UPDATE inspiration_analyses SET state='cancelled', failure_code=?, revision=revision+1, updated_at=? WHERE tenant_id=? AND id=? AND revision=? AND state IN ('pending', 'running')`,
		reason, now, tenantID, id, expectedRevision)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return pe.Contract(409, "inspiration.invalid_state", nil)
	}
	return nil
}

func (r *Repository) FailAnalysis(ctx context.Context, tenantID, id string, fence int64, failureCode string, usedTokens, callsUsed int) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	res := db.Exec(`UPDATE inspiration_analyses SET state='failed', failure_code=?, used_tokens=used_tokens+?, calls_used=calls_used+?, revision=revision+1, updated_at=? WHERE tenant_id=? AND id=? AND state='running' AND execution_fence<=?`,
		failureCode, usedTokens, callsUsed, now, tenantID, id, fence)
	return res.Error
}

func (r *Repository) CompleteAnalysisWithReport(ctx context.Context, tenantID, id string, fence int64, report d.Report, usedTokens, callsUsed int) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}
	findingsJSON, _ := json.Marshal(report.Findings)
	patternsJSON, _ := json.Marshal(report.Patterns)
	ideasJSON, _ := json.Marshal(report.Ideas)
	var styleMetricsJSON []byte
	if report.StyleMetrics != nil {
		styleMetricsJSON, _ = json.Marshal(report.StyleMetrics)
	}

	const repSQL = `INSERT INTO inspiration_reports (id, tenant_id, created_at, updated_at, revision, analysis_id, summary, findings, patterns, style_metrics, ideas) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	repRes := db.Exec(repSQL, report.ID, report.TenantID, report.CreatedAt, report.UpdatedAt, report.Revision, report.AnalysisID, report.Summary, findingsJSON, patternsJSON, styleMetricsJSON, ideasJSON)
	if repRes.Error != nil {
		return repRes.Error
	}

	now := time.Now().UTC()
	const updSQL = `UPDATE inspiration_analyses SET state='succeeded', used_tokens=used_tokens+?, calls_used=calls_used+?, revision=revision+1, updated_at=? WHERE tenant_id=? AND id=? AND state='running' AND execution_fence<=?`
	updRes := db.Exec(updSQL, usedTokens, callsUsed, now, tenantID, id, fence)
	if updRes.Error != nil {
		return updRes.Error
	}
	if updRes.RowsAffected != 1 {
		return pe.Contract(409, "inspiration.invalid_state", nil)
	}
	return nil
}

func (r *Repository) GetReportByAnalysisID(ctx context.Context, tenantID, analysisID string) (d.Report, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return d.Report{}, err
	}
	var row struct {
		ID           string     `gorm:"column:id"`
		TenantID     string     `gorm:"column:tenant_id"`
		CreatedAt    time.Time  `gorm:"column:created_at"`
		UpdatedAt    time.Time  `gorm:"column:updated_at"`
		Revision     int64      `gorm:"column:revision"`
		AnalysisID   string     `gorm:"column:analysis_id"`
		Summary      string     `gorm:"column:summary"`
		Findings     []byte     `gorm:"column:findings"`
		Patterns     []byte     `gorm:"column:patterns"`
		StyleMetrics []byte     `gorm:"column:style_metrics"`
		Ideas        []byte     `gorm:"column:ideas"`
		RedactedAt   *time.Time `gorm:"column:redacted_at"`
	}
	res := db.Table("inspiration_reports").Where("tenant_id=? AND analysis_id=?", tenantID, analysisID).Take(&row)
	if errors.Is(res.Error, gorm.ErrRecordNotFound) || res.RowsAffected != 1 {
		return d.Report{}, pe.Contract(404, "platform.resource_not_found", nil)
	}
	if res.Error != nil {
		return d.Report{}, res.Error
	}
	rep := d.Report{
		ID:         row.ID,
		TenantID:   row.TenantID,
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
		Revision:   row.Revision,
		AnalysisID: row.AnalysisID,
		Summary:    row.Summary,
		RedactedAt: row.RedactedAt,
		Findings:   []d.Finding{},
		Patterns:   []d.Pattern{},
		Ideas:      []d.Idea{},
	}
	if len(row.Findings) > 0 {
		_ = json.Unmarshal(row.Findings, &rep.Findings)
	}
	if len(row.Patterns) > 0 {
		_ = json.Unmarshal(row.Patterns, &rep.Patterns)
	}
	if len(row.Ideas) > 0 {
		_ = json.Unmarshal(row.Ideas, &rep.Ideas)
	}
	if len(row.StyleMetrics) > 0 && string(row.StyleMetrics) != "null" {
		var sm d.StyleMetrics
		if json.Unmarshal(row.StyleMetrics, &sm) == nil {
			rep.StyleMetrics = &sm
		}
	}
	return rep, nil
}

// --- Style operations ---

func (r *Repository) CreateStyleWithVersion(ctx context.Context, s d.Style, v d.StyleVersion) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}
	const styleSQL = `INSERT INTO inspiration_styles (id, tenant_id, created_at, updated_at, revision, owner_user_id, title, current_version_id, state) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	if err := db.Exec(styleSQL, s.ID, s.TenantID, s.CreatedAt, s.UpdatedAt, s.Revision, s.OwnerUserID, s.Title, s.CurrentVersionID, s.State).Error; err != nil {
		return err
	}
	metricsJSON, _ := json.Marshal(v.Metrics)
	const verSQL = `INSERT INTO inspiration_style_versions (id, tenant_id, created_at, updated_at, revision, style_id, version_no, instructions, metrics, source_report_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	return db.Exec(verSQL, v.ID, v.TenantID, v.CreatedAt, v.UpdatedAt, v.Revision, v.StyleID, v.VersionNo, v.Instructions, metricsJSON, v.SourceReportID).Error
}

func (r *Repository) AddStyleVersion(ctx context.Context, styleID string, expectedRevision int64, v d.StyleVersion, title *string) (d.Style, d.StyleVersion, error) {
	db, err := r.db(ctx, true)
	if err != nil {
		return d.Style{}, d.StyleVersion{}, err
	}
	var maxVer int
	_ = db.Raw(`SELECT COALESCE(MAX(version_no), 0) FROM inspiration_style_versions WHERE tenant_id=? AND style_id=?`, v.TenantID, styleID).Scan(&maxVer).Error
	v.VersionNo = maxVer + 1

	metricsJSON, _ := json.Marshal(v.Metrics)
	const verSQL = `INSERT INTO inspiration_style_versions (id, tenant_id, created_at, updated_at, revision, style_id, version_no, instructions, metrics, source_report_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	if err := db.Exec(verSQL, v.ID, v.TenantID, v.CreatedAt, v.UpdatedAt, v.Revision, v.StyleID, v.VersionNo, v.Instructions, metricsJSON, v.SourceReportID).Error; err != nil {
		return d.Style{}, d.StyleVersion{}, err
	}

	now := time.Now().UTC()
	upd := db.Exec(`UPDATE inspiration_styles SET current_version_id=?, revision=revision+1, updated_at=?, title=COALESCE(?, title) WHERE tenant_id=? AND id=? AND revision=? AND state!='deleted'`,
		v.ID, now, title, v.TenantID, styleID, expectedRevision)
	if upd.Error != nil {
		return d.Style{}, d.StyleVersion{}, upd.Error
	}
	if upd.RowsAffected != 1 {
		return d.Style{}, d.StyleVersion{}, pe.Contract(409, "inspiration.version_conflict", nil)
	}
	style, err := r.GetStyle(ctx, v.TenantID, styleID, false)
	return style, v, err
}

func (r *Repository) SoftDeleteStyle(ctx context.Context, tenantID, id string, expectedRevision int64, now time.Time) (int64, error) {
	db, err := r.db(ctx, true)
	if err != nil {
		return 0, err
	}
	var newRev int64
	row := db.Raw(`UPDATE inspiration_styles SET state='deleted', deleted_at=?, revision=revision+1, updated_at=? WHERE tenant_id=? AND id=? AND revision=? AND state='active' RETURNING revision`, now, now, tenantID, id, expectedRevision).Row()
	if err := row.Scan(&newRev); err != nil {
		return 0, pe.Contract(409, "inspiration.version_conflict", nil)
	}
	return newRev, nil
}

func (r *Repository) RestoreStyle(ctx context.Context, tenantID, id string, expectedRevision int64, now time.Time) (int64, error) {
	db, err := r.db(ctx, true)
	if err != nil {
		return 0, err
	}
	var newRev int64
	row := db.Raw(`UPDATE inspiration_styles SET state='active', deleted_at=NULL, revision=revision+1, updated_at=? WHERE tenant_id=? AND id=? AND revision=? AND state='deleted' RETURNING revision`, now, tenantID, id, expectedRevision).Row()
	if err := row.Scan(&newRev); err != nil {
		return 0, pe.Contract(409, "inspiration.version_conflict", nil)
	}
	return newRev, nil
}

func (r *Repository) GetStyle(ctx context.Context, tenantID, id string, includeDeleted bool) (d.Style, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return d.Style{}, err
	}
	var out d.Style
	where := "tenant_id=? AND id=?"
	if !includeDeleted {
		where += " AND deleted_at IS NULL"
	}
	res := db.Table("inspiration_styles").Where(where, tenantID, id).Take(&out)
	if errors.Is(res.Error, gorm.ErrRecordNotFound) || res.RowsAffected != 1 {
		return out, pe.Contract(404, "platform.resource_not_found", nil)
	}
	return out, res.Error
}

func (r *Repository) ListStyles(ctx context.Context, tenantID, ownerUserID string, limit int, cursorTime *time.Time, cursorID string, includeDeleted bool) ([]d.Style, bool, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, false, err
	}
	q := db.Table("inspiration_styles").Where("tenant_id=? AND owner_user_id=?", tenantID, ownerUserID)
	if !includeDeleted {
		q = q.Where("deleted_at IS NULL")
	}
	if cursorTime != nil && cursorID != "" {
		q = q.Where("(updated_at < ? OR (updated_at = ? AND id < ?))", *cursorTime, *cursorTime, cursorID)
	}
	var items []d.Style
	err = q.Order("updated_at DESC, id DESC").Limit(limit + 1).Find(&items).Error
	if err != nil {
		return nil, false, err
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	return items, hasMore, nil
}

type styleVersionRow struct {
	ID             string     `gorm:"column:id"`
	TenantID       string     `gorm:"column:tenant_id"`
	CreatedAt      time.Time  `gorm:"column:created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at"`
	Revision       int64      `gorm:"column:revision"`
	StyleID        string     `gorm:"column:style_id"`
	VersionNo      int        `gorm:"column:version_no"`
	Instructions   string     `gorm:"column:instructions"`
	Metrics        []byte     `gorm:"column:metrics"`
	SourceReportID *string    `gorm:"column:source_report_id"`
	RedactedAt     *time.Time `gorm:"column:redacted_at"`
}

func (r *Repository) ListStyleVersions(ctx context.Context, tenantID, styleID string, limit int, cursorTime *time.Time, cursorID string) ([]d.StyleVersion, bool, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, false, err
	}
	q := db.Table("inspiration_style_versions").Where("tenant_id=? AND style_id=?", tenantID, styleID)
	if cursorTime != nil && cursorID != "" {
		q = q.Where("(created_at < ? OR (created_at = ? AND id < ?))", *cursorTime, *cursorTime, cursorID)
	}
	var rows []styleVersionRow
	err = q.Order("created_at DESC, id DESC").Limit(limit + 1).Find(&rows).Error
	if err != nil {
		return nil, false, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	items := make([]d.StyleVersion, len(rows))
	for i, row := range rows {
		items[i] = d.StyleVersion{
			ID:             row.ID,
			TenantID:       row.TenantID,
			CreatedAt:      row.CreatedAt,
			UpdatedAt:      row.UpdatedAt,
			Revision:       row.Revision,
			StyleID:        row.StyleID,
			VersionNo:      row.VersionNo,
			Instructions:   row.Instructions,
			SourceReportID: row.SourceReportID,
			RedactedAt:     row.RedactedAt,
		}
		if len(row.Metrics) > 0 {
			_ = json.Unmarshal(row.Metrics, &items[i].Metrics)
		}
	}
	return items, hasMore, nil
}

func (r *Repository) GetStyleVersion(ctx context.Context, tenantID, styleID, versionID string) (d.StyleVersion, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return d.StyleVersion{}, err
	}
	var row styleVersionRow
	res := db.Table("inspiration_style_versions").Where("tenant_id=? AND style_id=? AND id=?", tenantID, styleID, versionID).Take(&row)
	if errors.Is(res.Error, gorm.ErrRecordNotFound) || res.RowsAffected != 1 {
		return d.StyleVersion{}, pe.Contract(404, "platform.resource_not_found", nil)
	}
	if res.Error != nil {
		return d.StyleVersion{}, res.Error
	}
	out := d.StyleVersion{
		ID:             row.ID,
		TenantID:       row.TenantID,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		Revision:       row.Revision,
		StyleID:        row.StyleID,
		VersionNo:      row.VersionNo,
		Instructions:   row.Instructions,
		SourceReportID: row.SourceReportID,
		RedactedAt:     row.RedactedAt,
	}
	if len(row.Metrics) > 0 {
		_ = json.Unmarshal(row.Metrics, &out.Metrics)
	}
	return out, nil
}

// --- Card operations ---

type cardRow struct {
	ID             string     `gorm:"column:id"`
	TenantID       string     `gorm:"column:tenant_id"`
	CreatedAt      time.Time  `gorm:"column:created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at"`
	Revision       int64      `gorm:"column:revision"`
	OwnerUserID    string     `gorm:"column:owner_user_id"`
	NovelID        *string    `gorm:"column:novel_id"`
	Title          string     `gorm:"column:title"`
	Category       string     `gorm:"column:category"`
	Body           string     `gorm:"column:body"`
	Rationale      string     `gorm:"column:rationale"`
	SourceKind     string     `gorm:"column:source_kind"`
	SourceReportID *string    `gorm:"column:source_report_id"`
	Evidence       []byte     `gorm:"column:evidence"`
	Fingerprint    string     `gorm:"column:fingerprint"`
	State          string     `gorm:"column:state"`
	DeletedAt      *time.Time `gorm:"column:deleted_at"`
	RedactedAt     *time.Time `gorm:"column:redacted_at"`
}

func rowToCard(row cardRow) d.Card {
	c := d.Card{
		ID:             row.ID,
		TenantID:       row.TenantID,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		Revision:       row.Revision,
		OwnerUserID:    row.OwnerUserID,
		NovelID:        row.NovelID,
		Title:          row.Title,
		Category:       row.Category,
		Body:           row.Body,
		Rationale:      row.Rationale,
		SourceKind:     row.SourceKind,
		SourceReportID: row.SourceReportID,
		Fingerprint:    row.Fingerprint,
		State:          row.State,
		DeletedAt:      row.DeletedAt,
		RedactedAt:     row.RedactedAt,
		Evidence:       []d.Evidence{},
	}
	if len(row.Evidence) > 0 {
		_ = json.Unmarshal(row.Evidence, &c.Evidence)
	}
	return c
}

func (r *Repository) CreateCard(ctx context.Context, c d.Card) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}
	evidenceJSON, _ := json.Marshal(c.Evidence)
	const sql = `INSERT INTO inspiration_cards (id, tenant_id, created_at, updated_at, revision, owner_user_id, novel_id, title, category, body, rationale, source_kind, source_report_id, evidence, fingerprint, state) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	res := db.Exec(sql, c.ID, c.TenantID, c.CreatedAt, c.UpdatedAt, c.Revision, c.OwnerUserID, c.NovelID, c.Title, c.Category, c.Body, c.Rationale, c.SourceKind, c.SourceReportID, evidenceJSON, c.Fingerprint, c.State)
	return res.Error
}

func (r *Repository) UpdateCard(ctx context.Context, c d.Card, expectedRevision int64) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}
	evidenceJSON, _ := json.Marshal(c.Evidence)
	res := db.Exec(`UPDATE inspiration_cards SET novel_id=?, title=?, category=?, body=?, rationale=?, source_kind=?, source_report_id=?, evidence=?, fingerprint=?, revision=?, updated_at=? WHERE tenant_id=? AND id=? AND revision=? AND state!='deleted'`,
		c.NovelID, c.Title, c.Category, c.Body, c.Rationale, c.SourceKind, c.SourceReportID, evidenceJSON, c.Fingerprint, c.Revision, c.UpdatedAt, c.TenantID, c.ID, expectedRevision)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return pe.Contract(409, "inspiration.version_conflict", nil)
	}
	return nil
}

func (r *Repository) UpdateCardState(ctx context.Context, tenantID, id string, state string, expectedRevision int64) (d.Card, error) {
	db, err := r.db(ctx, true)
	if err != nil {
		return d.Card{}, err
	}
	now := time.Now().UTC()
	res := db.Exec(`UPDATE inspiration_cards SET state=?, revision=revision+1, updated_at=? WHERE tenant_id=? AND id=? AND revision=? AND state!='deleted'`,
		state, now, tenantID, id, expectedRevision)
	if res.Error != nil {
		return d.Card{}, res.Error
	}
	if res.RowsAffected != 1 {
		return d.Card{}, pe.Contract(409, "inspiration.version_conflict", nil)
	}
	return r.GetCard(ctx, tenantID, id, false)
}

func (r *Repository) SoftDeleteCard(ctx context.Context, tenantID, id string, expectedRevision int64, now time.Time) (int64, error) {
	db, err := r.db(ctx, true)
	if err != nil {
		return 0, err
	}
	var newRev int64
	row := db.Raw(`UPDATE inspiration_cards SET state='deleted', deleted_at=?, revision=revision+1, updated_at=? WHERE tenant_id=? AND id=? AND revision=? AND state!='deleted' RETURNING revision`, now, now, tenantID, id, expectedRevision).Row()
	if err := row.Scan(&newRev); err != nil {
		return 0, pe.Contract(409, "inspiration.version_conflict", nil)
	}
	return newRev, nil
}

func (r *Repository) RestoreCard(ctx context.Context, tenantID, id string, expectedRevision int64, now time.Time) (int64, error) {
	db, err := r.db(ctx, true)
	if err != nil {
		return 0, err
	}
	var newRev int64
	row := db.Raw(`UPDATE inspiration_cards SET state='suggested', deleted_at=NULL, revision=revision+1, updated_at=? WHERE tenant_id=? AND id=? AND revision=? AND state='deleted' RETURNING revision`, now, tenantID, id, expectedRevision).Row()
	if err := row.Scan(&newRev); err != nil {
		return 0, pe.Contract(409, "inspiration.version_conflict", nil)
	}
	return newRev, nil
}

func (r *Repository) GetCard(ctx context.Context, tenantID, id string, includeDeleted bool) (d.Card, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return d.Card{}, err
	}
	var row cardRow
	where := "tenant_id=? AND id=?"
	if !includeDeleted {
		where += " AND deleted_at IS NULL"
	}
	res := db.Table("inspiration_cards").Where(where, tenantID, id).Take(&row)
	if errors.Is(res.Error, gorm.ErrRecordNotFound) || res.RowsAffected != 1 {
		return d.Card{}, pe.Contract(404, "platform.resource_not_found", nil)
	}
	return rowToCard(row), res.Error
}

func (r *Repository) CheckCardDuplicate(ctx context.Context, tenantID, ownerUserID string, novelID *string, fingerprint string, excludeID string) (*string, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, err
	}
	q := db.Table("inspiration_cards").Select("id").Where("tenant_id=? AND owner_user_id=? AND fingerprint=? AND deleted_at IS NULL", tenantID, ownerUserID, fingerprint)
	if novelID == nil {
		q = q.Where("novel_id IS NULL")
	} else {
		q = q.Where("novel_id=?", *novelID)
	}
	if excludeID != "" {
		q = q.Where("id!=?", excludeID)
	}
	var existingID string
	res := q.Take(&existingID)
	if errors.Is(res.Error, gorm.ErrRecordNotFound) || res.RowsAffected == 0 {
		return nil, nil
	}
	if res.Error != nil {
		return nil, res.Error
	}
	return &existingID, nil
}

func (r *Repository) ListCards(ctx context.Context, tenantID, ownerUserID string, novelID *string, category *string, state *string, limit int, cursorTime *time.Time, cursorID string, includeDeleted bool) ([]d.Card, bool, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, false, err
	}
	q := db.Table("inspiration_cards").Where("tenant_id=? AND owner_user_id=?", tenantID, ownerUserID)
	if !includeDeleted {
		q = q.Where("deleted_at IS NULL")
	}
	if novelID != nil {
		q = q.Where("novel_id=?", *novelID)
	}
	if category != nil {
		q = q.Where("category=?", *category)
	}
	if state != nil {
		q = q.Where("state=?", *state)
	}
	if cursorTime != nil && cursorID != "" {
		q = q.Where("(updated_at < ? OR (updated_at = ? AND id < ?))", *cursorTime, *cursorTime, cursorID)
	}
	var rows []cardRow
	err = q.Order("updated_at DESC, id DESC").Limit(limit + 1).Find(&rows).Error
	if err != nil {
		return nil, false, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	items := make([]d.Card, len(rows))
	for i, row := range rows {
		items[i] = rowToCard(row)
	}
	return items, hasMore, nil
}

// --- Adoption operations ---

func (r *Repository) CreateAdoption(ctx context.Context, a d.Adoption, cardExpectedRevision int64) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	cardUpd := db.Exec(`UPDATE inspiration_cards SET revision=revision+1, updated_at=? WHERE tenant_id=? AND id=? AND revision=? AND state!='deleted'`, now, a.TenantID, a.CardID, cardExpectedRevision)
	if cardUpd.Error != nil {
		return cardUpd.Error
	}
	if cardUpd.RowsAffected != 1 {
		return pe.Contract(409, "inspiration.version_conflict", nil)
	}
	const sql = `INSERT INTO inspiration_adoptions (id, tenant_id, created_at, updated_at, revision, card_id, actor_user_id, target_domain, target_id, target_revision, target_hash, state) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	res := db.Exec(sql, a.ID, a.TenantID, a.CreatedAt, a.UpdatedAt, a.Revision, a.CardID, a.ActorUserID, a.TargetDomain, a.TargetID, a.TargetRevision, a.TargetHash, a.State)
	return res.Error
}

func (r *Repository) RevokeAdoption(ctx context.Context, tenantID, id string, expectedRevision int64, reason string, now time.Time) (d.Adoption, error) {
	db, err := r.db(ctx, true)
	if err != nil {
		return d.Adoption{}, err
	}
	res := db.Exec(`UPDATE inspiration_adoptions SET state='revoked', revoke_reason=?, revoked_at=?, revision=revision+1, updated_at=? WHERE tenant_id=? AND id=? AND revision=? AND state='active'`,
		reason, now, now, tenantID, id, expectedRevision)
	if res.Error != nil {
		return d.Adoption{}, res.Error
	}
	if res.RowsAffected != 1 {
		return d.Adoption{}, pe.Contract(409, "inspiration.version_conflict", nil)
	}
	return r.GetAdoption(ctx, tenantID, id)
}

func (r *Repository) GetAdoption(ctx context.Context, tenantID, id string) (d.Adoption, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return d.Adoption{}, err
	}
	var out d.Adoption
	res := db.Table("inspiration_adoptions").Where("tenant_id=? AND id=?", tenantID, id).Take(&out)
	if errors.Is(res.Error, gorm.ErrRecordNotFound) || res.RowsAffected != 1 {
		return out, pe.Contract(404, "platform.resource_not_found", nil)
	}
	return out, res.Error
}

func (r *Repository) ListAdoptions(ctx context.Context, tenantID, cardID string, limit int, cursorTime *time.Time, cursorID string) ([]d.Adoption, bool, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, false, err
	}
	q := db.Table("inspiration_adoptions").Where("tenant_id=? AND card_id=?", tenantID, cardID)
	if cursorTime != nil && cursorID != "" {
		q = q.Where("(created_at < ? OR (created_at = ? AND id < ?))", *cursorTime, *cursorTime, cursorID)
	}
	var items []d.Adoption
	err = q.Order("created_at DESC, id DESC").Limit(limit + 1).Find(&items).Error
	if err != nil {
		return nil, false, err
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	return items, hasMore, nil
}

// --- Retention operations ---

func (r *Repository) PurgeSoftDeletedBatch(ctx context.Context, limit int, cutoff time.Time) (int, error) {
	db, err := r.db(ctx, true)
	if err != nil {
		return 0, err
	}
	now := time.Now().UTC()
	var total int

	// 1. Redact assets deleted > 30 days ago
	var assetIDs []string
	_ = db.Table("inspiration_assets").Select("id").Where("deleted_at <= ? AND redacted_at IS NULL", cutoff).Limit(limit).Find(&assetIDs).Error
	if len(assetIDs) > 0 {
		_ = db.Exec(`UPDATE inspiration_assets SET redacted_at=?, revision=revision+1, updated_at=? WHERE id IN (?)`, now, now, assetIDs)
		total += len(assetIDs)
	}

	// 2. Redact cards deleted > 30 days ago
	var cardIDs []string
	_ = db.Table("inspiration_cards").Select("id").Where("deleted_at <= ? AND redacted_at IS NULL", cutoff).Limit(limit).Find(&cardIDs).Error
	if len(cardIDs) > 0 {
		_ = db.Exec(`UPDATE inspiration_cards SET redacted_at=?, body='', rationale='', evidence='[]', revision=revision+1, updated_at=? WHERE id IN (?)`, now, now, cardIDs)
		total += len(cardIDs)
	}

	// 3. Redact styles deleted > 30 days ago
	var styleIDs []string
	_ = db.Table("inspiration_styles").Select("id").Where("deleted_at <= ? AND redacted_at IS NULL", cutoff).Limit(limit).Find(&styleIDs).Error
	if len(styleIDs) > 0 {
		_ = db.Exec(`UPDATE inspiration_style_versions SET redacted_at=?, instructions='', revision=revision+1, updated_at=? WHERE style_id IN (?) AND redacted_at IS NULL`, now, now, styleIDs)
		_ = db.Exec(`UPDATE inspiration_styles SET redacted_at=?, revision=revision+1, updated_at=? WHERE id IN (?)`, now, now, styleIDs)
		total += len(styleIDs)
	}

	return total, nil
}
