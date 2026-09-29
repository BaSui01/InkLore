package infrastructure

import (
	d "InkLore/internal/compliance/domain"
	"context"
	"encoding/json"
	"time"
)

type reportRow struct {
	ID              string     `gorm:"column:id"`
	TenantID        string     `gorm:"column:tenant_id"`
	CreatedAt       time.Time  `gorm:"column:created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at"`
	Revision        int64      `gorm:"column:revision"`
	OwnerUserID     string     `gorm:"column:owner_user_id"`
	NovelID         string     `gorm:"column:novel_id"`
	ChapterID       string     `gorm:"column:chapter_id"`
	ChapterRevision int64      `gorm:"column:chapter_revision"`
	ContentHash     string     `gorm:"column:content_hash"`
	SourceText      string     `gorm:"column:source_text"`
	RuleSetID       string     `gorm:"column:rule_set_id"`
	Mode            string     `gorm:"column:mode"`
	State           string     `gorm:"column:state"`
	RunID           *string    `gorm:"column:run_id"`
	ExecutionKey    *string    `gorm:"column:execution_key"`
	ExecutionFence  int64      `gorm:"column:execution_fence"`
	TotalTokens     int        `gorm:"column:total_tokens"`
	MaxOutputTokens int        `gorm:"column:max_output_tokens"`
	MaxCalls        int        `gorm:"column:max_calls"`
	UsedTokens      int        `gorm:"column:used_tokens"`
	ReservedTokens  int        `gorm:"column:reserved_tokens"`
	CallsUsed       int        `gorm:"column:calls_used"`
	Summary         string     `gorm:"column:summary"`
	AIFlavorScore   *int       `gorm:"column:ai_flavor_score"`
	FormulaVersion  string     `gorm:"column:formula_version"`
	RiskCount       int        `gorm:"column:risk_count"`
	OpenCount       int        `gorm:"column:open_count"`
	BlockCount      int        `gorm:"column:block_count"`
	FailureCode     *string    `gorm:"column:failure_code"`
	ExpireAt        time.Time  `gorm:"column:expire_at"`
	RedactedAt      *time.Time `gorm:"column:redacted_at"`
}

func (row reportRow) toDomain() d.Report {
	return d.Report{
		ID:              row.ID,
		TenantID:        row.TenantID,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
		Revision:        row.Revision,
		OwnerUserID:     row.OwnerUserID,
		NovelID:         row.NovelID,
		ChapterID:       row.ChapterID,
		ChapterRevision: row.ChapterRevision,
		ContentHash:     row.ContentHash,
		SourceText:      row.SourceText,
		RuleSetID:       row.RuleSetID,
		Mode:            row.Mode,
		State:           row.State,
		RunID:           row.RunID,
		ExecutionKey:    row.ExecutionKey,
		ExecutionFence:  row.ExecutionFence,
		Budget: d.Budget{
			TotalTokens:     row.TotalTokens,
			MaxOutputTokens: row.MaxOutputTokens,
			MaxCalls:        row.MaxCalls,
		},
		Usage: d.Usage{
			UsedTokens:     row.UsedTokens,
			ReservedTokens: row.ReservedTokens,
			CallsUsed:      row.CallsUsed,
		},
		Summary:        row.Summary,
		AIFlavorScore:  row.AIFlavorScore,
		FormulaVersion: row.FormulaVersion,
		RiskCount:      row.RiskCount,
		OpenCount:      row.OpenCount,
		BlockCount:     row.BlockCount,
		FailureCode:    row.FailureCode,
		ExpireAt:       row.ExpireAt,
		RedactedAt:     row.RedactedAt,
	}
}

func (r *Repository) CreateReport(ctx context.Context, report d.Report) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}
	query := "INSERT INTO compliance_reports(id, tenant_id, created_at, updated_at, revision, owner_user_id, novel_id, chapter_id, chapter_revision, content_hash, source_text, rule_set_id, mode, state, run_id, execution_key, execution_fence, total_tokens, max_output_tokens, max_calls, used_tokens, reserved_tokens, calls_used, summary, ai_flavor_score, formula_version, risk_count, open_count, block_count, failure_code, expire_at, redacted_at) " +
		"VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	return db.Exec(query,
		report.ID, report.TenantID, report.CreatedAt, report.UpdatedAt, report.Revision,
		report.OwnerUserID, report.NovelID, report.ChapterID, report.ChapterRevision, report.ContentHash,
		report.SourceText, report.RuleSetID, report.Mode, report.State, report.RunID,
		report.ExecutionKey, report.ExecutionFence,
		report.Budget.TotalTokens, report.Budget.MaxOutputTokens, report.Budget.MaxCalls,
		report.Usage.UsedTokens, report.Usage.ReservedTokens, report.Usage.CallsUsed,
		report.Summary, report.AIFlavorScore, report.FormulaVersion,
		report.RiskCount, report.OpenCount, report.BlockCount,
		report.FailureCode, report.ExpireAt, report.RedactedAt,
	).Error
}

func (r *Repository) GetReport(ctx context.Context, tenantID, id string) (d.Report, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return d.Report{}, err
	}
	var row reportRow
	res := db.Raw("SELECT * FROM compliance_reports WHERE tenant_id = ? AND id = ?", tenantID, id).Scan(&row)
	if res.Error != nil {
		return d.Report{}, res.Error
	}
	if res.RowsAffected == 0 {
		return d.Report{}, missing()
	}
	return row.toDomain(), nil
}

func (r *Repository) GetReportForUpdate(ctx context.Context, tenantID, id string) (d.Report, error) {
	db, err := r.db(ctx, true)
	if err != nil {
		return d.Report{}, err
	}
	var row reportRow
	res := db.Raw("SELECT * FROM compliance_reports WHERE tenant_id = ? AND id = ? FOR UPDATE", tenantID, id).Scan(&row)
	if res.Error != nil {
		return d.Report{}, res.Error
	}
	if res.RowsAffected == 0 {
		return d.Report{}, missing()
	}
	return row.toDomain(), nil
}

func (r *Repository) UpdateReport(ctx context.Context, report d.Report) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}
	query := "UPDATE compliance_reports SET updated_at = ?, revision = ?, state = ?, run_id = ?, execution_key = ?, execution_fence = ?, used_tokens = ?, reserved_tokens = ?, calls_used = ?, summary = ?, ai_flavor_score = ?, risk_count = ?, open_count = ?, block_count = ?, failure_code = ?, redacted_at = ? WHERE tenant_id = ? AND id = ?"
	res := db.Exec(query,
		report.UpdatedAt, report.Revision, report.State, report.RunID,
		report.ExecutionKey, report.ExecutionFence,
		report.Usage.UsedTokens, report.Usage.ReservedTokens, report.Usage.CallsUsed,
		report.Summary, report.AIFlavorScore, report.RiskCount, report.OpenCount, report.BlockCount,
		report.FailureCode, report.RedactedAt, report.TenantID, report.ID,
	)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return missing()
	}
	return nil
}

func (r *Repository) ListReports(ctx context.Context, tenantID, ownerID string, query d.ListQuery) ([]d.Report, bool, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, false, err
	}

	q := "SELECT * FROM compliance_reports WHERE tenant_id = ? AND owner_user_id = ?"
	args := []any{tenantID, ownerID}

	if query.ChapterID != "" {
		q += " AND chapter_id = ?"
		args = append(args, query.ChapterID)
	}
	if query.State != "" {
		q += " AND state = ?"
		args = append(args, query.State)
	}

	if !query.BeforeAt.IsZero() && query.BeforeID != "" {
		q += " AND (updated_at < ? OR (updated_at = ? AND id < ?))"
		args = append(args, query.BeforeAt, query.BeforeAt, query.BeforeID)
	}

	q += " ORDER BY updated_at DESC, id DESC LIMIT ?"
	limit := query.Limit
	if limit <= 0 {
		limit = 20
	}
	args = append(args, limit+1)

	var rows []reportRow
	if err := db.Raw(q, args...).Scan(&rows).Error; err != nil {
		return nil, false, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	reports := make([]d.Report, len(rows))
	for i, row := range rows {
		reports[i] = row.toDomain()
	}
	return reports, hasMore, nil
}

func (r *Repository) ClaimReportExecution(ctx context.Context, tenantID, reportID, runID, executionKey string, fence int64) (d.Report, error) {
	db, err := r.db(ctx, true)
	if err != nil {
		return d.Report{}, err
	}

	var row reportRow
	res := db.Raw("SELECT * FROM compliance_reports WHERE tenant_id = ? AND id = ? FOR UPDATE", tenantID, reportID).Scan(&row)
	if res.Error != nil {
		return d.Report{}, res.Error
	}
	if res.RowsAffected == 0 {
		return d.Report{}, missing()
	}

	now := time.Now().UTC()
	row.State = d.ReportStateRunning
	row.RunID = &runID
	row.ExecutionKey = &executionKey
	row.ExecutionFence = fence
	row.Revision++
	row.UpdatedAt = now

	upd := db.Exec("UPDATE compliance_reports SET state = ?, run_id = ?, execution_key = ?, execution_fence = ?, revision = ?, updated_at = ? WHERE tenant_id = ? AND id = ?",
		row.State, row.RunID, row.ExecutionKey, row.ExecutionFence, row.Revision, row.UpdatedAt, tenantID, reportID)
	if upd.Error != nil {
		return d.Report{}, upd.Error
	}

	return row.toDomain(), nil
}

// IssueRepository implementation

func (r *Repository) BatchCreateIssues(ctx context.Context, issues []d.Issue) error {
	if len(issues) == 0 {
		return nil
	}
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}

	for _, iss := range issues {
		now := iss.CreatedAt
		if now.IsZero() {
			now = time.Now().UTC()
		}
		query := "INSERT INTO compliance_issues(id, tenant_id, created_at, updated_at, revision, report_id, rule_id, detector, kind, severity, start_offset, end_offset, excerpt, excerpt_truncated, message, suggestion, confidence, state) " +
			"VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) " +
			"ON CONFLICT (tenant_id, report_id, rule_id, start_offset, end_offset) WHERE rule_id IS NOT NULL DO NOTHING"
		if err := db.Exec(query,
			iss.ID, iss.TenantID, now, now, iss.Revision,
			iss.ReportID, iss.RuleID, iss.Detector, iss.Kind, iss.Severity,
			iss.StartOffset, iss.EndOffset, iss.Excerpt, iss.ExcerptTruncated,
			iss.Message, iss.Suggestion, iss.Confidence, iss.State,
		).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) GetIssue(ctx context.Context, tenantID, reportID, id string) (d.Issue, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return d.Issue{}, err
	}
	var issue d.Issue
	res := db.Raw("SELECT * FROM compliance_issues WHERE tenant_id = ? AND report_id = ? AND id = ?", tenantID, reportID, id).Scan(&issue)
	if res.Error != nil {
		return d.Issue{}, res.Error
	}
	if res.RowsAffected == 0 {
		return d.Issue{}, missing()
	}
	return issue, nil
}

func (r *Repository) GetIssueForUpdate(ctx context.Context, tenantID, reportID, id string) (d.Issue, error) {
	db, err := r.db(ctx, true)
	if err != nil {
		return d.Issue{}, err
	}
	var issue d.Issue
	res := db.Raw("SELECT * FROM compliance_issues WHERE tenant_id = ? AND report_id = ? AND id = ? FOR UPDATE", tenantID, reportID, id).Scan(&issue)
	if res.Error != nil {
		return d.Issue{}, res.Error
	}
	if res.RowsAffected == 0 {
		return d.Issue{}, missing()
	}
	return issue, nil
}

func (r *Repository) UpdateIssue(ctx context.Context, issue d.Issue, expectedRevision int64) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}
	res := db.Exec("UPDATE compliance_issues SET updated_at = ?, revision = ?, state = ? WHERE tenant_id = ? AND report_id = ? AND id = ? AND revision = ?",
		issue.UpdatedAt, issue.Revision, issue.State, issue.TenantID, issue.ReportID, issue.ID, expectedRevision)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return versionConflict(expectedRevision)
	}
	return nil
}

func (r *Repository) ListIssuesByReport(ctx context.Context, tenantID, reportID string, query d.ListQuery) ([]d.Issue, bool, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, false, err
	}

	q := "SELECT * FROM compliance_issues WHERE tenant_id = ? AND report_id = ?"
	args := []any{tenantID, reportID}

	if query.State != "" {
		q += " AND state = ?"
		args = append(args, query.State)
	}

	if !query.BeforeAt.IsZero() && query.BeforeID != "" {
		q += " AND (updated_at < ? OR (updated_at = ? AND id < ?))"
		args = append(args, query.BeforeAt, query.BeforeAt, query.BeforeID)
	}

	q += " ORDER BY updated_at DESC, id DESC LIMIT ?"
	limit := query.Limit
	if limit <= 0 {
		limit = 20
	}
	args = append(args, limit+1)

	var items []d.Issue
	if err := db.Raw(q, args...).Scan(&items).Error; err != nil {
		return nil, false, err
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	return items, hasMore, nil
}

func (r *Repository) GetIssuesByIDs(ctx context.Context, tenantID, reportID string, issueIDs []string) ([]d.Issue, error) {
	if len(issueIDs) == 0 {
		return nil, nil
	}
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, err
	}

	var items []d.Issue
	err = db.Raw("SELECT * FROM compliance_issues WHERE tenant_id = ? AND report_id = ? AND id IN (?)", tenantID, reportID, issueIDs).Scan(&items).Error
	return items, err
}

// DecisionRepository implementation

func (r *Repository) CreateDecision(ctx context.Context, decision d.IssueDecision) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}
	query := "INSERT INTO compliance_issue_decisions(id, tenant_id, created_at, updated_at, report_id, issue_id, actor_user_id, action, reason, issue_revision) " +
		"VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	return db.Exec(query,
		decision.ID, decision.TenantID, decision.CreatedAt, decision.UpdatedAt,
		decision.ReportID, decision.IssueID, decision.ActorUserID,
		decision.Action, decision.Reason, decision.IssueRevision,
	).Error
}

func (r *Repository) ListDecisionsByIssue(ctx context.Context, tenantID, reportID, issueID string, query d.ListQuery) ([]d.IssueDecision, bool, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, false, err
	}

	q := "SELECT * FROM compliance_issue_decisions WHERE tenant_id = ? AND report_id = ? AND issue_id = ?"
	args := []any{tenantID, reportID, issueID}

	if !query.BeforeAt.IsZero() && query.BeforeID != "" {
		q += " AND (created_at < ? OR (created_at = ? AND id < ?))"
		args = append(args, query.BeforeAt, query.BeforeAt, query.BeforeID)
	}

	q += " ORDER BY created_at DESC, id DESC LIMIT ?"
	limit := query.Limit
	if limit <= 0 {
		limit = 20
	}
	args = append(args, limit+1)

	var items []d.IssueDecision
	if err := db.Raw(q, args...).Scan(&items).Error; err != nil {
		return nil, false, err
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	return items, hasMore, nil
}

// ReviewRepository implementation

func (r *Repository) CreateReview(ctx context.Context, review d.Review) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}
	query := "INSERT INTO compliance_reviews(id, tenant_id, created_at, updated_at, report_id, report_revision, chapter_id, chapter_revision, content_hash, rule_set_id, actor_user_id, acknowledged_risk, note) " +
		"VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	return db.Exec(query,
		review.ID, review.TenantID, review.CreatedAt, review.UpdatedAt,
		review.ReportID, review.ReportRevision, review.ChapterID, review.ChapterRevision,
		review.ContentHash, review.RuleSetID, review.ActorUserID,
		review.AcknowledgedRisk, review.Note,
	).Error
}

func (r *Repository) GetReview(ctx context.Context, tenantID, id string) (d.Review, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return d.Review{}, err
	}
	var rev d.Review
	res := db.Raw("SELECT * FROM compliance_reviews WHERE tenant_id = ? AND id = ?", tenantID, id).Scan(&rev)
	if res.Error != nil {
		return d.Review{}, res.Error
	}
	if res.RowsAffected == 0 {
		return d.Review{}, missing()
	}
	return rev, nil
}

func (r *Repository) ListReviewsByReport(ctx context.Context, tenantID, reportID string, query d.ListQuery) ([]d.Review, bool, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, false, err
	}

	q := "SELECT * FROM compliance_reviews WHERE tenant_id = ? AND report_id = ?"
	args := []any{tenantID, reportID}

	if !query.BeforeAt.IsZero() && query.BeforeID != "" {
		q += " AND (created_at < ? OR (created_at = ? AND id < ?))"
		args = append(args, query.BeforeAt, query.BeforeAt, query.BeforeID)
	}

	q += " ORDER BY created_at DESC, id DESC LIMIT ?"
	limit := query.Limit
	if limit <= 0 {
		limit = 20
	}
	args = append(args, limit+1)

	var items []d.Review
	if err := db.Raw(q, args...).Scan(&items).Error; err != nil {
		return nil, false, err
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	return items, hasMore, nil
}

func (r *Repository) GetReviewsByIDs(ctx context.Context, tenantID string, reviewIDs []string) ([]d.Review, error) {
	if len(reviewIDs) == 0 {
		return nil, nil
	}
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, err
	}
	var items []d.Review
	err = db.Raw("SELECT * FROM compliance_reviews WHERE tenant_id = ? AND id IN (?)", tenantID, reviewIDs).Scan(&items).Error
	return items, err
}

// RewriteRepository implementation

type rewriteRow struct {
	ID                   string     `gorm:"column:id"`
	TenantID             string     `gorm:"column:tenant_id"`
	CreatedAt            time.Time  `gorm:"column:created_at"`
	UpdatedAt            time.Time  `gorm:"column:updated_at"`
	Revision             int64      `gorm:"column:revision"`
	OwnerUserID          string     `gorm:"column:owner_user_id"`
	ReportID             string     `gorm:"column:report_id"`
	ReportRevision       int64      `gorm:"column:report_revision"`
	SelectedIssueIDsJSON string     `gorm:"column:selected_issue_ids_json"`
	State                string     `gorm:"column:state"`
	MaxRounds            int        `gorm:"column:max_rounds"`
	RoundsUsed           int        `gorm:"column:rounds_used"`
	TotalTokens          int        `gorm:"column:total_tokens"`
	MaxOutputTokens      int        `gorm:"column:max_output_tokens"`
	MaxCalls             int        `gorm:"column:max_calls"`
	UsedTokens           int        `gorm:"column:used_tokens"`
	ReservedTokens       int        `gorm:"column:reserved_tokens"`
	CallsUsed            int        `gorm:"column:calls_used"`
	RunID                *string    `gorm:"column:run_id"`
	ExecutionKey         *string    `gorm:"column:execution_key"`
	ExecutionFence       int64      `gorm:"column:execution_fence"`
	PatchesJSON          string     `gorm:"column:patches_json"`
	CandidateText        *string    `gorm:"column:candidate_text"`
	CandidateHash        *string    `gorm:"column:candidate_hash"`
	RemainingRiskCount   *int       `gorm:"column:remaining_risk_count"`
	StopReason           *string    `gorm:"column:stop_reason"`
	FailureCode          *string    `gorm:"column:failure_code"`
	RedactedAt           *time.Time `gorm:"column:redacted_at"`
}

func (row rewriteRow) toDomain() (d.Rewrite, error) {
	var selectedIDs []string
	if row.SelectedIssueIDsJSON != "" {
		if err := json.Unmarshal([]byte(row.SelectedIssueIDsJSON), &selectedIDs); err != nil {
			return d.Rewrite{}, err
		}
	}
	var patches []d.Patch
	if row.PatchesJSON != "" {
		if err := json.Unmarshal([]byte(row.PatchesJSON), &patches); err != nil {
			return d.Rewrite{}, err
		}
	}
	if patches == nil {
		patches = []d.Patch{}
	}

	return d.Rewrite{
		ID:               row.ID,
		TenantID:         row.TenantID,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
		Revision:         row.Revision,
		OwnerUserID:      row.OwnerUserID,
		ReportID:         row.ReportID,
		ReportRevision:   row.ReportRevision,
		SelectedIssueIDs: selectedIDs,
		State:            row.State,
		MaxRounds:        row.MaxRounds,
		RoundsUsed:       row.RoundsUsed,
		Budget: d.Budget{
			TotalTokens:     row.TotalTokens,
			MaxOutputTokens: row.MaxOutputTokens,
			MaxCalls:        row.MaxCalls,
		},
		Usage: d.Usage{
			UsedTokens:     row.UsedTokens,
			ReservedTokens: row.ReservedTokens,
			CallsUsed:      row.CallsUsed,
		},
		RunID:              row.RunID,
		ExecutionKey:       row.ExecutionKey,
		ExecutionFence:     row.ExecutionFence,
		Patches:            patches,
		CandidateText:      row.CandidateText,
		CandidateHash:      row.CandidateHash,
		RemainingRiskCount: row.RemainingRiskCount,
		StopReason:         row.StopReason,
		FailureCode:        row.FailureCode,
		RedactedAt:         row.RedactedAt,
	}, nil
}

func (r *Repository) CreateRewrite(ctx context.Context, rewrite d.Rewrite) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}

	selectedJSON, err := json.Marshal(rewrite.SelectedIssueIDs)
	if err != nil {
		return err
	}
	patchesJSON, err := json.Marshal(rewrite.Patches)
	if err != nil {
		return err
	}

	query := "INSERT INTO compliance_rewrites(id, tenant_id, created_at, updated_at, revision, owner_user_id, report_id, report_revision, selected_issue_ids, state, max_rounds, rounds_used, total_tokens, max_output_tokens, max_calls, used_tokens, reserved_tokens, calls_used, run_id, execution_key, execution_fence, patches, candidate_text, candidate_hash, remaining_risk_count, stop_reason, failure_code, redacted_at) " +
		"VALUES(?, ?, ?, ?, ?, ?, ?, ?, ARRAY(SELECT jsonb_array_elements_text(?::jsonb)::uuid), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?::jsonb, ?, ?, ?, ?, ?, ?)"
	return db.Exec(query,
		rewrite.ID, rewrite.TenantID, rewrite.CreatedAt, rewrite.UpdatedAt, rewrite.Revision,
		rewrite.OwnerUserID, rewrite.ReportID, rewrite.ReportRevision, string(selectedJSON),
		rewrite.State, rewrite.MaxRounds, rewrite.RoundsUsed,
		rewrite.Budget.TotalTokens, rewrite.Budget.MaxOutputTokens, rewrite.Budget.MaxCalls,
		rewrite.Usage.UsedTokens, rewrite.Usage.ReservedTokens, rewrite.Usage.CallsUsed,
		rewrite.RunID, rewrite.ExecutionKey, rewrite.ExecutionFence, string(patchesJSON),
		rewrite.CandidateText, rewrite.CandidateHash, rewrite.RemainingRiskCount,
		rewrite.StopReason, rewrite.FailureCode, rewrite.RedactedAt,
	).Error
}

func (r *Repository) GetRewrite(ctx context.Context, tenantID, id string) (d.Rewrite, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return d.Rewrite{}, err
	}
	var row rewriteRow
	query := "SELECT r.*, array_to_json(r.selected_issue_ids)::text AS selected_issue_ids_json, r.patches::text AS patches_json FROM compliance_rewrites r WHERE r.tenant_id = ? AND r.id = ?"
	res := db.Raw(query, tenantID, id).Scan(&row)
	if res.Error != nil {
		return d.Rewrite{}, res.Error
	}
	if res.RowsAffected == 0 {
		return d.Rewrite{}, missing()
	}
	return row.toDomain()
}

func (r *Repository) GetRewriteForUpdate(ctx context.Context, tenantID, id string) (d.Rewrite, error) {
	db, err := r.db(ctx, true)
	if err != nil {
		return d.Rewrite{}, err
	}
	var row rewriteRow
	query := "SELECT r.*, array_to_json(r.selected_issue_ids)::text AS selected_issue_ids_json, r.patches::text AS patches_json FROM compliance_rewrites r WHERE r.tenant_id = ? AND r.id = ? FOR UPDATE"
	res := db.Raw(query, tenantID, id).Scan(&row)
	if res.Error != nil {
		return d.Rewrite{}, res.Error
	}
	if res.RowsAffected == 0 {
		return d.Rewrite{}, missing()
	}
	return row.toDomain()
}

func (r *Repository) UpdateRewrite(ctx context.Context, rewrite d.Rewrite) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}
	patchesJSON, err := json.Marshal(rewrite.Patches)
	if err != nil {
		return err
	}

	query := "UPDATE compliance_rewrites SET updated_at = ?, revision = ?, state = ?, rounds_used = ?, used_tokens = ?, reserved_tokens = ?, calls_used = ?, patches = ?::jsonb, candidate_text = ?, candidate_hash = ?, remaining_risk_count = ?, stop_reason = ?, failure_code = ?, redacted_at = ? WHERE tenant_id = ? AND id = ?"
	res := db.Exec(query,
		rewrite.UpdatedAt, rewrite.Revision, rewrite.State, rewrite.RoundsUsed,
		rewrite.Usage.UsedTokens, rewrite.Usage.ReservedTokens, rewrite.Usage.CallsUsed,
		string(patchesJSON), rewrite.CandidateText, rewrite.CandidateHash,
		rewrite.RemainingRiskCount, rewrite.StopReason, rewrite.FailureCode,
		rewrite.RedactedAt, rewrite.TenantID, rewrite.ID,
	)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return missing()
	}
	return nil
}

func (r *Repository) ListRewrites(ctx context.Context, tenantID, ownerID string, query d.ListQuery) ([]d.Rewrite, bool, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, false, err
	}

	q := "SELECT r.*, array_to_json(r.selected_issue_ids)::text AS selected_issue_ids_json, r.patches::text AS patches_json FROM compliance_rewrites r WHERE r.tenant_id = ? AND r.owner_user_id = ?"
	args := []any{tenantID, ownerID}

	if query.ReportID != "" {
		q += " AND r.report_id = ?"
		args = append(args, query.ReportID)
	}

	if !query.BeforeAt.IsZero() && query.BeforeID != "" {
		q += " AND (r.updated_at < ? OR (r.updated_at = ? AND r.id < ?))"
		args = append(args, query.BeforeAt, query.BeforeAt, query.BeforeID)
	}

	q += " ORDER BY r.updated_at DESC, r.id DESC LIMIT ?"
	limit := query.Limit
	if limit <= 0 {
		limit = 20
	}
	args = append(args, limit+1)

	var rows []rewriteRow
	if err := db.Raw(q, args...).Scan(&rows).Error; err != nil {
		return nil, false, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	rewrites := make([]d.Rewrite, len(rows))
	for i, row := range rows {
		item, err := row.toDomain()
		if err != nil {
			return nil, false, err
		}
		rewrites[i] = item
	}
	return rewrites, hasMore, nil
}

func (r *Repository) ClaimRewriteExecution(ctx context.Context, tenantID, rewriteID, runID, executionKey string, fence int64) (d.Rewrite, error) {
	db, err := r.db(ctx, true)
	if err != nil {
		return d.Rewrite{}, err
	}

	var row rewriteRow
	query := "SELECT r.*, array_to_json(r.selected_issue_ids)::text AS selected_issue_ids_json, r.patches::text AS patches_json FROM compliance_rewrites r WHERE r.tenant_id = ? AND r.id = ? FOR UPDATE"
	res := db.Raw(query, tenantID, rewriteID).Scan(&row)
	if res.Error != nil {
		return d.Rewrite{}, res.Error
	}
	if res.RowsAffected == 0 {
		return d.Rewrite{}, missing()
	}

	now := time.Now().UTC()
	row.State = d.ReportStateRunning
	row.RunID = &runID
	row.ExecutionKey = &executionKey
	row.ExecutionFence = fence
	row.Revision++
	row.UpdatedAt = now

	upd := db.Exec("UPDATE compliance_rewrites SET state = ?, run_id = ?, execution_key = ?, execution_fence = ?, revision = ?, updated_at = ? WHERE tenant_id = ? AND id = ?",
		row.State, row.RunID, row.ExecutionKey, row.ExecutionFence, row.Revision, row.UpdatedAt, tenantID, rewriteID)
	if upd.Error != nil {
		return d.Rewrite{}, upd.Error
	}

	return row.toDomain()
}
