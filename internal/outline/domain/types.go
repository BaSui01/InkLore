// package domain contains the frozen outline values and invariants.
package domain

import "time"

type Node struct {
	ID          string     `json:"id"`
	TenantID    string     `json:"tenant_id"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Revision    int64      `json:"revision"`
	NovelID     string     `json:"novel_id"`
	ParentID    *string    `json:"parent_id"`
	Level       string     `json:"level"`
	TargetID    string     `json:"target_id"`
	Title       string     `json:"title"`
	Summary     string     `json:"summary"`
	Position    int64      `json:"position"`
	Status      string     `json:"status"`
	DeletedAt   *time.Time `json:"deleted_at"`
	ContentHash string     `json:"content_hash"`
}

type Beat struct {
	ID            string     `json:"id"`
	TenantID      string     `json:"tenant_id"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	Revision      int64      `json:"revision"`
	NovelID       string     `json:"novel_id"`
	NodeID        string     `json:"node_id"`
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	BeatType      string     `json:"beat_type"`
	HookType      string     `json:"hook_type"`
	Tension       int64      `json:"tension"`
	ExpectedWords int64      `json:"expected_words"`
	Position      int64      `json:"position"`
	Status        string     `json:"status"`
	DeletedAt     *time.Time `json:"deleted_at"`
}

type Scene struct {
	ID               string     `json:"id"`
	TenantID         string     `json:"tenant_id"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	Revision         int64      `json:"revision"`
	NovelID          string     `json:"novel_id"`
	NodeID           string     `json:"node_id"`
	Title            string     `json:"title"`
	Summary          string     `json:"summary"`
	PovEntityID      *string    `json:"pov_entity_id"`
	LocationEntityID *string    `json:"location_entity_id"`
	Position         int64      `json:"position"`
	Status           string     `json:"status"`
	SourceRevision   *int64     `json:"source_revision"`
	SourceHash       *string    `json:"source_hash"`
	StartOffset      *int64     `json:"start_offset"`
	EndOffset        *int64     `json:"end_offset"`
	DeletedAt        *time.Time `json:"deleted_at"`
}

type Clue struct {
	ID                  string     `json:"id"`
	TenantID            string     `json:"tenant_id"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	Revision            int64      `json:"revision"`
	NovelID             string     `json:"novel_id"`
	Title               string     `json:"title"`
	Description         string     `json:"description"`
	Status              string     `json:"status"`
	PlantedNodeID       *string    `json:"planted_node_id"`
	PlannedRevealNodeID *string    `json:"planned_reveal_node_id"`
	LastNodeID          *string    `json:"last_node_id"`
	Reason              *string    `json:"reason"`
	DeletedAt           *time.Time `json:"deleted_at"`
}

type ClueEvent struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	ClueID       string    `json:"clue_id"`
	NovelID      string    `json:"novel_id"`
	FromStatus   string    `json:"from_status"`
	ToStatus     string    `json:"to_status"`
	NodeID       *string   `json:"node_id"`
	Reason       *string   `json:"reason"`
	ActorID      string    `json:"actor_id"`
	ClueRevision int64     `json:"clue_revision"`
}

type PacingPoint struct {
	NodeID    string   `json:"node_id"`
	ChapterID string   `json:"chapter_id"`
	Ordinal   int64    `json:"ordinal"`
	Tension   *float64 `json:"tension"`
	BeatCount int64    `json:"beat_count"`
}

type PacingWarning struct {
	Code    string   `json:"code"`
	NodeIDs []string `json:"node_ids"`
	ClueID  *string  `json:"clue_id"`
}

type Pacing struct {
	Points            []PacingPoint   `json:"points"`
	Warnings          []PacingWarning `json:"warnings"`
	OutlineRevision   int64           `json:"outline_revision"`
	WarningsTruncated bool            `json:"warnings_truncated"`
}

type CreateNode struct {
	ParentID *string `json:"parent_id"`
	Level    string  `json:"level"`
	TargetID string  `json:"target_id"`
	Title    string  `json:"title"`
	Summary  string  `json:"summary"`
}

type UpdateNode struct {
	ExpectedRevision int64  `json:"expected_revision"`
	Title            string `json:"title"`
	Summary          string `json:"summary"`
	Status           string `json:"status"`
}

type Reorder struct {
	ExpectedRevision int64    `json:"expected_revision"`
	Resource         string   `json:"resource"`
	OrderedIDs       []string `json:"ordered_ids"`
}

type CreateBeat struct {
	NodeID        string `json:"node_id"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	BeatType      string `json:"beat_type"`
	HookType      string `json:"hook_type"`
	Tension       int64  `json:"tension"`
	ExpectedWords int64  `json:"expected_words"`
}

type UpdateBeat struct {
	ExpectedRevision int64  `json:"expected_revision"`
	NodeID           string `json:"node_id"`
	Title            string `json:"title"`
	Description      string `json:"description"`
	BeatType         string `json:"beat_type"`
	HookType         string `json:"hook_type"`
	Tension          int64  `json:"tension"`
	ExpectedWords    int64  `json:"expected_words"`
	Status           string `json:"status"`
}

type CreateScene struct {
	NodeID           string  `json:"node_id"`
	Title            string  `json:"title"`
	Summary          string  `json:"summary"`
	PovEntityID      *string `json:"pov_entity_id"`
	LocationEntityID *string `json:"location_entity_id"`
	SourceRevision   *int64  `json:"source_revision"`
	SourceHash       *string `json:"source_hash"`
	StartOffset      *int64  `json:"start_offset"`
	EndOffset        *int64  `json:"end_offset"`
}

type UpdateScene struct {
	ExpectedRevision int64   `json:"expected_revision"`
	NodeID           string  `json:"node_id"`
	Title            string  `json:"title"`
	Summary          string  `json:"summary"`
	PovEntityID      *string `json:"pov_entity_id"`
	LocationEntityID *string `json:"location_entity_id"`
	SourceRevision   *int64  `json:"source_revision"`
	SourceHash       *string `json:"source_hash"`
	StartOffset      *int64  `json:"start_offset"`
	EndOffset        *int64  `json:"end_offset"`
	Status           string  `json:"status"`
}

type CreateClue struct {
	Title               string  `json:"title"`
	Description         string  `json:"description"`
	PlannedRevealNodeID *string `json:"planned_reveal_node_id"`
}

type UpdateClue struct {
	ExpectedRevision    int64   `json:"expected_revision"`
	Title               string  `json:"title"`
	Description         string  `json:"description"`
	PlannedRevealNodeID *string `json:"planned_reveal_node_id"`
}

type TransitionClue struct {
	ExpectedRevision int64   `json:"expected_revision"`
	ToStatus         string  `json:"to_status"`
	NodeID           *string `json:"node_id"`
	Reason           *string `json:"reason"`
}

type OrderResult struct {
	Parent     Node     `json:"parent"`
	OrderedIDs []string `json:"ordered_ids"`
}

type RevisionCommand struct {
	ExpectedRevision int64 `json:"expected_revision"`
}

type RebindNode struct {
	ExpectedRevision int64  `json:"expected_revision"`
	ParentID         string `json:"parent_id"`
}

type Snapshot struct {
	NovelID         string  `json:"novel_id"`
	OutlineRevision int64   `json:"outline_revision"`
	Nodes           []Node  `json:"nodes"`
	Beats           []Beat  `json:"beats"`
	Scenes          []Scene `json:"scenes"`
	Clues           []Clue  `json:"clues"`
}
type State struct {
	Snapshot
	Events []ClueEvent
}
type IDMapping struct {
	Kind     string `json:"kind"`
	SourceID string `json:"source_id"`
	TargetID string `json:"target_id"`
}
type Page[T any] struct {
	Items      []T     `json:"items"`
	NextCursor *string `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
}
