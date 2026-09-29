package infrastructure

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	d "InkLore/internal/agent/domain"
	"InkLore/internal/platform/uow"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// RunRepositoryPostgres implements d.RunRepository for PostgreSQL.
type RunRepositoryPostgres struct {
	manager *uow.Manager
}

// NewRunRepository constructs a RunRepositoryPostgres instance.
func NewRunRepository(manager *uow.Manager) (*RunRepositoryPostgres, error) {
	if manager == nil {
		return nil, errors.New("agent run repository: uow manager required")
	}
	return &RunRepositoryPostgres{manager: manager}, nil
}

func (r *RunRepositoryPostgres) db(ctx context.Context, write bool) (*gorm.DB, error) {
	var db *gorm.DB
	var err error
	if write {
		db, err = r.manager.Require(ctx)
	} else {
		db, err = r.manager.DB(ctx)
	}
	if err != nil {
		return nil, err
	}
	return db.Session(&gorm.Session{Logger: logger.Discard}), nil
}

// --- Runs ---

type runRow struct {
	ID                  string         `gorm:"column:id"`
	TenantID            string         `gorm:"column:tenant_id"`
	CreatedAt           time.Time      `gorm:"column:created_at"`
	UpdatedAt           time.Time      `gorm:"column:updated_at"`
	Revision            int64          `gorm:"column:revision"`
	OwnerUserID         string         `gorm:"column:owner_user_id"`
	PlatformJobID       string         `gorm:"column:platform_job_id"`
	Kind                string         `gorm:"column:kind"`
	TaskKind            string         `gorm:"column:task_kind"`
	DiagnosticProbeKey  sql.NullString `gorm:"column:diagnostic_probe_key"`
	TaskKey             string         `gorm:"column:task_key"`
	ResourceType        sql.NullString `gorm:"column:resource_type"`
	ResourceID          sql.NullString `gorm:"column:resource_id"`
	NovelID             sql.NullString `gorm:"column:novel_id"`
	ChapterID           sql.NullString `gorm:"column:chapter_id"`
	SourceRevision      sql.NullInt64  `gorm:"column:source_revision"`
	SourceHash          sql.NullString `gorm:"column:source_hash"`
	Instruction         string         `gorm:"column:instruction"`
	DefinitionVersionID sql.NullString `gorm:"column:definition_version_id"`
	WorkflowVersionID   sql.NullString `gorm:"column:workflow_version_id"`
	PolicyVersionID     string         `gorm:"column:policy_version_id"`
	Status              string         `gorm:"column:status"`
	NextSeq             int64          `gorm:"column:next_seq"`
	RetainedFromSeq     int64          `gorm:"column:retained_from_seq"`
	DeadlineAt          time.Time      `gorm:"column:deadline_at"`
	StartedAt           sql.NullTime   `gorm:"column:started_at"`
	FinishedAt          sql.NullTime   `gorm:"column:finished_at"`
	ErrorCode           sql.NullString `gorm:"column:error_code"`
	TerminalReason      sql.NullString `gorm:"column:terminal_reason"`
	Retryable           bool           `gorm:"column:retryable"`
	UsageReceiptIDs     TextArray      `gorm:"column:usage_receipt_ids;type:text[]"`
	DeletedAt           sql.NullTime   `gorm:"column:deleted_at"`
}

func (row *runRow) toDomain() *d.AgentRun {
	var probeKey *string
	if row.DiagnosticProbeKey.Valid {
		probeKey = &row.DiagnosticProbeKey.String
	}
	var resType *string
	if row.ResourceType.Valid {
		resType = &row.ResourceType.String
	}
	var resID *string
	if row.ResourceID.Valid {
		resID = &row.ResourceID.String
	}
	var novelID *string
	if row.NovelID.Valid {
		novelID = &row.NovelID.String
	}
	var chapID *string
	if row.ChapterID.Valid {
		chapID = &row.ChapterID.String
	}
	var sourceRev *int64
	if row.SourceRevision.Valid {
		sourceRev = &row.SourceRevision.Int64
	}
	var sourceHash *string
	if row.SourceHash.Valid {
		sourceHash = &row.SourceHash.String
	}
	var defVerID *string
	if row.DefinitionVersionID.Valid {
		defVerID = &row.DefinitionVersionID.String
	}
	var wfVerID *string
	if row.WorkflowVersionID.Valid {
		wfVerID = &row.WorkflowVersionID.String
	}
	var startedAt *time.Time
	if row.StartedAt.Valid {
		startedAt = &row.StartedAt.Time
	}
	var finishedAt *time.Time
	if row.FinishedAt.Valid {
		finishedAt = &row.FinishedAt.Time
	}
	var errCode *string
	if row.ErrorCode.Valid {
		errCode = &row.ErrorCode.String
	}
	var termReason *string
	if row.TerminalReason.Valid {
		termReason = &row.TerminalReason.String
	}
	var deletedAt *time.Time
	if row.DeletedAt.Valid {
		deletedAt = &row.DeletedAt.Time
	}

	return &d.AgentRun{
		ID:                  row.ID,
		TenantID:            row.TenantID,
		CreatedAt:           row.CreatedAt,
		UpdatedAt:           row.UpdatedAt,
		Revision:            row.Revision,
		OwnerUserID:         row.OwnerUserID,
		PlatformJobID:       row.PlatformJobID,
		Kind:                row.Kind,
		TaskKind:            row.TaskKind,
		DiagnosticProbeKey:  probeKey,
		TaskKey:             row.TaskKey,
		ResourceType:        resType,
		ResourceID:          resID,
		NovelID:             novelID,
		ChapterID:           chapID,
		SourceRevision:      sourceRev,
		SourceHash:          sourceHash,
		Instruction:         row.Instruction,
		DefinitionVersionID: defVerID,
		WorkflowVersionID:   wfVerID,
		PolicyVersionID:     row.PolicyVersionID,
		Status:              d.RunStatus(row.Status),
		NextSeq:             row.NextSeq,
		RetainedFromSeq:     row.RetainedFromSeq,
		DeadlineAt:          row.DeadlineAt,
		StartedAt:           startedAt,
		FinishedAt:          finishedAt,
		ErrorCode:           errCode,
		TerminalReason:      termReason,
		Retryable:           row.Retryable,
		UsageReceiptIDs:     []string(row.UsageReceiptIDs),
		DeletedAt:           deletedAt,
	}
}

func (r *RunRepositoryPostgres) CreateRun(ctx context.Context, tenantID string, run *d.AgentRun, nodes []d.AgentRunNode) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}

	receipts := TextArray(run.UsageReceiptIDs)
	if receipts == nil {
		receipts = TextArray{}
	}

	q := db.Exec(`
		INSERT INTO agent_runs (
			id, tenant_id, created_at, updated_at, revision, owner_user_id, platform_job_id,
			kind, task_kind, diagnostic_probe_key, task_key, resource_type, resource_id,
			novel_id, chapter_id, source_revision, source_hash, instruction,
			definition_version_id, workflow_version_id, policy_version_id, status,
			next_seq, retained_from_seq, deadline_at, started_at, finished_at,
			error_code, terminal_reason, retryable, usage_receipt_ids
		) VALUES (
			?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?, ?
		)
	`,
		run.ID, tenantID, run.CreatedAt, run.UpdatedAt, run.Revision, run.OwnerUserID, run.PlatformJobID,
		run.Kind, run.TaskKind, run.DiagnosticProbeKey, run.TaskKey, run.ResourceType, run.ResourceID,
		run.NovelID, run.ChapterID, run.SourceRevision, run.SourceHash, run.Instruction,
		run.DefinitionVersionID, run.WorkflowVersionID, run.PolicyVersionID, string(run.Status),
		run.NextSeq, run.RetainedFromSeq, run.DeadlineAt, run.StartedAt, run.FinishedAt,
		run.ErrorCode, run.TerminalReason, run.Retryable, receipts,
	)
	if q.Error != nil {
		return q.Error
	}

	for _, n := range nodes {
		deps := TextArray(n.DependsOn)
		if deps == nil {
			deps = TextArray{}
		}
		nq := db.Exec(`
			INSERT INTO agent_run_nodes (
				id, tenant_id, created_at, updated_at, revision, run_id, node_key,
				role, depends_on, required, definition_version_id, tool_version_id,
				status, phase, iteration, attempt, lease_owner, lease_until,
				fencing_token, job_epoch, next_attempt_at, checkpoint, error_code,
				started_at, finished_at
			) VALUES (
				?, ?, ?, ?, ?, ?, ?,
				?, ?, ?, ?, ?,
				?, ?, ?, ?, ?, ?,
				?, ?, ?, ?::jsonb, ?,
				?, ?
			)
		`,
			n.ID, tenantID, n.CreatedAt, n.UpdatedAt, n.Revision, n.RunID, n.NodeKey,
			string(n.Role), deps, n.Required, n.DefinitionVersionID, n.ToolVersionID,
			string(n.Status), string(n.Phase), n.Iteration, n.Attempt, n.LeaseOwner, n.LeaseUntil,
			n.FencingToken, n.JobEpoch, n.NextAttemptAt, string(n.Checkpoint), n.ErrorCode,
			n.StartedAt, n.FinishedAt,
		)
		if nq.Error != nil {
			return nq.Error
		}
	}

	return nil
}

func (r *RunRepositoryPostgres) GetRun(ctx context.Context, tenantID, runID string) (*d.AgentRun, []d.AgentRunNode, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, nil, err
	}

	var row runRow
	q := db.Raw(`SELECT * FROM agent_runs WHERE tenant_id = ? AND id = ? AND deleted_at IS NULL`, tenantID, runID).Scan(&row)
	if q.Error != nil {
		return nil, nil, q.Error
	}
	if q.RowsAffected == 0 {
		return nil, nil, d.ErrNotFound()
	}

	nodes, err := r.ListNodes(ctx, tenantID, runID)
	if err != nil {
		return nil, nil, err
	}

	return row.toDomain(), nodes, nil
}

func (r *RunRepositoryPostgres) GetRunOnly(ctx context.Context, tenantID, runID string) (*d.AgentRun, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, err
	}

	var row runRow
	q := db.Raw(`SELECT * FROM agent_runs WHERE tenant_id = ? AND id = ? AND deleted_at IS NULL`, tenantID, runID).Scan(&row)
	if q.Error != nil {
		return nil, q.Error
	}
	if q.RowsAffected == 0 {
		return nil, d.ErrNotFound()
	}
	return row.toDomain(), nil
}

func (r *RunRepositoryPostgres) ListRunsByOwner(ctx context.Context, tenantID, ownerID, status string, query d.ListQuery) ([]d.RunSummaryView, *string, bool, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, nil, false, err
	}

	limit := query.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	qb := db.Table("agent_runs").
		Where("tenant_id = ? AND owner_user_id = ? AND deleted_at IS NULL", tenantID, ownerID)

	if status != "" {
		qb = qb.Where("status = ?", status)
	}

	if query.BeforeTime != "" && query.BeforeID != "" {
		qb = qb.Where("(updated_at, id) < (?, ?)", query.BeforeTime, query.BeforeID)
	}

	var rows []runRow
	if err := qb.Order("updated_at DESC, id DESC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, nil, false, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	views := make([]d.RunSummaryView, 0, len(rows))
	for _, row := range rows {
		dom := row.toDomain()
		var res *d.ResourceRef
		if dom.ResourceID != nil && dom.ResourceType != nil && dom.SourceRevision != nil {
			res = &d.ResourceRef{
				ResourceID:     *dom.ResourceID,
				ResourceType:   *dom.ResourceType,
				SourceRevision: *dom.SourceRevision,
				NovelID:        dom.NovelID,
				SourceHash:     dom.SourceHash,
			}
		}
		views = append(views, d.RunSummaryView{
			ID:        dom.ID,
			Revision:  dom.Revision,
			CreatedAt: dom.CreatedAt,
			UpdatedAt: dom.UpdatedAt,
			TaskKind:  dom.TaskKind,
			Status:    dom.Status,
			Resource:  res,
			ErrorCode: dom.ErrorCode,
		})
	}

	var nextCursor *string
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		c := fmt.Sprintf("%s_%s", last.UpdatedAt.UTC().Format(time.RFC3339Nano), last.ID)
		nextCursor = &c
	}

	return views, nextCursor, hasMore, nil
}

func (r *RunRepositoryPostgres) CountActiveRunsByOwner(ctx context.Context, tenantID, ownerID string) (int, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return 0, err
	}
	var count int64
	err = db.Raw(`
		SELECT COUNT(*) FROM agent_runs
		WHERE tenant_id = ? AND owner_user_id = ?
		  AND status NOT IN ('succeeded', 'failed', 'cancelled')
		  AND deleted_at IS NULL
	`, tenantID, ownerID).Scan(&count).Error
	return int(count), err
}

func (r *RunRepositoryPostgres) UpdateRunStatus(ctx context.Context, tenantID, runID string, expectedRev int64, newStatus d.RunStatus) (*d.AgentRun, error) {
	db, err := r.db(ctx, true)
	if err != nil {
		return nil, err
	}

	var row runRow
	q := db.Raw(`
		UPDATE agent_runs
		SET status = ?, revision = revision + 1, updated_at = now()
		WHERE tenant_id = ? AND id = ? AND revision = ? AND deleted_at IS NULL
		RETURNING *
	`, string(newStatus), tenantID, runID, expectedRev).Scan(&row)

	if q.Error != nil {
		return nil, q.Error
	}
	if q.RowsAffected == 0 {
		return nil, d.ErrVersionConflict(map[string]any{"expected_revision": expectedRev})
	}
	return row.toDomain(), nil
}

func (r *RunRepositoryPostgres) TerminateRun(
	ctx context.Context,
	tenantID, runID string,
	expectedRev int64,
	targetStatus d.RunStatus,
	reason, errorCode string,
	retryable bool,
	receiptIDs []string,
) (*d.AgentRun, error) {
	db, err := r.db(ctx, true)
	if err != nil {
		return nil, err
	}

	receipts := TextArray(receiptIDs)
	if receipts == nil {
		receipts = TextArray{}
	}

	var row runRow
	q := db.Raw(`
		UPDATE agent_runs
		SET status = ?, finished_at = now(), revision = revision + 1,
		    error_code = NULLIF(?, ''), terminal_reason = NULLIF(?, ''),
		    retryable = ?, usage_receipt_ids = ?, updated_at = now()
		WHERE tenant_id = ? AND id = ? AND revision = ?
		  AND status NOT IN ('succeeded', 'failed', 'cancelled')
		  AND deleted_at IS NULL
		RETURNING *
	`, string(targetStatus), errorCode, reason, retryable, receipts, tenantID, runID, expectedRev).Scan(&row)

	if q.Error != nil {
		return nil, q.Error
	}
	if q.RowsAffected == 0 {
		return nil, d.ErrVersionConflict(map[string]any{"expected_revision": expectedRev, "reason": "already terminal or version mismatch"})
	}
	return row.toDomain(), nil
}

func (r *RunRepositoryPostgres) AllocateNextSeq(ctx context.Context, tenantID, runID string) (int64, error) {
	db, err := r.db(ctx, true)
	if err != nil {
		return 0, err
	}
	var nextSeq int64
	q := db.Raw(`
		UPDATE agent_runs
		SET next_seq = next_seq + 1, updated_at = now()
		WHERE tenant_id = ? AND id = ?
		RETURNING next_seq - 1
	`, tenantID, runID).Scan(&nextSeq)
	if q.Error != nil {
		return 0, q.Error
	}
	if q.RowsAffected == 0 {
		return 0, d.ErrNotFound()
	}
	return nextSeq, nil
}

// --- Nodes ---

type nodeRow struct {
	ID                  string         `gorm:"column:id"`
	TenantID            string         `gorm:"column:tenant_id"`
	CreatedAt           time.Time      `gorm:"column:created_at"`
	UpdatedAt           time.Time      `gorm:"column:updated_at"`
	Revision            int64          `gorm:"column:revision"`
	RunID               string         `gorm:"column:run_id"`
	NodeKey             string         `gorm:"column:node_key"`
	Role                string         `gorm:"column:role"`
	DependsOn           TextArray      `gorm:"column:depends_on;type:text[]"`
	Required            bool           `gorm:"column:required"`
	DefinitionVersionID sql.NullString `gorm:"column:definition_version_id"`
	ToolVersionID       sql.NullString `gorm:"column:tool_version_id"`
	Status              string         `gorm:"column:status"`
	Phase               string         `gorm:"column:phase"`
	Iteration           int            `gorm:"column:iteration"`
	Attempt             int            `gorm:"column:attempt"`
	LeaseOwner          sql.NullString `gorm:"column:lease_owner"`
	LeaseUntil          sql.NullTime   `gorm:"column:lease_until"`
	FencingToken        int64          `gorm:"column:fencing_token"`
	JobEpoch            int64          `gorm:"column:job_epoch"`
	NextAttemptAt       time.Time      `gorm:"column:next_attempt_at"`
	Checkpoint          string         `gorm:"column:checkpoint"`
	ErrorCode           sql.NullString `gorm:"column:error_code"`
	StartedAt           sql.NullTime   `gorm:"column:started_at"`
	FinishedAt          sql.NullTime   `gorm:"column:finished_at"`
}

func (row *nodeRow) toDomain() d.AgentRunNode {
	var defVerID *string
	if row.DefinitionVersionID.Valid {
		defVerID = &row.DefinitionVersionID.String
	}
	var toolVerID *string
	if row.ToolVersionID.Valid {
		toolVerID = &row.ToolVersionID.String
	}
	var leaseOwner *string
	if row.LeaseOwner.Valid {
		leaseOwner = &row.LeaseOwner.String
	}
	var leaseUntil *time.Time
	if row.LeaseUntil.Valid {
		leaseUntil = &row.LeaseUntil.Time
	}
	var errCode *string
	if row.ErrorCode.Valid {
		errCode = &row.ErrorCode.String
	}
	var startedAt *time.Time
	if row.StartedAt.Valid {
		startedAt = &row.StartedAt.Time
	}
	var finishedAt *time.Time
	if row.FinishedAt.Valid {
		finishedAt = &row.FinishedAt.Time
	}

	checkpoint := json.RawMessage(row.Checkpoint)
	if len(checkpoint) == 0 {
		checkpoint = json.RawMessage("{}")
	}

	return d.AgentRunNode{
		ID:                  row.ID,
		TenantID:            row.TenantID,
		CreatedAt:           row.CreatedAt,
		UpdatedAt:           row.UpdatedAt,
		Revision:            row.Revision,
		RunID:               row.RunID,
		NodeKey:             row.NodeKey,
		Role:                d.NodeRole(row.Role),
		DependsOn:           []string(row.DependsOn),
		Required:            row.Required,
		DefinitionVersionID: defVerID,
		ToolVersionID:       toolVerID,
		Status:              d.NodeStatus(row.Status),
		Phase:               d.NodePhase(row.Phase),
		Iteration:           row.Iteration,
		Attempt:             row.Attempt,
		LeaseOwner:          leaseOwner,
		LeaseUntil:          leaseUntil,
		FencingToken:        row.FencingToken,
		JobEpoch:            row.JobEpoch,
		NextAttemptAt:       row.NextAttemptAt,
		Checkpoint:          checkpoint,
		ErrorCode:           errCode,
		StartedAt:           startedAt,
		FinishedAt:          finishedAt,
	}
}

func (r *RunRepositoryPostgres) GetNode(ctx context.Context, tenantID, runID, nodeKey string) (*d.AgentRunNode, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, err
	}
	var row nodeRow
	q := db.Raw(`SELECT * FROM agent_run_nodes WHERE tenant_id = ? AND run_id = ? AND node_key = ?`, tenantID, runID, nodeKey).Scan(&row)
	if q.Error != nil {
		return nil, q.Error
	}
	if q.RowsAffected == 0 {
		return nil, d.ErrNotFound()
	}
	res := row.toDomain()
	return &res, nil
}

func (r *RunRepositoryPostgres) GetNodeByID(ctx context.Context, tenantID, runID, nodeID string) (*d.AgentRunNode, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, err
	}
	var row nodeRow
	q := db.Raw(`SELECT * FROM agent_run_nodes WHERE tenant_id = ? AND run_id = ? AND id = ?`, tenantID, runID, nodeID).Scan(&row)
	if q.Error != nil {
		return nil, q.Error
	}
	if q.RowsAffected == 0 {
		return nil, d.ErrNotFound()
	}
	res := row.toDomain()
	return &res, nil
}

func (r *RunRepositoryPostgres) ListNodes(ctx context.Context, tenantID, runID string) ([]d.AgentRunNode, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, err
	}
	var rows []nodeRow
	if err := db.Raw(`SELECT * FROM agent_run_nodes WHERE tenant_id = ? AND run_id = ? ORDER BY id ASC`, tenantID, runID).Scan(&rows).Error; err != nil {
		return nil, err
	}
	nodes := make([]d.AgentRunNode, 0, len(rows))
	for _, row := range rows {
		nodes = append(nodes, row.toDomain())
	}
	return nodes, nil
}

func (r *RunRepositoryPostgres) ListReadyNodes(ctx context.Context, tenantID, runID string, limit int) ([]d.AgentRunNode, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, err
	}
	var rows []nodeRow
	q := db.Raw(`
		SELECT * FROM agent_run_nodes
		WHERE tenant_id = ? AND run_id = ? AND status = 'ready'
		ORDER BY id ASC
		LIMIT ?
	`, tenantID, runID, limit).Scan(&rows)
	if q.Error != nil {
		return nil, q.Error
	}
	nodes := make([]d.AgentRunNode, 0, len(rows))
	for _, row := range rows {
		nodes = append(nodes, row.toDomain())
	}
	return nodes, nil
}

func (r *RunRepositoryPostgres) ClaimNodeAttempt(
	ctx context.Context,
	tenantID, nodeID string,
	expectedRev int64,
	workerID string,
	leaseUntil time.Time,
	jobEpoch int64,
) (*d.AgentRunNode, error) {
	db, err := r.db(ctx, true)
	if err != nil {
		return nil, err
	}

	var row nodeRow
	q := db.Raw(`
		UPDATE agent_run_nodes
		SET status = 'running',
		    attempt = attempt + 1,
		    fencing_token = fencing_token + 1,
		    job_epoch = ?,
		    lease_owner = ?,
		    lease_until = ?,
		    revision = revision + 1,
		    started_at = COALESCE(started_at, now()),
		    updated_at = now()
		WHERE tenant_id = ? AND id = ? AND revision = ? AND status IN ('ready', 'retry_wait')
		RETURNING *
	`, jobEpoch, workerID, leaseUntil, tenantID, nodeID, expectedRev).Scan(&row)

	if q.Error != nil {
		return nil, q.Error
	}
	if q.RowsAffected == 0 {
		return nil, d.ErrVersionConflict(map[string]any{"expected_revision": expectedRev, "reason": "node lease lost or conflict"})
	}
	res := row.toDomain()
	return &res, nil
}

func (r *RunRepositoryPostgres) HeartbeatNodes(ctx context.Context, tenantID string, nodeIDs []string, workerID string, leaseUntil time.Time) error {
	if len(nodeIDs) == 0 {
		return nil
	}
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}
	return db.Exec(`
		UPDATE agent_run_nodes
		SET lease_until = ?, updated_at = now()
		WHERE tenant_id = ? AND id IN ? AND lease_owner = ? AND status = 'running'
	`, leaseUntil, tenantID, nodeIDs, workerID).Error
}

func (r *RunRepositoryPostgres) SaveNodeCheckpoint(
	ctx context.Context,
	tenantID, nodeID string,
	expectedRev int64,
	jobEpoch, fence int64,
	phase d.NodePhase,
	iteration int,
	status d.NodeStatus,
	checkpoint json.RawMessage,
	errorCode *string,
) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}

	q := db.Exec(`
		UPDATE agent_run_nodes
		SET phase = ?, iteration = ?, status = ?, checkpoint = ?::jsonb,
		    error_code = ?, revision = revision + 1, updated_at = now()
		WHERE tenant_id = ? AND id = ? AND revision = ? AND job_epoch = ? AND fencing_token = ?
	`, string(phase), iteration, string(status), string(checkpoint), errorCode, tenantID, nodeID, expectedRev, jobEpoch, fence)

	if q.Error != nil {
		return q.Error
	}
	if q.RowsAffected == 0 {
		return d.ErrVersionConflict(map[string]any{"node_id": nodeID, "reason": "fence or epoch mismatch"})
	}
	return nil
}

func (r *RunRepositoryPostgres) FinishNode(
	ctx context.Context,
	tenantID, nodeID string,
	expectedRev int64,
	jobEpoch, fence int64,
	status d.NodeStatus,
	errorCode *string,
) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}

	q := db.Exec(`
		UPDATE agent_run_nodes
		SET status = ?, finished_at = now(), error_code = ?, revision = revision + 1, updated_at = now()
		WHERE tenant_id = ? AND id = ? AND revision = ? AND job_epoch = ? AND fencing_token = ?
	`, string(status), errorCode, tenantID, nodeID, expectedRev, jobEpoch, fence)

	if q.Error != nil {
		return q.Error
	}
	if q.RowsAffected == 0 {
		return d.ErrVersionConflict(map[string]any{"node_id": nodeID, "reason": "fence or epoch mismatch"})
	}
	return nil
}

func (r *RunRepositoryPostgres) UpdateNodeStatus(ctx context.Context, tenantID, nodeID string, expectedRev int64, status d.NodeStatus) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}

	q := db.Exec(`
		UPDATE agent_run_nodes
		SET status = ?, revision = revision + 1, updated_at = now()
		WHERE tenant_id = ? AND id = ? AND revision = ?
	`, string(status), tenantID, nodeID, expectedRev)

	if q.Error != nil {
		return q.Error
	}
	if q.RowsAffected == 0 {
		return d.ErrVersionConflict(map[string]any{"node_id": nodeID})
	}
	return nil
}

// --- Executions ---

type executionRow struct {
	ID              string         `gorm:"column:id"`
	TenantID        string         `gorm:"column:tenant_id"`
	CreatedAt       time.Time      `gorm:"column:created_at"`
	UpdatedAt       time.Time      `gorm:"column:updated_at"`
	Revision        int64          `gorm:"column:revision"`
	RunID           string         `gorm:"column:run_id"`
	NodeID          string         `gorm:"column:node_id"`
	ToolVersionID   string         `gorm:"column:tool_version_id"`
	ExecutionKey    string         `gorm:"column:execution_key"`
	Iteration       int            `gorm:"column:iteration"`
	CallIndex       int            `gorm:"column:call_index"`
	ArgumentsHash   string         `gorm:"column:arguments_hash"`
	FrozenArguments string         `gorm:"column:frozen_arguments"`
	Status          string         `gorm:"column:status"`
	FencingToken    int64          `gorm:"column:fencing_token"`
	JobEpoch        int64          `gorm:"column:job_epoch"`
	ResultFileID    sql.NullString `gorm:"column:result_file_id"`
	ResultHash      sql.NullString `gorm:"column:result_hash"`
	UpstreamReceipt sql.NullString `gorm:"column:upstream_receipt"`
	ErrorCode       sql.NullString `gorm:"column:error_code"`
	DispatchedAt    sql.NullTime   `gorm:"column:dispatched_at"`
	FinishedAt      sql.NullTime   `gorm:"column:finished_at"`
}

func (row *executionRow) toDomain() *d.AgentExecution {
	var resFileID *string
	if row.ResultFileID.Valid {
		resFileID = &row.ResultFileID.String
	}
	var resHash *string
	if row.ResultHash.Valid {
		resHash = &row.ResultHash.String
	}
	var receipt *string
	if row.UpstreamReceipt.Valid {
		receipt = &row.UpstreamReceipt.String
	}
	var errCode *string
	if row.ErrorCode.Valid {
		errCode = &row.ErrorCode.String
	}
	var dispatchedAt *time.Time
	if row.DispatchedAt.Valid {
		dispatchedAt = &row.DispatchedAt.Time
	}
	var finishedAt *time.Time
	if row.FinishedAt.Valid {
		finishedAt = &row.FinishedAt.Time
	}

	return &d.AgentExecution{
		ID:              row.ID,
		TenantID:        row.TenantID,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
		Revision:        row.Revision,
		RunID:           row.RunID,
		NodeID:          row.NodeID,
		ToolVersionID:   row.ToolVersionID,
		ExecutionKey:    row.ExecutionKey,
		Iteration:       row.Iteration,
		CallIndex:       row.CallIndex,
		ArgumentsHash:   row.ArgumentsHash,
		FrozenArguments: json.RawMessage(row.FrozenArguments),
		Status:          d.ExecutionStatus(row.Status),
		FencingToken:    row.FencingToken,
		JobEpoch:        row.JobEpoch,
		ResultFileID:    resFileID,
		ResultHash:      resHash,
		UpstreamReceipt: receipt,
		ErrorCode:       errCode,
		DispatchedAt:    dispatchedAt,
		FinishedAt:      finishedAt,
	}
}

func (r *RunRepositoryPostgres) CreateExecution(ctx context.Context, tenantID string, exec *d.AgentExecution) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}

	return db.Exec(`
		INSERT INTO agent_executions (
			id, tenant_id, created_at, updated_at, revision, run_id, node_id, tool_version_id,
			execution_key, iteration, call_index, arguments_hash, frozen_arguments,
			status, fencing_token, job_epoch
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?::jsonb,
			?, ?, ?
		)
	`,
		exec.ID, tenantID, exec.CreatedAt, exec.UpdatedAt, exec.Revision, exec.RunID, exec.NodeID, exec.ToolVersionID,
		exec.ExecutionKey, exec.Iteration, exec.CallIndex, exec.ArgumentsHash, string(exec.FrozenArguments),
		string(exec.Status), exec.FencingToken, exec.JobEpoch,
	).Error
}

func (r *RunRepositoryPostgres) GetExecutionByKey(ctx context.Context, tenantID, executionKey string) (*d.AgentExecution, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, err
	}
	var row executionRow
	q := db.Raw(`SELECT * FROM agent_executions WHERE tenant_id = ? AND execution_key = ?`, tenantID, executionKey).Scan(&row)
	if q.Error != nil {
		return nil, q.Error
	}
	if q.RowsAffected == 0 {
		return nil, d.ErrNotFound()
	}
	return row.toDomain(), nil
}

func (r *RunRepositoryPostgres) GetExecutionByID(ctx context.Context, tenantID, executionID string) (*d.AgentExecution, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, err
	}
	var row executionRow
	q := db.Raw(`SELECT * FROM agent_executions WHERE tenant_id = ? AND id = ?`, tenantID, executionID).Scan(&row)
	if q.Error != nil {
		return nil, q.Error
	}
	if q.RowsAffected == 0 {
		return nil, d.ErrNotFound()
	}
	return row.toDomain(), nil
}

func (r *RunRepositoryPostgres) UpdateExecution(
	ctx context.Context,
	tenantID, execID string,
	status d.ExecutionStatus,
	resultFileID *string,
	resultHash *string,
	receipt *string,
	errCode *string,
) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}

	return db.Exec(`
		UPDATE agent_executions
		SET status = ?, result_file_id = ?, result_hash = ?, upstream_receipt = ?,
		    error_code = ?, finished_at = now(), updated_at = now(), revision = revision + 1
		WHERE tenant_id = ? AND id = ?
	`, string(status), resultFileID, resultHash, receipt, errCode, tenantID, execID).Error
}

// --- Approvals ---

type approvalRow struct {
	ID             string         `gorm:"column:id"`
	TenantID       string         `gorm:"column:tenant_id"`
	CreatedAt      time.Time      `gorm:"column:created_at"`
	UpdatedAt      time.Time      `gorm:"column:updated_at"`
	Revision       int64          `gorm:"column:revision"`
	OwnerUserID    string         `gorm:"column:owner_user_id"`
	RunID          string         `gorm:"column:run_id"`
	NodeID         string         `gorm:"column:node_id"`
	ExecutionID    string         `gorm:"column:execution_id"`
	ToolVersionID  string         `gorm:"column:tool_version_id"`
	ArgumentsHash  string         `gorm:"column:arguments_hash"`
	SourceRevision int64          `gorm:"column:source_revision"`
	Status         string         `gorm:"column:status"`
	ExpiresAt      time.Time      `gorm:"column:expires_at"`
	DecidedAt      sql.NullTime   `gorm:"column:decided_at"`
	DecisionReason sql.NullString `gorm:"column:decision_reason"`
}

func (row *approvalRow) toDomain() *d.AgentApproval {
	var decidedAt *time.Time
	if row.DecidedAt.Valid {
		decidedAt = &row.DecidedAt.Time
	}
	var decReason *string
	if row.DecisionReason.Valid {
		decReason = &row.DecisionReason.String
	}

	return &d.AgentApproval{
		ID:             row.ID,
		TenantID:       row.TenantID,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		Revision:       row.Revision,
		OwnerUserID:    row.OwnerUserID,
		RunID:          row.RunID,
		NodeID:         row.NodeID,
		ExecutionID:    row.ExecutionID,
		ToolVersionID:  row.ToolVersionID,
		ArgumentsHash:  row.ArgumentsHash,
		SourceRevision: row.SourceRevision,
		Status:         d.ApprovalStatus(row.Status),
		ExpiresAt:      row.ExpiresAt,
		DecidedAt:      decidedAt,
		DecisionReason: decReason,
	}
}

func (r *RunRepositoryPostgres) CreateApproval(ctx context.Context, tenantID string, app *d.AgentApproval) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}

	return db.Exec(`
		INSERT INTO agent_approvals (
			id, tenant_id, created_at, updated_at, revision, owner_user_id,
			run_id, node_id, execution_id, tool_version_id, arguments_hash,
			source_revision, status, expires_at
		) VALUES (
			?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?
		)
	`,
		app.ID, tenantID, app.CreatedAt, app.UpdatedAt, app.Revision, app.OwnerUserID,
		app.RunID, app.NodeID, app.ExecutionID, app.ToolVersionID, app.ArgumentsHash,
		app.SourceRevision, string(app.Status), app.ExpiresAt,
	).Error
}

func (r *RunRepositoryPostgres) GetApproval(ctx context.Context, tenantID, approvalID string) (*d.AgentApproval, *d.AgentExecution, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, nil, err
	}

	var row approvalRow
	q := db.Raw(`SELECT * FROM agent_approvals WHERE tenant_id = ? AND id = ?`, tenantID, approvalID).Scan(&row)
	if q.Error != nil {
		return nil, nil, q.Error
	}
	if q.RowsAffected == 0 {
		return nil, nil, d.ErrNotFound()
	}

	exec, err := r.GetExecutionByID(ctx, tenantID, row.ExecutionID)
	if err != nil {
		return row.toDomain(), nil, nil
	}

	return row.toDomain(), exec, nil
}

func (r *RunRepositoryPostgres) ListPendingApprovals(ctx context.Context, tenantID, ownerID string, query d.ListQuery) ([]d.ApprovalView, *string, bool, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, nil, false, err
	}

	limit := query.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	type joinedRow struct {
		approvalRow
		FrozenArguments string `gorm:"column:frozen_arguments"`
	}

	qb := db.Table("agent_approvals a").
		Select("a.*, e.frozen_arguments").
		Joins("JOIN agent_executions e ON e.tenant_id = a.tenant_id AND e.id = a.execution_id").
		Where("a.tenant_id = ? AND a.owner_user_id = ? AND a.status = 'pending'", tenantID, ownerID)

	if query.BeforeTime != "" && query.BeforeID != "" {
		qb = qb.Where("(a.created_at, a.id) < (?, ?)", query.BeforeTime, query.BeforeID)
	}

	var rows []joinedRow
	if err := qb.Order("a.created_at DESC, a.id DESC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, nil, false, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	views := make([]d.ApprovalView, 0, len(rows))
	for _, r := range rows {
		dom := r.toDomain()
		views = append(views, d.ApprovalView{
			ID:             dom.ID,
			Revision:       dom.Revision,
			CreatedAt:      dom.CreatedAt,
			UpdatedAt:      dom.UpdatedAt,
			RunID:          dom.RunID,
			NodeID:         dom.NodeID,
			OwnerUserID:    dom.OwnerUserID,
			ToolVersionID:  dom.ToolVersionID,
			ArgumentsHash:  dom.ArgumentsHash,
			ArgumentsJSON:  r.FrozenArguments,
			SourceRevision: dom.SourceRevision,
			Status:         dom.Status,
			ExpiresAt:      dom.ExpiresAt,
			DecidedAt:      dom.DecidedAt,
			DecisionReason: dom.DecisionReason,
		})
	}

	var nextCursor *string
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		c := fmt.Sprintf("%s_%s", last.CreatedAt.UTC().Format(time.RFC3339Nano), last.ID)
		nextCursor = &c
	}

	return views, nextCursor, hasMore, nil
}

func (r *RunRepositoryPostgres) ConsumeApproval(ctx context.Context, tenantID, approvalID string, expectedRev int64, reason string) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}

	q := db.Exec(`
		UPDATE agent_approvals
		SET status = 'consumed', decided_at = now(), decision_reason = NULLIF(?, ''),
		    revision = revision + 1, updated_at = now()
		WHERE tenant_id = ? AND id = ? AND revision = ? AND status = 'pending'
	`, reason, tenantID, approvalID, expectedRev)

	if q.Error != nil {
		return q.Error
	}
	if q.RowsAffected == 0 {
		return d.ErrVersionConflict(map[string]any{"approval_id": approvalID, "expected_revision": expectedRev})
	}
	return nil
}

func (r *RunRepositoryPostgres) RejectApproval(ctx context.Context, tenantID, approvalID string, expectedRev int64, reason string) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}

	q := db.Exec(`
		UPDATE agent_approvals
		SET status = 'rejected', decided_at = now(), decision_reason = NULLIF(?, ''),
		    revision = revision + 1, updated_at = now()
		WHERE tenant_id = ? AND id = ? AND revision = ? AND status = 'pending'
	`, reason, tenantID, approvalID, expectedRev)

	if q.Error != nil {
		return q.Error
	}
	if q.RowsAffected == 0 {
		return d.ErrVersionConflict(map[string]any{"approval_id": approvalID, "expected_revision": expectedRev})
	}
	return nil
}

func (r *RunRepositoryPostgres) CancelApproval(ctx context.Context, tenantID, approvalID string, expectedRev int64, reason string) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}

	q := db.Exec(`
		UPDATE agent_approvals
		SET status = 'cancelled', decided_at = now(), decision_reason = NULLIF(?, ''),
		    revision = revision + 1, updated_at = now()
		WHERE tenant_id = ? AND id = ? AND revision = ? AND status = 'pending'
	`, reason, tenantID, approvalID, expectedRev)

	if q.Error != nil {
		return q.Error
	}
	if q.RowsAffected == 0 {
		return d.ErrVersionConflict(map[string]any{"approval_id": approvalID, "expected_revision": expectedRev})
	}
	return nil
}

// --- Artifacts ---

type artifactRow struct {
	ID               string         `gorm:"column:id"`
	TenantID         string         `gorm:"column:tenant_id"`
	CreatedAt        time.Time      `gorm:"column:created_at"`
	UpdatedAt        time.Time      `gorm:"column:updated_at"`
	RunID            string         `gorm:"column:run_id"`
	NodeID           string         `gorm:"column:node_id"`
	Kind             string         `gorm:"column:kind"`
	TargetResourceID sql.NullString `gorm:"column:target_resource_id"`
	SourceRevision   int64          `gorm:"column:source_revision"`
	ContentHash      string         `gorm:"column:content_hash"`
	TextContent      sql.NullString `gorm:"column:text_content"`
	FileID           sql.NullString `gorm:"column:file_id"`
	IsPartial        bool           `gorm:"column:is_partial"`
}

func (row *artifactRow) toDomain() *d.AgentArtifact {
	var targetResID *string
	if row.TargetResourceID.Valid {
		targetResID = &row.TargetResourceID.String
	}
	var textContent *string
	if row.TextContent.Valid {
		textContent = &row.TextContent.String
	}
	var fileID *string
	if row.FileID.Valid {
		fileID = &row.FileID.String
	}

	return &d.AgentArtifact{
		ID:               row.ID,
		TenantID:         row.TenantID,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
		RunID:            row.RunID,
		NodeID:           row.NodeID,
		Kind:             d.ArtifactKind(row.Kind),
		TargetResourceID: targetResID,
		SourceRevision:   row.SourceRevision,
		ContentHash:      row.ContentHash,
		TextContent:      textContent,
		FileID:           fileID,
		IsPartial:        row.IsPartial,
	}
}

func (r *RunRepositoryPostgres) CreateArtifact(ctx context.Context, tenantID string, art *d.AgentArtifact) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}

	return db.Exec(`
		INSERT INTO agent_artifacts (
			id, tenant_id, created_at, updated_at, run_id, node_id, kind,
			target_resource_id, source_revision, content_hash, text_content, file_id, is_partial
		) VALUES (
			?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?
		)
	`,
		art.ID, tenantID, art.CreatedAt, art.UpdatedAt, art.RunID, art.NodeID, string(art.Kind),
		art.TargetResourceID, art.SourceRevision, art.ContentHash, art.TextContent, art.FileID, art.IsPartial,
	).Error
}

func (r *RunRepositoryPostgres) GetArtifact(ctx context.Context, tenantID, artifactID string) (*d.AgentArtifact, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, err
	}
	var row artifactRow
	q := db.Raw(`SELECT * FROM agent_artifacts WHERE tenant_id = ? AND id = ?`, tenantID, artifactID).Scan(&row)
	if q.Error != nil {
		return nil, q.Error
	}
	if q.RowsAffected == 0 {
		return nil, d.ErrNotFound()
	}
	return row.toDomain(), nil
}

func (r *RunRepositoryPostgres) ListArtifacts(ctx context.Context, tenantID, runID string, query d.ListQuery) ([]d.ArtifactView, *string, bool, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, nil, false, err
	}

	limit := query.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	qb := db.Table("agent_artifacts").
		Where("tenant_id = ? AND run_id = ?", tenantID, runID)

	if query.BeforeTime != "" && query.BeforeID != "" {
		qb = qb.Where("(created_at, id) < (?, ?)", query.BeforeTime, query.BeforeID)
	}

	var rows []artifactRow
	if err := qb.Order("created_at DESC, id DESC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, nil, false, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	views := make([]d.ArtifactView, 0, len(rows))
	for _, row := range rows {
		dom := row.toDomain()
		views = append(views, d.ArtifactView{
			ID:               dom.ID,
			CreatedAt:        dom.CreatedAt,
			RunID:            dom.RunID,
			NodeID:           dom.NodeID,
			Kind:             dom.Kind,
			TargetResourceID: dom.TargetResourceID,
			SourceRevision:   dom.SourceRevision,
			ContentHash:      dom.ContentHash,
			TextContent:      dom.TextContent,
			FileID:           dom.FileID,
			IsPartial:        dom.IsPartial,
		})
	}

	var nextCursor *string
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		c := fmt.Sprintf("%s_%s", last.CreatedAt.UTC().Format(time.RFC3339Nano), last.ID)
		nextCursor = &c
	}

	return views, nextCursor, hasMore, nil
}

// --- Context Sources ---

func (r *RunRepositoryPostgres) SaveContextSources(ctx context.Context, tenantID string, sources []d.AgentContextSource) error {
	if len(sources) == 0 {
		return nil
	}
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}

	for _, s := range sources {
		parents := TextArray(s.MemoryParentIDs)
		if parents == nil {
			parents = TextArray{}
		}
		err := db.Exec(`
			INSERT INTO agent_context_sources (
				id, tenant_id, created_at, updated_at, run_id, source_domain,
				source_type, source_id, source_revision, content_hash, memory_kind,
				token_count, estimated, disposition, memory_parent_ids
			) VALUES (
				?, ?, ?, ?, ?, ?,
				?, ?, ?, ?, ?,
				?, ?, ?, ?
			)
		`,
			s.ID, tenantID, s.CreatedAt, s.UpdatedAt, s.RunID, s.SourceDomain,
			s.SourceType, s.SourceID, s.SourceRevision, s.ContentHash, string(s.MemoryKind),
			s.TokenCount, s.Estimated, string(s.Disposition), parents,
		).Error
		if err != nil {
			return err
		}
	}
	return nil
}

// --- Events ---

type eventRow struct {
	ID        string    `gorm:"column:id"`
	TenantID  string    `gorm:"column:tenant_id"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
	RunID     string    `gorm:"column:run_id"`
	Seq       int64     `gorm:"column:seq"`
	EventName string    `gorm:"column:event_name"`
	Payload   string    `gorm:"column:payload"`
	ExpiresAt time.Time `gorm:"column:expires_at"`
}

func (r *RunRepositoryPostgres) AppendEvent(ctx context.Context, tenantID string, event *d.AgentRunEvent) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}

	return db.Exec(`
		INSERT INTO agent_run_events (
			id, tenant_id, created_at, updated_at, run_id, seq, event_name, payload, expires_at
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?::jsonb, ?
		)
	`,
		event.ID, tenantID, event.CreatedAt, event.UpdatedAt, event.RunID, event.Seq, event.EventName, string(event.Payload), event.ExpiresAt,
	).Error
}

func (r *RunRepositoryPostgres) AppendEvents(ctx context.Context, tenantID string, events []*d.AgentRunEvent) error {
	for _, ev := range events {
		if err := r.AppendEvent(ctx, tenantID, ev); err != nil {
			return err
		}
	}
	return nil
}

func (r *RunRepositoryPostgres) GetEventsAfter(ctx context.Context, tenantID, runID string, afterSeq int64, limit int) ([]d.AgentRunEvent, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}

	var rows []eventRow
	q := db.Raw(`
		SELECT * FROM agent_run_events
		WHERE tenant_id = ? AND run_id = ? AND seq > ?
		ORDER BY seq ASC
		LIMIT ?
	`, tenantID, runID, afterSeq, limit).Scan(&rows)
	if q.Error != nil {
		return nil, q.Error
	}

	events := make([]d.AgentRunEvent, 0, len(rows))
	for _, row := range rows {
		events = append(events, d.AgentRunEvent{
			ID:        row.ID,
			TenantID:  row.TenantID,
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
			RunID:     row.RunID,
			Seq:       row.Seq,
			EventName: row.EventName,
			Payload:   json.RawMessage(row.Payload),
			ExpiresAt: row.ExpiresAt,
		})
	}
	return events, nil
}

func (r *RunRepositoryPostgres) CheckEventCursorExpired(ctx context.Context, tenantID, runID string, cursorSeq int64) (bool, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return false, err
	}

	var runRow runRow
	q := db.Raw(`SELECT retained_from_seq FROM agent_runs WHERE tenant_id = ? AND id = ?`, tenantID, runID).Scan(&runRow)
	if q.Error != nil {
		return false, q.Error
	}
	if q.RowsAffected == 0 {
		return false, d.ErrNotFound()
	}

	if cursorSeq < runRow.RetainedFromSeq {
		return true, nil
	}

	// Check if the event itself has expired (now() > expires_at)
	var expired bool
	err = db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM agent_run_events
			WHERE tenant_id = ? AND run_id = ? AND seq = ? AND expires_at <= now()
		)
	`, tenantID, runID, cursorSeq).Scan(&expired).Error
	if err != nil {
		return false, err
	}
	return expired, nil
}

var _ d.RunRepository = (*RunRepositoryPostgres)(nil)
