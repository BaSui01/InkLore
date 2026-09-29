// package domain contains the domain value types, invariants, and aggregate definitions
// for the inspiration and deconstruction bounded context.
package domain

import (
	"time"
)

// Constants and Enums
const (
	AssetSourceKindFile  = "file"
	AssetSourceKindPaste = "paste"

	RightsStatusOwned        = "owned"
	RightsStatusLicensed     = "licensed"
	RightsStatusPublicDomain = "public_domain"
	RightsStatusUnknown      = "unknown"

	AssetStateReady   = "ready"
	AssetStateDeleted = "deleted"

	AnalysisKindDeconstruction = "deconstruction"
	AnalysisKindStyle          = "style"
	AnalysisKindIdeas          = "ideas"

	AnalysisStatePending   = "pending"
	AnalysisStateRunning   = "running"
	AnalysisStateSucceeded = "succeeded"
	AnalysisStateFailed    = "failed"
	AnalysisStateCancelled = "cancelled"

	StyleStateActive  = "active"
	StyleStateDeleted = "deleted"

	CardCategoryOpening     = "opening"
	CardCategoryConflict    = "conflict"
	CardCategoryTwist       = "twist"
	CardCategoryCliffhanger = "cliffhanger"
	CardCategorySetting     = "setting"

	CardSourceKindOriginal  = "original"
	CardSourceKindReference = "reference"

	CardStateSuggested = "suggested"
	CardStateArchived  = "archived"
	CardStateDeleted   = "deleted"

	AdoptionDomainContent = "content"
	AdoptionDomainOutline = "outline"

	AdoptionStateActive  = "active"
	AdoptionStateRevoked = "revoked"

	SensoryBiasVisual    = "visual"
	SensoryBiasAuditory  = "auditory"
	SensoryBiasTactile   = "tactile"
	SensoryBiasOlfactory = "olfactory"
	SensoryBiasBalanced  = "balanced"

	FindingDimensionOpeningHook = "opening_hook"
	FindingDimensionConflict    = "conflict"
	FindingDimensionPacing      = "pacing"
	FindingDimensionCharacter   = "character"
	FindingDimensionCliffhanger = "cliffhanger"

	MaxAssetCodepoints    = 200000
	MaxSegmentCodepoints  = 4000
	MaxEvidenceExcerpt    = 500
	MaxCardBodyRunes      = 4000
	MaxCardRationaleRunes = 2000
	MaxStyleInstructions  = 4000
	MaxBriefRunes         = 4000
	MinIdeasBriefRunes    = 50
	MinStyleCodepoints    = 1000
)

// Common commands and responses
type Expected struct {
	ExpectedRevision int64 `json:"expected_revision"`
}

type ReasonCommand struct {
	ExpectedRevision int64  `json:"expected_revision"`
	Reason           string `json:"reason"`
}

type Deleted struct {
	ID       string `json:"id"`
	Revision int64  `json:"revision"`
	Deleted  bool   `json:"deleted"`
}

type PageResult[T any] struct {
	Items      []T     `json:"items"`
	NextCursor *string `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
}

// Asset and Segment types
type Asset struct {
	ID             string     `json:"id"`
	TenantID       string     `json:"tenant_id"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	Revision       int64      `json:"revision"`
	OwnerUserID    string     `json:"owner_user_id"`
	FileID         string     `json:"file_id"`
	Title          string     `json:"title"`
	SourceKind     string     `json:"source_kind"`
	Author         string     `json:"author"`
	SourceURL      *string    `json:"source_url"`
	SourceNote     string     `json:"source_note"`
	RightsStatus   string     `json:"rights_status"`
	ContentHash    string     `json:"content_hash"`
	CodepointCount int        `json:"codepoint_count"`
	State          string     `json:"state"`
	DeletedAt      *time.Time `json:"deleted_at"`
	RedactedAt     *time.Time `json:"redacted_at"`
}

type AssetCreate struct {
	Title        string  `json:"title"`
	SourceKind   string  `json:"source_kind"`
	FileID       *string `json:"file_id,omitempty"`
	PasteText    *string `json:"paste_text,omitempty"`
	Author       string  `json:"author"`
	SourceURL    *string `json:"source_url,omitempty"`
	SourceNote   string  `json:"source_note"`
	RightsStatus string  `json:"rights_status"`
}

type AssetUpdate struct {
	ExpectedRevision int64   `json:"expected_revision"`
	Title            string  `json:"title"`
	Author           string  `json:"author"`
	SourceURL        *string `json:"source_url,omitempty"`
	SourceNote       string  `json:"source_note"`
	RightsStatus     string  `json:"rights_status"`
}

type Segment struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	AssetID     string    `json:"asset_id"`
	Seq         int       `json:"seq"`
	StartOffset int       `json:"start_offset"`
	EndOffset   int       `json:"end_offset"`
	Text        string    `json:"text"`
}

type SegmentSpan struct {
	Seq         int
	StartOffset int
	EndOffset   int
}

// Analysis types
type Budget struct {
	TotalTokens     int `json:"total_tokens"`
	MaxOutputTokens int `json:"max_output_tokens"`
	MaxCalls        int `json:"max_calls"`
}

type Usage struct {
	UsedTokens     int `json:"used_tokens"`
	ReservedTokens int `json:"reserved_tokens"`
	CallsUsed      int `json:"calls_used"`
}

type Analysis struct {
	ID            string    `json:"id"`
	TenantID      string    `json:"tenant_id"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	Revision      int64     `json:"revision"`
	OwnerUserID   string    `json:"owner_user_id"`
	NovelID       *string   `json:"novel_id"`
	AssetID       *string   `json:"asset_id"`
	AssetRevision *int64    `json:"asset_revision"`
	Kind          string    `json:"kind"`
	Brief         string    `json:"brief"`
	State         string    `json:"state"`
	RunID         *string   `json:"run_id"`
	Budget        Budget    `json:"budget"`
	Usage         Usage     `json:"usage"`
	FailureCode   *string   `json:"failure_code"`
}

type AnalysisCreate struct {
	Kind          string  `json:"kind"`
	AssetID       *string `json:"asset_id,omitempty"`
	AssetRevision *int64  `json:"asset_revision,omitempty"`
	NovelID       *string `json:"novel_id,omitempty"`
	Brief         string  `json:"brief"`
	Budget        Budget  `json:"budget"`
}

// Report and Evidence types
type Evidence struct {
	AssetID           string `json:"asset_id"`
	AssetRevision     int64  `json:"asset_revision"`
	SegmentID         string `json:"segment_id"`
	StartOffset       int    `json:"start_offset"`
	EndOffset         int    `json:"end_offset"`
	ContentHash       string `json:"content_hash"`
	Excerpt           string `json:"excerpt"`
	SourceUnavailable bool   `json:"source_unavailable"`
}

type Finding struct {
	Dimension   string     `json:"dimension"`
	Observation string     `json:"observation"`
	Confidence  float64    `json:"confidence"`
	Evidence    []Evidence `json:"evidence"`
}

type Pattern struct {
	Name           string     `json:"name"`
	Steps          []string   `json:"steps"`
	Applicability  string     `json:"applicability"`
	Counterexample string     `json:"counterexample"`
	Evidence       []Evidence `json:"evidence"`
}

type StyleMetrics struct {
	MeanSentenceLength float64 `json:"mean_sentence_length"`
	DialogueRatio      float64 `json:"dialogue_ratio"`
	SensoryBias        string  `json:"sensory_bias"`
	Instructions       string  `json:"instructions"`
}

type Idea struct {
	Title     string     `json:"title"`
	Category  string     `json:"category"`
	Body      string     `json:"body"`
	Rationale string     `json:"rationale"`
	Evidence  []Evidence `json:"evidence"`
}

type Report struct {
	ID           string        `json:"id"`
	TenantID     string        `json:"tenant_id"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
	Revision     int64         `json:"revision"`
	AnalysisID   string        `json:"analysis_id"`
	Summary      string        `json:"summary"`
	Findings     []Finding     `json:"findings"`
	Patterns     []Pattern     `json:"patterns"`
	StyleMetrics *StyleMetrics `json:"style_metrics"`
	Ideas        []Idea        `json:"ideas"`
	RedactedAt   *time.Time    `json:"redacted_at"`
}

// Style types
type Style struct {
	ID               string     `json:"id"`
	TenantID         string     `json:"tenant_id"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	Revision         int64      `json:"revision"`
	OwnerUserID      string     `json:"owner_user_id"`
	Title            string     `json:"title"`
	CurrentVersionID string     `json:"current_version_id"`
	State            string     `json:"state"`
	DeletedAt        *time.Time `json:"deleted_at"`
	RedactedAt       *time.Time `json:"redacted_at"`
}

type StyleCreate struct {
	Title          string       `json:"title"`
	Instructions   string       `json:"instructions"`
	Metrics        StyleMetrics `json:"metrics"`
	SourceReportID *string      `json:"source_report_id"`
}

type StyleUpdate struct {
	ExpectedRevision int64        `json:"expected_revision"`
	Title            string       `json:"title"`
	Instructions     string       `json:"instructions"`
	Metrics          StyleMetrics `json:"metrics"`
	SourceReportID   *string      `json:"source_report_id"`
}

type StyleVersion struct {
	ID             string       `json:"id"`
	TenantID       string       `json:"tenant_id"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
	Revision       int64        `json:"revision"`
	StyleID        string       `json:"style_id"`
	VersionNo      int          `json:"version_no"`
	Instructions   string       `json:"instructions"`
	Metrics        StyleMetrics `json:"metrics"`
	SourceReportID *string      `json:"source_report_id"`
	RedactedAt     *time.Time   `json:"redacted_at"`
}

// Card types
type Card struct {
	ID             string     `json:"id"`
	TenantID       string     `json:"tenant_id"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	Revision       int64      `json:"revision"`
	OwnerUserID    string     `json:"owner_user_id"`
	NovelID        *string    `json:"novel_id"`
	Title          string     `json:"title"`
	Category       string     `json:"category"`
	Body           string     `json:"body"`
	Rationale      string     `json:"rationale"`
	SourceKind     string     `json:"source_kind"`
	SourceReportID *string    `json:"source_report_id"`
	Evidence       []Evidence `json:"evidence"`
	Fingerprint    string     `json:"fingerprint"`
	State          string     `json:"state"`
	DeletedAt      *time.Time `json:"deleted_at"`
	RedactedAt     *time.Time `json:"redacted_at"`
}

type CardCreate struct {
	NovelID        *string    `json:"novel_id"`
	Title          string     `json:"title"`
	Category       string     `json:"category"`
	Body           string     `json:"body"`
	Rationale      string     `json:"rationale"`
	SourceKind     string     `json:"source_kind"`
	SourceReportID *string    `json:"source_report_id"`
	Evidence       []Evidence `json:"evidence"`
}

type CardUpdate struct {
	ExpectedRevision int64      `json:"expected_revision"`
	NovelID          *string    `json:"novel_id"`
	Title            string     `json:"title"`
	Category         string     `json:"category"`
	Body             string     `json:"body"`
	Rationale        string     `json:"rationale"`
	SourceKind       string     `json:"source_kind"`
	SourceReportID   *string    `json:"source_report_id"`
	Evidence         []Evidence `json:"evidence"`
}

type CardStateCommand struct {
	ExpectedRevision int64  `json:"expected_revision"`
	State            string `json:"state"`
}

// Adoption types
type Adoption struct {
	ID             string     `json:"id"`
	TenantID       string     `json:"tenant_id"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	Revision       int64      `json:"revision"`
	CardID         string     `json:"card_id"`
	ActorUserID    string     `json:"actor_user_id"`
	TargetDomain   string     `json:"target_domain"`
	TargetID       string     `json:"target_id"`
	TargetRevision int64      `json:"target_revision"`
	TargetHash     string     `json:"target_hash"`
	State          string     `json:"state"`
	RevokeReason   *string    `json:"revoke_reason"`
	RevokedAt      *time.Time `json:"revoked_at"`
}

type AdoptionCreate struct {
	ExpectedRevision int64  `json:"expected_revision"`
	TargetDomain     string `json:"target_domain"`
	TargetID         string `json:"target_id"`
	TargetRevision   int64  `json:"target_revision"`
	TargetHash       string `json:"target_hash"`
}
