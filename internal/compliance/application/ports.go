package application

import (
	d "InkLore/internal/compliance/domain"
	"InkLore/internal/platform/scope"
	"context"
)

type UnitOfWork interface {
	Within(context.Context, func(context.Context) error) error
	WithinReadSnapshot(context.Context, func(context.Context) error) error
}

type ScopePort interface {
	Assert(context.Context, scope.Principal, scope.Resource, scope.Action) (scope.Scope, error)
	AssertChapterSnapshot(context.Context, scope.Principal, string, int64, string) (scope.SnapshotProof, error)
}

type OutboxPort interface {
	Publish(ctx context.Context, tenantID, producer, eventType, aggregateID string, revision int64, payload map[string]any) error
}

type AuditLLMResult struct {
	Issues     []d.Issue
	Summary    string
	UsedTokens int
	CallsUsed  int
}

type RewriteLLMResult struct {
	Patches    []d.Patch
	UsedTokens int
	CallsUsed  int
}

type GenerationPort interface {
	GenerateAudit(ctx context.Context, tenantID, reportID, novelID, chapterID, text string, budget d.Budget) (AuditLLMResult, error)
	GenerateRewrite(ctx context.Context, tenantID, novelID, chapterID, text string, issues []d.Issue, round int, budget d.Budget) (RewriteLLMResult, error)
}

type RuleSnapshotCache interface {
	Get(tenantID, checksum string) (*d.Matcher, bool)
	Put(tenantID, checksum string, matcher *d.Matcher)
	Invalidate(tenantID string)
}

type ExportSnapshot struct {
	ChapterID       string `json:"chapter_id"`
	ChapterRevision int64  `json:"chapter_revision"`
	ContentHash     string `json:"content_hash"`
}

type ValidatedReviewReceipt struct {
	ChapterID       string `json:"chapter_id"`
	ChapterRevision int64  `json:"chapter_revision"`
	ContentHash     string `json:"content_hash"`
	ReviewID        string `json:"review_id"`
}

type ValidatedReviews struct {
	Receipts []ValidatedReviewReceipt `json:"receipts"`
}

type ExecutionInput struct {
	TenantID         string
	ResourceID       string
	RunID            string
	ExecutionKey     string
	Fence            int64
	ExpectedRevision int64
}
