package domain

import (
	"context"
)

type RuleSetRepository interface {
	CreateRuleSet(ctx context.Context, set RuleSet) error
	GetRuleSet(ctx context.Context, tenantID, id string) (RuleSet, error)
	GetRuleSetForUpdate(ctx context.Context, tenantID, id string) (RuleSet, error)
	GetCurrentPublishedRuleSet(ctx context.Context, tenantID string) (RuleSet, error)
	ListRuleSets(ctx context.Context, tenantID string, query ListQuery) ([]RuleSet, bool, error)
	PublishRuleSet(ctx context.Context, tenantID, id, checksum string, expectedRevision int64) (RuleSet, error)
	GetNextVersionNo(ctx context.Context, tenantID string) (int64, error)
}

type RuleRepository interface {
	CreateRule(ctx context.Context, rule Rule) error
	GetRule(ctx context.Context, tenantID, ruleSetID, id string) (Rule, error)
	GetRuleForUpdate(ctx context.Context, tenantID, ruleSetID, id string) (Rule, error)
	UpdateRule(ctx context.Context, rule Rule, expectedRevision int64) error
	DeleteRule(ctx context.Context, tenantID, ruleSetID, id string, expectedRevision int64) error
	ListRulesBySet(ctx context.Context, tenantID, ruleSetID string, query ListQuery) ([]Rule, bool, error)
	GetAllRulesBySet(ctx context.Context, tenantID, ruleSetID string) ([]Rule, error)
	CountRulesBySet(ctx context.Context, tenantID, ruleSetID string) (int, int, error)
	ExistsRule(ctx context.Context, tenantID, ruleSetID, kind, expression string, excludeRuleID string) (bool, error)
	CopyRules(ctx context.Context, tenantID, srcSetID, dstSetID string) error
}

type ReportRepository interface {
	CreateReport(ctx context.Context, report Report) error
	GetReport(ctx context.Context, tenantID, id string) (Report, error)
	GetReportForUpdate(ctx context.Context, tenantID, id string) (Report, error)
	UpdateReport(ctx context.Context, report Report) error
	ListReports(ctx context.Context, tenantID, ownerID string, query ListQuery) ([]Report, bool, error)
	ClaimReportExecution(ctx context.Context, tenantID, reportID, runID, executionKey string, fence int64) (Report, error)
}

type IssueRepository interface {
	BatchCreateIssues(ctx context.Context, issues []Issue) error
	GetIssue(ctx context.Context, tenantID, reportID, id string) (Issue, error)
	GetIssueForUpdate(ctx context.Context, tenantID, reportID, id string) (Issue, error)
	UpdateIssue(ctx context.Context, issue Issue, expectedRevision int64) error
	ListIssuesByReport(ctx context.Context, tenantID, reportID string, query ListQuery) ([]Issue, bool, error)
	GetIssuesByIDs(ctx context.Context, tenantID, reportID string, issueIDs []string) ([]Issue, error)
}

type DecisionRepository interface {
	CreateDecision(ctx context.Context, decision IssueDecision) error
	ListDecisionsByIssue(ctx context.Context, tenantID, reportID, issueID string, query ListQuery) ([]IssueDecision, bool, error)
}

type ReviewRepository interface {
	CreateReview(ctx context.Context, review Review) error
	GetReview(ctx context.Context, tenantID, id string) (Review, error)
	ListReviewsByReport(ctx context.Context, tenantID, reportID string, query ListQuery) ([]Review, bool, error)
	GetReviewsByIDs(ctx context.Context, tenantID string, reviewIDs []string) ([]Review, error)
}

type RewriteRepository interface {
	CreateRewrite(ctx context.Context, rewrite Rewrite) error
	GetRewrite(ctx context.Context, tenantID, id string) (Rewrite, error)
	GetRewriteForUpdate(ctx context.Context, tenantID, id string) (Rewrite, error)
	UpdateRewrite(ctx context.Context, rewrite Rewrite) error
	ListRewrites(ctx context.Context, tenantID, ownerID string, query ListQuery) ([]Rewrite, bool, error)
	ClaimRewriteExecution(ctx context.Context, tenantID, rewriteID, runID, executionKey string, fence int64) (Rewrite, error)
}

type Repositories struct {
	RuleSets  RuleSetRepository
	Rules     RuleRepository
	Reports   ReportRepository
	Issues    IssueRepository
	Decisions DecisionRepository
	Reviews   ReviewRepository
	Rewrites  RewriteRepository
}
