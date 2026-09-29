package artifactstore

import (
	"context"
	"fmt"
	"strings"
	"time"

	"InkLore/internal/platform/nulltype"
	"InkLore/internal/platform/tenantcontext"

	"gorm.io/gorm"
)

type ArtifactRefRepository struct {
	db *gorm.DB
}

type ArtifactRefStore = ArtifactRefRepository

type ArtifactRef struct {
	UpdatedAt         time.Time
	CreatedAt         time.Time
	StorageKey        string
	ArtifactType      string
	Status            string
	ContentHash       string
	StorageBackend    string
	Source            string
	StorageObjectID   string
	IdempotencyKey    string
	LastErrorCategory string
	TaskID            nulltype.Int64
	AgentNodeRunID    nulltype.Int64
	AgentRunID        nulltype.Int64
	WorkflowID        nulltype.Int64
	OwnerUserID       nulltype.Int64
	SizeBytes         int64
	AttemptCount      int
	ID                int64
	ReadyAt           *time.Time
}

type ArtifactRefCreate struct {
	StorageBackend  string
	Source          string
	StorageObjectID string
	StorageKey      string
	ArtifactType    string
	ContentHash     string
	Status          string
	OwnerUserID     nulltype.Int64
	WorkflowID      nulltype.Int64
	TaskID          nulltype.Int64
	AgentRunID      nulltype.Int64
	AgentNodeRunID  nulltype.Int64
	SizeBytes       int64
	IdempotencyKey  string
}

func NewArtifactRefRepository(db any) *ArtifactRefRepository {
	switch v := db.(type) {
	case *ArtifactRefRepository:
		return v
	case *gorm.DB:
		return &ArtifactRefRepository{db: v}
	default:
		panic(fmt.Sprintf("artifactstore: unsupported DB handle %T", db))
	}
}

// Create 是 legacy 无条件插入路径（无幂等键，同 key 并发可产生多行）。
// M-P2-9：新代码必须使用 Begin/Saga.Write——Begin 以 idempotency_key 唯一索引 +
// ON CONFLICT DO UPDATE 保证同 key 并发 Write 收敛到同一 ref（at-least-once 重放安全）。
// 本方法仅保留给历史调用方迁移；新增业务引用一律走 Begin。
func (r *ArtifactRefRepository) Create(ctx context.Context, in ArtifactRefCreate) (int64, error) {
	if in.ArtifactType == "" {
		return 0, fmt.Errorf("artifact_type is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if in.StorageBackend == "" {
		in.StorageBackend = BackendPostgres
	}
	if in.Status == "" {
		in.Status = "active"
	}
	var id int64
	if err := r.db.WithContext(ctx).Raw(`
		INSERT INTO artifact_refs (
			tenant_id, artifact_type, owner_user_id, workflow_id, generation_task_id, agent_run_id, agent_node_run_id,
			source, storage_object_id, storage_key, storage_backend, content_hash, size_bytes, status,
			created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING id
	`, tenantcontext.OrDefault(ctx), in.ArtifactType, nulltype.NullableIntArg(in.OwnerUserID), nulltype.NullableIntArg(in.WorkflowID), nulltype.NullableIntArg(in.TaskID), nulltype.NullableIntArg(in.AgentRunID), nulltype.NullableIntArg(in.AgentNodeRunID), in.Source, in.StorageObjectID, in.StorageKey, in.StorageBackend, in.ContentHash, in.SizeBytes, in.Status).Scan(&id).Error; err != nil {
		return 0, fmt.Errorf("创建 artifact ref 失败: %w", err)
	}
	return id, nil
}

func (r *ArtifactRefRepository) Begin(ctx context.Context, in ArtifactRefCreate) (*ArtifactRef, bool, error) {
	if r == nil || r.db == nil {
		return nil, false, fmt.Errorf("artifact ref repository is not initialized")
	}
	in.ArtifactType = strings.TrimSpace(in.ArtifactType)
	in.IdempotencyKey = strings.TrimSpace(in.IdempotencyKey)
	if in.ArtifactType == "" {
		return nil, false, fmt.Errorf("artifact_type is required")
	}
	if in.IdempotencyKey == "" {
		return nil, false, fmt.Errorf("idempotency_key is required")
	}
	if in.StorageBackend == "" {
		in.StorageBackend = BackendPostgres
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var item ArtifactRef
	result := r.db.WithContext(ctx).Raw(`
		INSERT INTO artifact_refs (
			tenant_id, artifact_type, owner_user_id, workflow_id, generation_task_id, agent_run_id, agent_node_run_id,
			source, storage_backend, status, idempotency_key, attempt_count, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT (tenant_id, idempotency_key) WHERE idempotency_key <> '' DO UPDATE SET
			attempt_count = artifact_refs.attempt_count + 1,
			status = CASE WHEN artifact_refs.status IN ('ready', 'active') THEN artifact_refs.status ELSE 'pending' END,
			last_error_category = CASE WHEN artifact_refs.status IN ('ready', 'active') THEN artifact_refs.last_error_category ELSE '' END,
			updated_at = CURRENT_TIMESTAMP
		WHERE artifact_refs.status NOT IN ('deleted', 'cleanup_pending')
		RETURNING id, artifact_type, owner_user_id, workflow_id, generation_task_id AS task_id,
		          agent_run_id, agent_node_run_id, source, storage_object_id, storage_key, storage_backend,
		          content_hash, size_bytes, status, idempotency_key, attempt_count,
		          last_error_category, ready_at, created_at, updated_at
	`, tenantcontext.OrDefault(ctx), in.ArtifactType, nulltype.NullableIntArg(in.OwnerUserID), nulltype.NullableIntArg(in.WorkflowID), nulltype.NullableIntArg(in.TaskID),
		nulltype.NullableIntArg(in.AgentRunID), nulltype.NullableIntArg(in.AgentNodeRunID), in.Source, in.StorageBackend,
		StatusPending, in.IdempotencyKey).Scan(&item)
	if result.Error != nil {
		return nil, false, fmt.Errorf("创建 artifact pending ref 失败: %w", result.Error)
	}
	// P1-16：同 idempotency_key 已 deleted / cleanup_pending 的重放必须拒绝
	// （不能把已终结/清理中的 ref 静默复活为 pending）。DO UPDATE 的 WHERE 守卫已阻止
	// 状态改写；此处兜底识别「守卫未命中」的两种情况：RETURNING 未返回行（item.ID==0）
	// 或返回了未变更的既有行（status 仍为 deleted/cleanup_pending）。
	if item.ID == 0 {
		return nil, false, fmt.Errorf("创建 artifact pending ref 失败: idempotency_key %q 已存在且处于终态（deleted/cleanup_pending），拒绝复活重放", in.IdempotencyKey)
	}
	if item.Status == StatusDeleted || item.Status == StatusCleanupPending {
		return nil, false, fmt.Errorf("创建 artifact pending ref 失败: idempotency_key %q 对应 ref %d 已处于 %s，拒绝复活重放", in.IdempotencyKey, item.ID, item.Status)
	}
	return &item, item.AttemptCount > 1, nil
}

func (r *ArtifactRefRepository) MarkReady(ctx context.Context, id int64, locator ArtifactLocator) error {
	if r == nil || r.db == nil || id <= 0 {
		return fmt.Errorf("invalid artifact ref")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	// P1-16：MarkReady 带状态守卫（CAS）——仅 pending/ready/active 可置为 ready；
	// failed/cleanup_pending/deleted 一律拒绝，防止「Compensate 删除与在途 Write 交错」
	// 时把已补偿/已清理的 ref 复活为 ready。守卫未命中（RowsAffected==0）即放弃本次提交，
	// 调用方（Saga.Write）会走补偿路径删除刚写入的 payload。
	result := r.db.WithContext(ctx).Exec(`
		UPDATE artifact_refs
		SET storage_object_id = ?, storage_key = ?, storage_backend = ?, content_hash = ?, size_bytes = ?,
		    status = ?, last_error_category = '', ready_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND tenant_id = ?
		  AND status IN ('pending', 'ready', 'active')
	`, strings.TrimSpace(locator.StorageObjectID), strings.TrimSpace(locator.StorageKey), strings.TrimSpace(locator.StorageBackend),
		strings.TrimSpace(locator.ContentHash), locator.SizeBytes, StatusReady, id, tenantcontext.OrDefault(ctx))
	if result.Error != nil {
		return fmt.Errorf("更新 artifact ready ref 失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		// 重读当前状态以区分「ref 不存在」与「状态守卫冲突（CAS 失败）」。
		existing, getErr := r.Get(ctx, id)
		if getErr != nil || existing == nil {
			return fmt.Errorf("artifact ref not found")
		}
		return fmt.Errorf("artifact ref %d 状态冲突: 当前状态 %q 不允许置为 ready（终态/补偿中不可复活）", id, existing.Status)
	}
	return nil
}

func (r *ArtifactRefRepository) MarkStatus(ctx context.Context, id int64, status, category string) error {
	if r == nil || r.db == nil || id <= 0 {
		return fmt.Errorf("invalid artifact ref")
	}
	if !IsLifecycleStatus(status) {
		return fmt.Errorf("invalid artifact status %q", status)
	}
	// P1-16：MarkStatus 仅允许写入 failed / cleanup_pending / deleted（前向流转）；
	// pending/ready/active 只能由 Begin/MarkReady 写入，禁止经 MarkStatus 直接覆盖，
	// 避免「无条件 UPDATE 状态」绕过守卫复活已终结 ref。
	switch status {
	case StatusFailed, StatusCleanupPending, StatusDeleted:
	default:
		return fmt.Errorf("invalid artifact status transition target %q via MarkStatus", status)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	normalizedCategory := ""
	if strings.TrimSpace(category) != "" || status != StatusDeleted {
		normalizedCategory = NormalizeErrorCategory(category)
	}
	// 状态守卫：deleted 为终态，任何来源（含 deleted 自身）都不再允许改写；
	// cleanup_pending 可继续前向（failed/cleanup_pending/deleted），但不可复活为可用态。
	result := r.db.WithContext(ctx).Exec(`
		UPDATE artifact_refs
		SET status = ?, last_error_category = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND tenant_id = ?
		  AND status <> 'deleted'
	`, status, normalizedCategory, id, tenantcontext.OrDefault(ctx))
	if result.Error != nil {
		return fmt.Errorf("更新 artifact ref 状态失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		// 重读当前状态区分「ref 不存在」与「终态守卫冲突」。
		existing, getErr := r.Get(ctx, id)
		if getErr != nil || existing == nil {
			return fmt.Errorf("artifact ref not found")
		}
		if existing.Status == StatusDeleted && status == StatusDeleted {
			// 幂等：已 deleted 的 ref 再次标记 deleted（清理重试/重复补偿）视为成功。
			return nil
		}
		return fmt.Errorf("artifact ref %d 状态冲突: 当前状态 %q 不允许转为 %q（deleted 终态不可改写/复活）", id, existing.Status, status)
	}
	return nil
}

func (r *ArtifactRefRepository) AttachOwners(ctx context.Context, id int64, owner ArtifactOwners) error {
	if r == nil || r.db == nil || id <= 0 {
		return fmt.Errorf("invalid artifact ref")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	result := r.db.WithContext(ctx).Exec(`
		UPDATE artifact_refs
		SET owner_user_id = COALESCE(?, owner_user_id), workflow_id = COALESCE(?, workflow_id),
		    generation_task_id = COALESCE(?, generation_task_id), agent_run_id = COALESCE(?, agent_run_id),
		    agent_node_run_id = COALESCE(?, agent_node_run_id), updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND tenant_id = ?
	`, nulltype.NullableIntArg(owner.OwnerUserID), nulltype.NullableIntArg(owner.WorkflowID), nulltype.NullableIntArg(owner.TaskID),
		nulltype.NullableIntArg(owner.AgentRunID), nulltype.NullableIntArg(owner.AgentNodeRunID), id, tenantcontext.OrDefault(ctx))
	if result.Error != nil {
		return fmt.Errorf("关联 artifact owner 失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("artifact ref not found")
	}
	return nil
}

func (r *ArtifactRefRepository) ListReconciliationCandidates(ctx context.Context, cutoff time.Time, limit int) ([]ArtifactRef, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("artifact ref repository is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	items := make([]ArtifactRef, 0)
	if err := r.db.WithContext(ctx).Raw(`
		SELECT id, artifact_type, owner_user_id, workflow_id, generation_task_id AS task_id,
		       agent_run_id, agent_node_run_id, source, storage_object_id, storage_key, storage_backend,
		       content_hash, size_bytes, status, idempotency_key, attempt_count,
		       last_error_category, ready_at, created_at, updated_at
		FROM artifact_refs
		WHERE tenant_id = ?
		  AND updated_at <= ?
		  AND (status IN ('pending', 'cleanup_pending', 'failed')
		       OR (status = 'ready' AND storage_backend = ? AND storage_object_id = ''))
		ORDER BY updated_at ASC, id ASC
		LIMIT ?
	`, tenantcontext.OrDefault(ctx), cutoff, BackendPostgres, limit).Scan(&items).Error; err != nil {
		return nil, fmt.Errorf("查询 artifact reconciliation candidates 失败: %w", err)
	}
	return items, nil
}

func (r *ArtifactRefRepository) Get(ctx context.Context, id int64) (*ArtifactRef, error) {
	if id <= 0 {
		return nil, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var item ArtifactRef
	res := r.db.WithContext(ctx).Raw(`
		SELECT id, artifact_type, owner_user_id, workflow_id, generation_task_id AS task_id, agent_run_id, agent_node_run_id,
		       source, storage_object_id, storage_key, storage_backend, content_hash, size_bytes, status,
		       idempotency_key, attempt_count, last_error_category, ready_at, created_at, updated_at
		FROM artifact_refs
		WHERE id = ? AND tenant_id = ?
	`, id, tenantcontext.OrDefault(ctx)).Scan(&item)
	if res.Error != nil {
		return nil, fmt.Errorf("查询 artifact ref 失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, nil
	}
	return &item, nil
}

func (r *ArtifactRefRepository) ListByAgentRunID(ctx context.Context, runID int64) ([]ArtifactRef, error) {
	if runID <= 0 {
		return nil, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	items := make([]ArtifactRef, 0)
	if err := r.db.WithContext(ctx).Raw(`
		SELECT id, artifact_type, owner_user_id, workflow_id, generation_task_id AS task_id, agent_run_id, agent_node_run_id,
		       source, storage_object_id, storage_key, storage_backend, content_hash, size_bytes, status, created_at, updated_at
		FROM artifact_refs
		WHERE agent_run_id = ? AND tenant_id = ?
		ORDER BY id ASC
	`, runID, tenantcontext.OrDefault(ctx)).Scan(&items).Error; err != nil {
		return nil, fmt.Errorf("查询 Agent artifact refs 失败: %w", err)
	}
	return items, nil
}

func (r *ArtifactRefRepository) ListByWorkflowID(ctx context.Context, workflowID int64) ([]ArtifactRef, error) {
	if workflowID <= 0 {
		return nil, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	items := make([]ArtifactRef, 0)
	if err := r.db.WithContext(ctx).Raw(`
		SELECT id, artifact_type, owner_user_id, workflow_id, generation_task_id AS task_id, agent_run_id, agent_node_run_id,
		       source, storage_object_id, storage_key, storage_backend, content_hash, size_bytes, status, created_at, updated_at
		FROM artifact_refs
		WHERE workflow_id = ? AND tenant_id = ?
		ORDER BY id ASC
	`, workflowID, tenantcontext.OrDefault(ctx)).Scan(&items).Error; err != nil {
		return nil, fmt.Errorf("查询工作流 artifact refs 失败: %w", err)
	}
	return items, nil
}
