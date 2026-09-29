package domain

import "time"

const (
	JobKindImport = "import"
	JobKindExport = "export"
	JobKindCopy   = "copy"

	JobStatusQueued       = "queued"
	JobStatusRunning      = "running"
	JobStatusPreviewReady = "preview_ready"
	JobStatusCommitting   = "committing"
	JobStatusCancelling   = "cancelling"
	JobStatusSucceeded    = "succeeded"
	JobStatusFailed       = "failed"
	JobStatusCancelled    = "cancelled"
	JobStatusExpired      = "expired"

	JobStageQueued   = "queued"
	JobStageSnapshot = "snapshot"
	JobStageParse    = "parse"
	JobStagePreview  = "preview"
	JobStageCommit   = "commit"
	JobStageRender   = "render"
	JobStageComplete = "complete"

	JobFormatTXT      = "txt"
	JobFormatMarkdown = "markdown"
	JobFormatDOCX     = "docx"
	JobFormatEPUB     = "epub"
	JobFormatCopy     = "copy"
)

type Job struct {
	ID                 string     `json:"id"`
	TenantID           string     `json:"tenant_id"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	Revision           int64      `json:"revision"`
	NovelID            string     `json:"novel_id"`
	ActorID            string     `json:"actor_id"`
	Kind               string     `json:"kind"`
	Status             string     `json:"status"`
	Stage              string     `json:"stage"`
	Progress           int        `json:"progress"`
	SourceFileID       *string    `json:"source_file_id"`
	OutputFileID       *string    `json:"output_file_id"`
	TargetNovelID      *string    `json:"target_novel_id"`
	Format             string     `json:"format"`
	SourceRevision     int64      `json:"source_revision"`
	LeaseUntil         *time.Time `json:"lease_until"`
	Attempts           int        `json:"attempts"`
	ExpiresAt          time.Time  `json:"expires_at"`
	ErrorCode          *string    `json:"error_code"`
	ResultChapterCount int        `json:"result_chapter_count"`
	CopyTitle          *string    `json:"copy_title"`
	ReviewTicket       *string    `json:"review_ticket"`
}

type JobPage struct {
	Items      []Job   `json:"items"`
	NextCursor *string `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
}

type ImportItem struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Revision    int64     `json:"revision"`
	JobID       string    `json:"job_id"`
	Ordinal     int       `json:"ordinal"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	WordCount   int64     `json:"word_count"`
	ContentHash string    `json:"content_hash"`
	Selected    bool      `json:"selected"`
	Warnings    []string  `json:"warnings"`
}

type ImportPreview struct {
	Items      []ImportItem `json:"items"`
	NextCursor *string      `json:"next_cursor"`
	HasMore    bool         `json:"has_more"`
}

type JobSnapshot struct {
	ID              string    `json:"id"`
	TenantID        string    `json:"tenant_id"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	JobID           string    `json:"job_id"`
	ChapterID       string    `json:"chapter_id"`
	VersionID       string    `json:"version_id"`
	VolumeID        string    `json:"volume_id"`
	VolumeTitle     string    `json:"volume_title"`
	Title           string    `json:"title"`
	VolumePosition  int       `json:"volume_position"`
	ChapterPosition int       `json:"chapter_position"`
	ChapterRevision int64     `json:"chapter_revision"`
	Body            string    `json:"body"`
	ContentHash     string    `json:"content_hash"`
}

type CreateImport struct {
	ExpectedRevision int64  `json:"expected_revision"`
	FileID           string `json:"file_id"`
	Format           string `json:"format"`
}

type CommitImport struct {
	ExpectedRevision      int64    `json:"expected_revision"`
	ExpectedNovelRevision int64    `json:"expected_novel_revision"`
	VolumeID              string   `json:"volume_id"`
	SelectedItemIDs       []string `json:"selected_item_ids"`
}

type CreateExport struct {
	ExpectedRevision int64    `json:"expected_revision"`
	Format           string   `json:"format"`
	VolumeIDs        []string `json:"volume_ids"`
	IncludeDrafts    bool     `json:"include_drafts"`
	ReviewTicket     *string  `json:"review_ticket"`
}

type CopyNovel struct {
	ExpectedRevision int64  `json:"expected_revision"`
	Title            string `json:"title"`
	IncludePlanning  bool   `json:"include_planning"`
	IncludeWorldview bool   `json:"include_worldview"`
}

type SearchHit struct {
	ChapterID       string `json:"chapter_id"`
	VersionID       string `json:"version_id"`
	Title           string `json:"title"`
	ContentHash     string `json:"content_hash"`
	ChapterRevision int64  `json:"chapter_revision"`
	Snippet         string `json:"snippet"`
	StartOffset     int    `json:"start_offset"`
	EndOffset       int    `json:"end_offset"`
}

type SearchPage struct {
	Items      []SearchHit `json:"items"`
	NextCursor *string     `json:"next_cursor"`
	HasMore    bool        `json:"has_more"`
}

type DailyStat struct {
	Date         string `json:"date"`
	AddedWords   int64  `json:"added_words"`
	RemovedWords int64  `json:"removed_words"`
	NetWords     int64  `json:"net_words"`
	SaveCount    int    `json:"save_count"`
	Timezone     string `json:"timezone"`
}

type Statistics struct {
	NovelID          string      `json:"novel_id"`
	From             string      `json:"from"`
	To               string      `json:"to"`
	Days             []DailyStat `json:"days"`
	CurrentWordCount int64       `json:"current_word_count"`
}
