package domain

import (
	"context"
	"time"
)

// AuditRunView is an explicit allowlist. Owner prompts, resources, question
// answers, event payloads and model routing must never enter this projection.
type AuditRunView struct {
	ID         string    `json:"id"`
	Revision   int64     `json:"revision"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	Kind       string    `json:"kind"`
	TaskKind   string    `json:"task_kind"`
	Status     RunStatus `json:"status"`
	NodeCount  int       `json:"node_count"`
	DurationMS int64     `json:"duration_ms"`
	CostMicros int64     `json:"cost_micros"`
	ErrorCode  *string   `json:"error_code"`
}

// AuditRunRecord is internal; receipt identifiers are never serialized.
type AuditRunRecord struct {
	View       AuditRunView
	ReceiptIDs []string
}

type AuditRunPage struct {
	Items      []AuditRunView `json:"items"`
	NextCursor *string        `json:"next_cursor"`
	HasMore    bool           `json:"has_more"`
}

type AuditRunRepository interface {
	ListAuditRuns(context.Context, string, ListQuery) ([]AuditRunRecord, bool, error)
	GetAuditRun(context.Context, string, string) (*AuditRunRecord, error)
}

// RunUsagePort delegates billing reads to the owning LLM domain. Missing
// receipts must fail rather than be represented as a free invocation.
type RunUsagePort interface {
	SumReceiptCostMicros(context.Context, string, []string) (int64, error)
}
