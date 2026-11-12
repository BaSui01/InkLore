// package domain contains the content authority value types and invariants.
package domain

import "time"

type Novel struct {
	ID                   string     `json:"id"`
	TenantID             string     `json:"tenant_id"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
	Revision             int64      `json:"revision"`
	OwnerID              string     `json:"owner_id"`
	Title                string     `json:"title"`
	Summary              string     `json:"summary"`
	Category             string     `json:"category"`
	Status               string     `json:"status"`
	PreviousStatus       *string    `json:"previous_status"`
	TargetWords          int64      `json:"target_words"`
	TargetCompletionDate *string    `json:"target_completion_date"`
	WordCount            int64      `json:"word_count"`
	DeletedAt            *time.Time `json:"deleted_at"`
}

type Volume struct {
	ID        string     `json:"id"`
	TenantID  string     `json:"tenant_id"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	Revision  int64      `json:"revision"`
	NovelID   string     `json:"novel_id"`
	Title     string     `json:"title"`
	Summary   string     `json:"summary"`
	Position  int64      `json:"position"`
	DeletedAt *time.Time `json:"deleted_at"`
}

type Chapter struct {
	ID                 string     `json:"id"`
	TenantID           string     `json:"tenant_id"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	Revision           int64      `json:"revision"`
	NovelID            string     `json:"novel_id"`
	VolumeID           string     `json:"volume_id"`
	Title              string     `json:"title"`
	Position           int64      `json:"position"`
	Status             string     `json:"status"`
	CurrentVersionID   string     `json:"current_version_id"`
	ConfirmedVersionID *string    `json:"confirmed_version_id"`
	ContentHash        string     `json:"content_hash"`
	WordCount          int64      `json:"word_count"`
	DeletedAt          *time.Time `json:"deleted_at"`
}

type Body struct {
	Chapter Chapter `json:"chapter"`
	Body    string  `json:"body"`
}

type Version struct {
	ID                    string    `json:"id"`
	TenantID              string    `json:"tenant_id"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
	ChapterID             string    `json:"chapter_id"`
	NovelID               string    `json:"novel_id"`
	ChapterRevision       int64     `json:"chapter_revision"`
	Title                 string    `json:"title"`
	Body                  string    `json:"body"`
	ContentHash           string    `json:"content_hash"`
	WordCount             int64     `json:"word_count"`
	Source                string    `json:"source"`
	ActorID               *string   `json:"actor_id"`
	RestoredFromVersionID *string   `json:"restored_from_version_id"`
}

type VersionSummary struct {
	ID                    string    `json:"id"`
	TenantID              string    `json:"tenant_id"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
	ChapterID             string    `json:"chapter_id"`
	NovelID               string    `json:"novel_id"`
	ChapterRevision       int64     `json:"chapter_revision"`
	Title                 string    `json:"title"`
	ContentHash           string    `json:"content_hash"`
	WordCount             int64     `json:"word_count"`
	Source                string    `json:"source"`
	ActorID               *string   `json:"actor_id"`
	RestoredFromVersionID *string   `json:"restored_from_version_id"`
}

type DiffHunk struct {
	Kind      string `json:"kind"`
	OldStart  int64  `json:"old_start"`
	OldLength int64  `json:"old_length"`
	NewStart  int64  `json:"new_start"`
	NewLength int64  `json:"new_length"`
	Text      string `json:"text"`
}

type Diff struct {
	FromVersionID string     `json:"from_version_id"`
	ToVersionID   string     `json:"to_version_id"`
	OffsetUnit    string     `json:"offset_unit"`
	Hunks         []DiffHunk `json:"hunks"`
	Coarse        bool       `json:"coarse"`
}

type Organization struct {
	NovelRevision int64     `json:"novel_revision"`
	Volumes       []Volume  `json:"volumes"`
	Chapters      []Chapter `json:"chapters"`
}

type Placement struct {
	ChapterID string `json:"chapter_id"`
	VolumeID  string `json:"volume_id"`
	Position  int64  `json:"position"`
}

type CreateNovel struct {
	Title                string  `json:"title"`
	Summary              string  `json:"summary"`
	Category             string  `json:"category"`
	TargetWords          int64   `json:"target_words"`
	TargetCompletionDate *string `json:"target_completion_date"`
}

type UpdateNovel struct {
	ExpectedRevision     int64   `json:"expected_revision"`
	Title                string  `json:"title"`
	Summary              string  `json:"summary"`
	Category             string  `json:"category"`
	TargetWords          int64   `json:"target_words"`
	TargetCompletionDate *string `json:"target_completion_date"`
}

type RevisionCommand struct {
	ExpectedRevision int64 `json:"expected_revision"`
}

type CreateVolume struct {
	Title   string `json:"title"`
	Summary string `json:"summary"`
}

type UpdateVolume struct {
	ExpectedRevision int64  `json:"expected_revision"`
	Title            string `json:"title"`
	Summary          string `json:"summary"`
}

type CreateChapter struct {
	VolumeID string `json:"volume_id"`
	Title    string `json:"title"`
}

type UpdateChapter struct {
	ExpectedRevision int64  `json:"expected_revision"`
	Title            string `json:"title"`
}

type SaveBody struct {
	ExpectedRevision int64  `json:"expected_revision"`
	Body             string `json:"body"`
}

type Reorder struct {
	ExpectedRevision int64       `json:"expected_revision"`
	VolumeIds        []string    `json:"volume_ids"`
	Chapters         []Placement `json:"chapters"`
}

type Rollback struct {
	ExpectedRevision int64  `json:"expected_revision"`
	VersionID        string `json:"version_id"`
}

type NovelPage struct {
	Items      []Novel `json:"items"`
	NextCursor *string `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
}

type VolumePage struct {
	Items      []Volume `json:"items"`
	NextCursor *string  `json:"next_cursor"`
	HasMore    bool     `json:"has_more"`
}

type ChapterPage struct {
	Items      []Chapter `json:"items"`
	NextCursor *string   `json:"next_cursor"`
	HasMore    bool      `json:"has_more"`
}

type VersionPage struct {
	Items      []VersionSummary `json:"items"`
	NextCursor *string          `json:"next_cursor"`
	HasMore    bool             `json:"has_more"`
}

// ChapterRecord is a repository value. Body never appears in metadata responses.
type ChapterRecord struct {
	Chapter
	Body string `json:"-"`
}
