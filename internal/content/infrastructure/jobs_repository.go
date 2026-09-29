package infrastructure

import (
	a "InkLore/internal/content/application"
	d "InkLore/internal/content/domain"
	"context"
	"database/sql/driver"
	"fmt"
	"strings"
	"time"
)

type StringArray []string

func (a StringArray) Value() (driver.Value, error) {
	if len(a) == 0 {
		return "{}", nil
	}
	var b strings.Builder
	b.WriteByte('{')
	for i, s := range a {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s)
	}
	b.WriteByte('}')
	return b.String(), nil
}

func (a *StringArray) Scan(src any) error {
	switch v := src.(type) {
	case []byte:
		*a = parsePgArray(string(v))
		return nil
	case string:
		*a = parsePgArray(v)
		return nil
	case nil:
		*a = []string{}
		return nil
	default:
		return fmt.Errorf("cannot scan %T into StringArray", src)
	}
}

func parsePgArray(src string) []string {
	src = strings.Trim(src, "{}")
	if src == "" {
		return []string{}
	}
	parts := strings.Split(src, ",")
	out := make([]string, len(parts))
	for i, p := range parts {
		out[i] = strings.Trim(p, "\"")
	}
	return out
}

type jobModel struct {
	ID                         string      `gorm:"primaryKey;column:id"`
	TenantID                   string      `gorm:"column:tenant_id"`
	CreatedAt                  time.Time   `gorm:"column:created_at"`
	UpdatedAt                  time.Time   `gorm:"column:updated_at"`
	Revision                   int64       `gorm:"column:revision"`
	NovelID                    string      `gorm:"column:novel_id"`
	ActorID                    string      `gorm:"column:actor_id"`
	Kind                       string      `gorm:"column:kind"`
	Status                     string      `gorm:"column:status"`
	Stage                      string      `gorm:"column:stage"`
	Progress                   int         `gorm:"column:progress"`
	SourceFileID               *string     `gorm:"column:source_file_id"`
	OutputFileID               *string     `gorm:"column:output_file_id"`
	TargetNovelID              *string     `gorm:"column:target_novel_id"`
	Format                     string      `gorm:"column:format"`
	SourceRevision             int64       `gorm:"column:source_revision"`
	LeaseUntil                 *time.Time  `gorm:"column:lease_until"`
	LeaseToken                 *string     `gorm:"column:lease_token"`
	Attempts                   int         `gorm:"column:attempts"`
	AvailableAt                time.Time   `gorm:"column:available_at"`
	ExpiresAt                  time.Time   `gorm:"column:expires_at"`
	ErrorCode                  *string     `gorm:"column:error_code"`
	ResultChapterCount         int         `gorm:"column:result_chapter_count"`
	CopyTitle                  *string     `gorm:"column:copy_title"`
	IncludePlanning            bool        `gorm:"column:include_planning"`
	IncludeWorldview           bool        `gorm:"column:include_worldview"`
	IncludeDrafts              bool        `gorm:"column:include_drafts"`
	VolumeIDs                  StringArray `gorm:"column:volume_ids;type:uuid[]"`
	CommitVolumeID             *string     `gorm:"column:commit_volume_id"`
	CommitItemIDs              StringArray `gorm:"column:commit_item_ids;type:uuid[]"`
	ReviewTicket               *string     `gorm:"column:review_ticket"`
	ExpectedNovelRevision      *int64      `gorm:"column:expected_novel_revision"`
	PlanningSnapshotFileID     *string     `gorm:"column:planning_snapshot_file_id"`
	WorldviewSnapshotFileID    *string     `gorm:"column:worldview_snapshot_file_id"`
	OrganizationSnapshotFileID *string     `gorm:"column:organization_snapshot_file_id"`
}

func (jobModel) TableName() string { return "content_jobs" }

func toJobRecord(m jobModel) a.JobRecord {
	return a.JobRecord{
		Job: d.Job{
			ID:                 m.ID,
			TenantID:           m.TenantID,
			CreatedAt:          m.CreatedAt,
			UpdatedAt:          m.UpdatedAt,
			Revision:           m.Revision,
			NovelID:            m.NovelID,
			ActorID:            m.ActorID,
			Kind:               m.Kind,
			Status:             m.Status,
			Stage:              m.Stage,
			Progress:           m.Progress,
			SourceFileID:       m.SourceFileID,
			OutputFileID:       m.OutputFileID,
			TargetNovelID:      m.TargetNovelID,
			Format:             m.Format,
			SourceRevision:     m.SourceRevision,
			LeaseUntil:         m.LeaseUntil,
			Attempts:           m.Attempts,
			ExpiresAt:          m.ExpiresAt,
			ErrorCode:          m.ErrorCode,
			ResultChapterCount: m.ResultChapterCount,
			CopyTitle:          m.CopyTitle,
			ReviewTicket:       m.ReviewTicket,
		},
		AvailableAt:                m.AvailableAt,
		LeaseToken:                 m.LeaseToken,
		IncludePlanning:            m.IncludePlanning,
		IncludeWorldview:           m.IncludeWorldview,
		IncludeDrafts:              m.IncludeDrafts,
		VolumeIDs:                  m.VolumeIDs,
		CommitVolumeID:             m.CommitVolumeID,
		CommitItemIDs:              m.CommitItemIDs,
		ExpectedNovelRevision:      m.ExpectedNovelRevision,
		PlanningSnapshotFileID:     m.PlanningSnapshotFileID,
		WorldviewSnapshotFileID:    m.WorldviewSnapshotFileID,
		OrganizationSnapshotFileID: m.OrganizationSnapshotFileID,
	}
}

func toJobModel(j a.JobRecord) jobModel {
	return jobModel{
		ID:                         j.ID,
		TenantID:                   j.TenantID,
		CreatedAt:                  j.CreatedAt,
		UpdatedAt:                  j.UpdatedAt,
		Revision:                   j.Revision,
		NovelID:                    j.NovelID,
		ActorID:                    j.ActorID,
		Kind:                       j.Kind,
		Status:                     j.Status,
		Stage:                      j.Stage,
		Progress:                   j.Progress,
		SourceFileID:               j.SourceFileID,
		OutputFileID:               j.OutputFileID,
		TargetNovelID:              j.TargetNovelID,
		Format:                     j.Format,
		SourceRevision:             j.SourceRevision,
		LeaseUntil:                 j.LeaseUntil,
		LeaseToken:                 j.LeaseToken,
		Attempts:                   j.Attempts,
		AvailableAt:                j.AvailableAt,
		ExpiresAt:                  j.ExpiresAt,
		ErrorCode:                  j.ErrorCode,
		ResultChapterCount:         j.ResultChapterCount,
		CopyTitle:                  j.CopyTitle,
		IncludePlanning:            j.IncludePlanning,
		IncludeWorldview:           j.IncludeWorldview,
		IncludeDrafts:              j.IncludeDrafts,
		VolumeIDs:                  j.VolumeIDs,
		CommitVolumeID:             j.CommitVolumeID,
		CommitItemIDs:              j.CommitItemIDs,
		ReviewTicket:               j.ReviewTicket,
		ExpectedNovelRevision:      j.ExpectedNovelRevision,
		PlanningSnapshotFileID:     j.PlanningSnapshotFileID,
		WorldviewSnapshotFileID:    j.WorldviewSnapshotFileID,
		OrganizationSnapshotFileID: j.OrganizationSnapshotFileID,
	}
}

func (r *Repository) InsertJob(ctx context.Context, o a.Owner, j a.JobRecord) error {
	if j.TenantID != o.TenantID || j.ActorID != o.ActorID || j.NovelID != o.NovelID {
		return d.Missing()
	}
	db, e := r.db(ctx, true)
	if e != nil {
		return e
	}
	m := toJobModel(j)
	return db.Table("content_jobs").Create(&m).Error
}

func (r *Repository) PutJob(ctx context.Context, o a.Owner, j a.JobRecord, expected int64) error {
	db, e := r.db(ctx, true)
	if e != nil {
		return e
	}
	m := toJobModel(j)
	q := db.Table("content_jobs").Where("id=? AND revision=? AND tenant_id=? AND novel_id=?", j.ID, expected, o.TenantID, o.NovelID).Updates(map[string]any{
		"status":                        m.Status,
		"stage":                         m.Stage,
		"progress":                      m.Progress,
		"source_file_id":                m.SourceFileID,
		"output_file_id":                m.OutputFileID,
		"target_novel_id":               m.TargetNovelID,
		"lease_until":                   m.LeaseUntil,
		"lease_token":                   m.LeaseToken,
		"attempts":                      m.Attempts,
		"available_at":                  m.AvailableAt,
		"expires_at":                    m.ExpiresAt,
		"error_code":                    m.ErrorCode,
		"result_chapter_count":          m.ResultChapterCount,
		"commit_volume_id":              m.CommitVolumeID,
		"commit_item_ids":               m.CommitItemIDs,
		"expected_novel_revision":       m.ExpectedNovelRevision,
		"planning_snapshot_file_id":     m.PlanningSnapshotFileID,
		"worldview_snapshot_file_id":    m.WorldviewSnapshotFileID,
		"organization_snapshot_file_id": m.OrganizationSnapshotFileID,
		"revision":                      m.Revision,
		"updated_at":                    m.UpdatedAt,
	})
	if q.Error != nil {
		return q.Error
	}
	if q.RowsAffected != 1 {
		return &d.RuleError{Code: "content.version_conflict", CurrentRevision: expected}
	}
	return nil
}

func (r *Repository) Job(ctx context.Context, o a.Owner, id string, exclusive bool) (a.JobRecord, error) {
	db, e := r.db(ctx, exclusive)
	if e != nil {
		return a.JobRecord{}, e
	}
	var m jobModel
	q := db.Table("content_jobs").Where("id=? AND tenant_id=? AND actor_id=?", id, o.TenantID, o.ActorID)
	if exclusive {
		q = lock(q, true)
	}
	if err := q.First(&m).Error; err != nil {
		return a.JobRecord{}, d.Missing()
	}
	return toJobRecord(m), nil
}

const jobPublicColumns = "id,tenant_id,created_at,updated_at,revision,novel_id,actor_id,kind,status,stage,progress,source_file_id,output_file_id,target_novel_id,format,source_revision,lease_until,attempts,expires_at,error_code,result_chapter_count,copy_title,review_ticket"

func (r *Repository) ListJobs(ctx context.Context, o a.Owner, p a.Query) ([]d.Job, error) {
	db, e := r.db(ctx, false)
	if e != nil {
		return nil, e
	}
	q := db.Table("content_jobs").Select(jobPublicColumns).Where("tenant_id=? AND actor_id=? AND novel_id=?", o.TenantID, o.ActorID, o.NovelID)
	return many[d.Job](page(q, p, "updated_at"))
}

func (r *Repository) CountRunningJobs(ctx context.Context, tenantID string) (int64, error) {
	db, e := r.db(ctx, false)
	if e != nil {
		return 0, e
	}
	var count int64
	e = db.Table("content_jobs").Where("tenant_id=? AND status IN ('queued','running','committing')", tenantID).Count(&count).Error
	return count, e
}

func (r *Repository) CountActorNonTerminalJobs(ctx context.Context, tenantID, actorID string) (int64, error) {
	db, e := r.db(ctx, false)
	if e != nil {
		return 0, e
	}
	var count int64
	e = db.Table("content_jobs").Where("tenant_id=? AND actor_id=? AND status IN ('queued','running','preview_ready','committing','cancelling')", tenantID, actorID).Count(&count).Error
	return count, e
}

type importItemModel struct {
	ID          string      `gorm:"primaryKey;column:id"`
	TenantID    string      `gorm:"column:tenant_id"`
	CreatedAt   time.Time   `gorm:"column:created_at"`
	UpdatedAt   time.Time   `gorm:"column:updated_at"`
	Revision    int64       `gorm:"column:revision"`
	JobID       string      `gorm:"column:job_id"`
	Ordinal     int         `gorm:"column:ordinal"`
	Title       string      `gorm:"column:title"`
	Body        string      `gorm:"column:body"`
	WordCount   int64       `gorm:"column:word_count"`
	ContentHash string      `gorm:"column:content_hash"`
	Selected    bool        `gorm:"column:selected"`
	Warnings    StringArray `gorm:"column:warnings;type:text[]"`
}

func (importItemModel) TableName() string { return "content_import_items" }

func toImportItem(m importItemModel) d.ImportItem {
	return d.ImportItem{
		ID:          m.ID,
		TenantID:    m.TenantID,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
		Revision:    m.Revision,
		JobID:       m.JobID,
		Ordinal:     m.Ordinal,
		Title:       m.Title,
		Body:        m.Body,
		WordCount:   m.WordCount,
		ContentHash: m.ContentHash,
		Selected:    m.Selected,
		Warnings:    m.Warnings,
	}
}

func (r *Repository) InsertImportItems(ctx context.Context, o a.Owner, items []d.ImportItem) error {
	if len(items) == 0 {
		return nil
	}
	db, e := r.db(ctx, true)
	if e != nil {
		return e
	}
	models := make([]importItemModel, len(items))
	for i, item := range items {
		models[i] = importItemModel{
			ID:          item.ID,
			TenantID:    o.TenantID,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
			Revision:    item.Revision,
			JobID:       item.JobID,
			Ordinal:     item.Ordinal,
			Title:       item.Title,
			Body:        item.Body,
			WordCount:   item.WordCount,
			ContentHash: item.ContentHash,
			Selected:    item.Selected,
			Warnings:    item.Warnings,
		}
	}
	return db.Table("content_import_items").CreateInBatches(models, 100).Error
}

func (r *Repository) ListImportItems(ctx context.Context, o a.Owner, jobID string, p a.Query) ([]d.ImportItem, error) {
	db, e := r.db(ctx, false)
	if e != nil {
		return nil, e
	}
	var models []importItemModel
	q := db.Table("content_import_items").Where("tenant_id=? AND job_id=?", o.TenantID, jobID)
	if p.After != nil {
		q = q.Where("(updated_at,id) < (?,?::uuid)", p.After.Time, p.After.ID)
	}
	err := q.Order("updated_at DESC,id DESC").Limit(p.Limit + 1).Find(&models).Error
	if err != nil {
		return nil, err
	}
	out := make([]d.ImportItem, len(models))
	for i, m := range models {
		out[i] = toImportItem(m)
	}
	return out, nil
}

func (r *Repository) GetImportItemsByIDs(ctx context.Context, o a.Owner, jobID string, itemIDs []string) ([]d.ImportItem, error) {
	db, e := r.db(ctx, false)
	if e != nil {
		return nil, e
	}
	var models []importItemModel
	err := db.Table("content_import_items").Where("tenant_id=? AND job_id=? AND id IN (?)", o.TenantID, jobID, itemIDs).Order("ordinal ASC").Find(&models).Error
	if err != nil {
		return nil, err
	}
	out := make([]d.ImportItem, len(models))
	for i, m := range models {
		out[i] = toImportItem(m)
	}
	return out, nil
}

func (r *Repository) MarkImportItemsSelected(ctx context.Context, o a.Owner, jobID string, itemIDs []string) error {
	db, e := r.db(ctx, true)
	if e != nil {
		return e
	}
	return db.Table("content_import_items").Where("tenant_id=? AND job_id=? AND id IN (?)", o.TenantID, jobID, itemIDs).Updates(map[string]any{
		"selected":   true,
		"updated_at": time.Now().UTC(),
	}).Error
}

type snapshotModel struct {
	ID              string    `gorm:"primaryKey;column:id"`
	TenantID        string    `gorm:"column:tenant_id"`
	CreatedAt       time.Time `gorm:"column:created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at"`
	JobID           string    `gorm:"column:job_id"`
	ChapterID       string    `gorm:"column:chapter_id"`
	VersionID       string    `gorm:"column:version_id"`
	VolumeID        string    `gorm:"column:volume_id"`
	VolumeTitle     string    `gorm:"column:volume_title"`
	Title           string    `gorm:"column:title"`
	VolumePosition  int       `gorm:"column:volume_position"`
	ChapterPosition int       `gorm:"column:chapter_position"`
	ChapterRevision int64     `gorm:"column:chapter_revision"`
	Body            string    `gorm:"column:body"`
	ContentHash     string    `gorm:"column:content_hash"`
}

func (snapshotModel) TableName() string { return "content_job_snapshots" }

func (r *Repository) InsertJobSnapshots(ctx context.Context, o a.Owner, snapshots []d.JobSnapshot) error {
	if len(snapshots) == 0 {
		return nil
	}
	db, e := r.db(ctx, true)
	if e != nil {
		return e
	}
	models := make([]snapshotModel, len(snapshots))
	for i, s := range snapshots {
		models[i] = snapshotModel{
			ID:              s.ID,
			TenantID:        o.TenantID,
			CreatedAt:       s.CreatedAt,
			UpdatedAt:       s.UpdatedAt,
			JobID:           s.JobID,
			ChapterID:       s.ChapterID,
			VersionID:       s.VersionID,
			VolumeID:        s.VolumeID,
			VolumeTitle:     s.VolumeTitle,
			Title:           s.Title,
			VolumePosition:  s.VolumePosition,
			ChapterPosition: s.ChapterPosition,
			ChapterRevision: s.ChapterRevision,
			Body:            s.Body,
			ContentHash:     s.ContentHash,
		}
	}
	return db.Table("content_job_snapshots").CreateInBatches(models, 100).Error
}

func (r *Repository) ListJobSnapshots(ctx context.Context, o a.Owner, jobID string) ([]d.JobSnapshot, error) {
	db, e := r.db(ctx, false)
	if e != nil {
		return nil, e
	}
	var models []snapshotModel
	err := db.Table("content_job_snapshots").Where("tenant_id=? AND job_id=?", o.TenantID, jobID).Order("volume_position ASC, chapter_position ASC").Find(&models).Error
	if err != nil {
		return nil, err
	}
	out := make([]d.JobSnapshot, len(models))
	for i, m := range models {
		out[i] = d.JobSnapshot{
			ID:              m.ID,
			TenantID:        m.TenantID,
			CreatedAt:       m.CreatedAt,
			UpdatedAt:       m.UpdatedAt,
			JobID:           m.JobID,
			ChapterID:       m.ChapterID,
			VersionID:       m.VersionID,
			VolumeID:        m.VolumeID,
			VolumeTitle:     m.VolumeTitle,
			Title:           m.Title,
			VolumePosition:  m.VolumePosition,
			ChapterPosition: m.ChapterPosition,
			ChapterRevision: m.ChapterRevision,
			Body:            m.Body,
			ContentHash:     m.ContentHash,
		}
	}
	return out, nil
}
