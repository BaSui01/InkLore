// package domain defines the compliance domain entities, value objects, and invariants.
package domain

import (
	"time"
)

const (
	MaxTextRunes           = 200000
	MaxIssues              = 10000
	MaxRules               = 20000
	MaxPatterns            = 100
	MaxRounds              = 3
	MaxContextTokens       = 60000
	FormulaVersion         = "r1"
	DefaultTotalTokens     = 30000
	DefaultMaxOutputTokens = 4000
	DefaultMaxCalls        = 4
	DefaultRewriteMaxCalls = 6
	DefaultRetentionDays   = 30
)

const (
	RuleSetStateDraft     = "draft"
	RuleSetStatePublished = "published"
	RuleSetStateRetired   = "retired"
)

const (
	RuleKindSensitiveLiteral = "sensitive_literal"
	RuleKindAIPhrase         = "ai_phrase"
	RuleKindAIPattern        = "ai_pattern"
)

const (
	SeverityWarning = "warning"
	SeverityBlock   = "block"
)

const (
	ReportModeRules = "rules"
	ReportModeDeep  = "deep"
)

const (
	ReportStatePending   = "pending"
	ReportStateRunning   = "running"
	ReportStateSucceeded = "succeeded"
	ReportStateFailed    = "failed"
	ReportStateCancelled = "cancelled"
)

const (
	DetectorRule = "rule"
	DetectorLLM  = "llm"
)

const (
	IssueKindSensitive = "sensitive"
	IssueKindAIFlavor  = "ai_flavor"
)

const (
	IssueStateOpen    = "open"
	IssueStateIgnored = "ignored"
)

const (
	DecisionActionIgnore = "ignore"
	DecisionActionReopen = "reopen"
)

const (
	StopReasonClean         = "clean"
	StopReasonMaxRounds     = "max_rounds"
	StopReasonBudget        = "budget"
	StopReasonNoImprovement = "no_improvement"
)

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

type Patch struct {
	StartOffset int    `json:"start_offset"`
	EndOffset   int    `json:"end_offset"`
	OldText     string `json:"old_text"`
	NewText     string `json:"new_text"`
}

type RuleSet struct {
	ID          string     `json:"id"`
	TenantID    string     `json:"tenant_id"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Revision    int64      `json:"revision"`
	Name        string     `json:"name"`
	VersionNo   int64      `json:"version_no"`
	State       string     `json:"state"`
	Checksum    *string    `json:"checksum"`
	CreatedBy   string     `json:"created_by"`
	PublishedAt *time.Time `json:"published_at"`
}

type Rule struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	Revision   int64     `json:"revision"`
	RuleSetID  string    `json:"rule_set_id"`
	Kind       string    `json:"kind"`
	Expression string    `json:"expression"`
	Severity   string    `json:"severity"`
	Category   string    `json:"category"`
	Advice     string    `json:"advice"`
	Weight     int       `json:"weight"`
	Enabled    bool      `json:"enabled"`
}

type Report struct {
	ID              string     `json:"id"`
	TenantID        string     `json:"tenant_id"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	Revision        int64      `json:"revision"`
	OwnerUserID     string     `json:"owner_user_id"`
	NovelID         string     `json:"novel_id"`
	ChapterID       string     `json:"chapter_id"`
	ChapterRevision int64      `json:"chapter_revision"`
	ContentHash     string     `json:"content_hash"`
	SourceText      string     `json:"-"`
	RuleSetID       string     `json:"rule_set_id"`
	Mode            string     `json:"mode"`
	State           string     `json:"state"`
	RunID           *string    `json:"run_id"`
	ExecutionKey    *string    `json:"-"`
	ExecutionFence  int64      `json:"-"`
	Budget          Budget     `json:"budget"`
	Usage           Usage      `json:"usage"`
	Summary         string     `json:"summary"`
	AIFlavorScore   *int       `json:"ai_flavor_score"`
	FormulaVersion  string     `json:"formula_version"`
	RiskCount       int        `json:"risk_count"`
	OpenCount       int        `json:"open_count"`
	BlockCount      int        `json:"block_count"`
	FailureCode     *string    `json:"failure_code"`
	ExpireAt        time.Time  `json:"expire_at"`
	RedactedAt      *time.Time `json:"redacted_at"`
}

type Issue struct {
	ID               string    `json:"id"`
	TenantID         string    `json:"tenant_id"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	Revision         int64     `json:"revision"`
	ReportID         string    `json:"report_id"`
	RuleID           *string   `json:"rule_id"`
	Detector         string    `json:"detector"`
	Kind             string    `json:"kind"`
	Severity         string    `json:"severity"`
	StartOffset      int       `json:"start_offset"`
	EndOffset        int       `json:"end_offset"`
	Excerpt          string    `json:"excerpt"`
	ExcerptTruncated bool      `json:"excerpt_truncated"`
	Message          string    `json:"message"`
	Suggestion       string    `json:"suggestion"`
	Confidence       float64   `json:"confidence"`
	State            string    `json:"state"`
}

type IssueDecision struct {
	ID            string    `json:"id"`
	TenantID      string    `json:"tenant_id"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	ReportID      string    `json:"report_id"`
	IssueID       string    `json:"issue_id"`
	ActorUserID   string    `json:"actor_user_id"`
	Action        string    `json:"action"`
	Reason        string    `json:"reason"`
	IssueRevision int64     `json:"issue_revision"`
}

type Review struct {
	ID               string    `json:"id"`
	TenantID         string    `json:"tenant_id"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	ReportID         string    `json:"report_id"`
	ReportRevision   int64     `json:"report_revision"`
	ChapterID        string    `json:"chapter_id"`
	ChapterRevision  int64     `json:"chapter_revision"`
	ContentHash      string    `json:"content_hash"`
	RuleSetID        string    `json:"rule_set_id"`
	ActorUserID      string    `json:"actor_user_id"`
	AcknowledgedRisk bool      `json:"acknowledged_risk"`
	Note             string    `json:"note"`
}

type Rewrite struct {
	ID                 string     `json:"id"`
	TenantID           string     `json:"tenant_id"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	Revision           int64      `json:"revision"`
	OwnerUserID        string     `json:"owner_user_id"`
	ReportID           string     `json:"report_id"`
	ReportRevision     int64      `json:"report_revision"`
	SelectedIssueIDs   []string   `json:"selected_issue_ids"`
	State              string     `json:"state"`
	MaxRounds          int        `json:"max_rounds"`
	RoundsUsed         int        `json:"rounds_used"`
	Budget             Budget     `json:"budget"`
	Usage              Usage      `json:"usage"`
	RunID              *string    `json:"run_id"`
	ExecutionKey       *string    `json:"-"`
	ExecutionFence     int64      `json:"-"`
	Patches            []Patch    `json:"patches"`
	CandidateText      *string    `json:"candidate_text"`
	CandidateHash      *string    `json:"candidate_hash"`
	RemainingRiskCount *int       `json:"remaining_risk_count"`
	StopReason         *string    `json:"stop_reason"`
	FailureCode        *string    `json:"failure_code"`
	RedactedAt         *time.Time `json:"redacted_at"`
}

type Deleted struct {
	ID       string `json:"id"`
	Revision int64  `json:"revision"`
	Deleted  bool   `json:"deleted"`
}

type Expected struct {
	ExpectedRevision int64 `json:"expected_revision"`
}

type ReasonCommand struct {
	ExpectedRevision int64  `json:"expected_revision"`
	Reason           string `json:"reason"`
}

type RuleSetCreate struct {
	Name          string  `json:"name"`
	BaseRuleSetID *string `json:"base_rule_set_id"`
}

type RuleCreate struct {
	ExpectedRevision int64  `json:"expected_revision"`
	Kind             string `json:"kind"`
	Expression       string `json:"expression"`
	Severity         string `json:"severity"`
	Category         string `json:"category"`
	Advice           string `json:"advice"`
	Weight           int    `json:"weight"`
	Enabled          bool   `json:"enabled"`
}

type RuleUpdate struct {
	ExpectedRevision int64  `json:"expected_revision"`
	Kind             string `json:"kind"`
	Expression       string `json:"expression"`
	Severity         string `json:"severity"`
	Category         string `json:"category"`
	Advice           string `json:"advice"`
	Weight           int    `json:"weight"`
	Enabled          bool   `json:"enabled"`
}

type ScanCreate struct {
	NovelID         string `json:"novel_id"`
	ChapterID       string `json:"chapter_id"`
	ChapterRevision int64  `json:"chapter_revision"`
	ContentHash     string `json:"content_hash"`
	Text            string `json:"text"`
}

type ReportCreate struct {
	NovelID         string `json:"novel_id"`
	ChapterID       string `json:"chapter_id"`
	ChapterRevision int64  `json:"chapter_revision"`
	ContentHash     string `json:"content_hash"`
	Text            string `json:"text"`
	Budget          Budget `json:"budget"`
}

type ReviewCreate struct {
	ExpectedRevision int64  `json:"expected_revision"`
	AcknowledgedRisk bool   `json:"acknowledged_risk"`
	Note             string `json:"note"`
}

type RewriteCreate struct {
	ReportID         string   `json:"report_id"`
	ExpectedRevision int64    `json:"expected_revision"`
	SelectedIssueIDs []string `json:"selected_issue_ids"`
	MaxRounds        int      `json:"max_rounds"`
	Budget           Budget   `json:"budget"`
}

type Page[T any] struct {
	Items      []T     `json:"items"`
	NextCursor *string `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
}

type ListQuery struct {
	Limit     int
	Cursor    string
	State     string
	ChapterID string
	ReportID  string
	IssueID   string
	BeforeAt  time.Time
	BeforeID  string
}
