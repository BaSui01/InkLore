package agentapproval

import (
	"context"
	"fmt"
	"strings"
	"time"

	agentpersistence "InkLore/internal/agent/infrastructure/persistence"
	"InkLore/internal/platform/dbcompat"
	"InkLore/internal/platform/nulltype"
	"InkLore/internal/platform/sqlscope"
	"InkLore/internal/platform/timeutil"
)

const (
	statusPending  = "pending"
	statusExpired  = "expired"
	statusConsumed = "consumed"
	statusDenied   = "denied"
	statusAll      = "all"

	// defaultApprovalCleanupRetention 是过期审批行的物理保留期（30 天）。
	// M-P2-7：过期/已终结行（consumed/denied/expired pending）只做物理清理，
	// 清理由外部周期任务按此保留期调度调用 DeleteExpired；未过期 pending 行绝不删除。
	defaultApprovalCleanupRetention = 30 * 24 * time.Hour
)

type Repository struct {
	db *agentpersistence.DB
}

type ToolApproval struct {
	ExpiresAt  time.Time
	UpdatedAt  time.Time
	CreatedAt  time.Time
	ConsumedAt nulltype.Time
	ArgsHash   string
	ToolName   string
	TokenHash  string
	Status     string
	RunID      nulltype.Int64
	AgentID    nulltype.Int64
	SessionID  nulltype.Int64
	ID         int64
	UserID     int64
}

type ToolApprovalCreate struct {
	ExpiresAt time.Time
	ToolName  string
	ArgsHash  string
	TokenHash string
	SessionID nulltype.Int64
	AgentID   nulltype.Int64
	RunID     nulltype.Int64
	UserID    int64
}

type ToolApprovalListFilter struct {
	Now           time.Time
	Status        string
	UserID        int64
	TeamID        int64
	DepartmentIDs []int64
	SessionID     int64
	RunID         int64
	Limit         int
	Offset        int
}

func NewRepository(db any) *Repository {
	return &Repository{db: agentpersistence.NewDB(db)}
}

func (r *Repository) Create(ctx context.Context, in ToolApprovalCreate) (*ToolApproval, error) {
	var id int64
	if err := r.db.GORM().WithContext(ctx).Raw(`
		INSERT INTO agent_tool_approvals (
			tenant_id, user_id, session_id, agent_id, run_id, tool_name, args_hash, token_hash, status, expires_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'pending', ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING id
	`, agentpersistence.TenantFromContext(ctx), in.UserID, nulltype.NullableIntArg(in.SessionID), nulltype.NullableIntArg(in.AgentID), nulltype.NullableIntArg(in.RunID), in.ToolName, in.ArgsHash, in.TokenHash, in.ExpiresAt).Scan(&id).Error; err != nil {
		return nil, fmt.Errorf("创建 Agent 工具审批失败: %w", err)
	}
	return r.GetByID(ctx, id)
}

func (r *Repository) GetByID(ctx context.Context, id int64) (*ToolApproval, error) {
	if id <= 0 {
		return nil, dbcompat.ErrRecordNotFound
	}
	return r.scanToolApproval(ctx, `
		SELECT id, user_id, session_id, agent_id, run_id, tool_name, args_hash, token_hash, status, expires_at, consumed_at, created_at, updated_at
		FROM agent_tool_approvals
		WHERE id=? AND tenant_id=?
	`, id, agentpersistence.TenantFromContext(ctx))
}

func (r *Repository) GetByTokenHash(ctx context.Context, tokenHash string) (*ToolApproval, error) {
	if tokenHash == "" {
		return nil, dbcompat.ErrRecordNotFound
	}
	return r.scanToolApproval(ctx, `
		SELECT id, user_id, session_id, agent_id, run_id, tool_name, args_hash, token_hash, status, expires_at, consumed_at, created_at, updated_at
		FROM agent_tool_approvals
		WHERE token_hash=? AND tenant_id=?
	`, tokenHash, agentpersistence.TenantFromContext(ctx))
}

func (r *Repository) ListPending(ctx context.Context, filter ToolApprovalListFilter) ([]*ToolApproval, error) {
	filter.Status = statusPending
	return r.List(ctx, filter)
}

func (r *Repository) List(ctx context.Context, filter ToolApprovalListFilter) ([]*ToolApproval, error) {
	// Owner scope on user_id: UserID > TeamID > DepartmentIDs; UserID=-1 empty; all zero unrestricted.
	// Legacy callers only set UserID; empty filter returns no rows to keep fail-closed for unset scope.
	if filter.UserID == 0 && filter.TeamID == 0 && len(filter.DepartmentIDs) == 0 {
		return []*ToolApproval{}, nil
	}
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	now := filter.Now
	if now.IsZero() {
		now = timeutil.Now()
	}
	status := normalizeApprovalStatusFilter(filter.Status)
	// 租户是第一道作用域：先钉死租户，再叠加 owner 作用域，避免跨租户可见。
	where := []string{"tenant_id=?"}
	args := []interface{}{agentpersistence.TenantFromContext(ctx)}
	ownerPred, ownerArgs := sqlscope.OwnerScopeSQLPredicate("user_id", filter.UserID, filter.TeamID, filter.DepartmentIDs)
	where = append(where, "("+ownerPred+")")
	args = append(args, ownerArgs...)
	if filter.SessionID > 0 {
		where = append(where, "session_id=?")
		args = append(args, filter.SessionID)
	}
	if filter.RunID > 0 {
		where = append(where, "run_id=?")
		args = append(args, filter.RunID)
	}
	switch status {
	case statusPending:
		where = append(where, "status='pending'", "consumed_at IS NULL", "expires_at > ?")
		args = append(args, now)
	case statusExpired:
		where = append(where, "status='pending'", "consumed_at IS NULL", "expires_at <= ?")
		args = append(args, now)
	case statusConsumed:
		where = append(where, "status='consumed'")
	case statusDenied:
		where = append(where, "status='denied'")
	case statusAll:
	default:
		where = append(where, "status='pending'", "consumed_at IS NULL", "expires_at > ?")
		args = append(args, now)
	}
	args = append(args, limit, offset)
	query := fmt.Sprintf(`
		SELECT id, user_id, session_id, agent_id, run_id, tool_name, args_hash, token_hash, status, expires_at, consumed_at, created_at, updated_at
		FROM agent_tool_approvals
		WHERE %s
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, strings.Join(where, " AND "))
	var rows []ToolApproval
	if err := r.db.GORM().WithContext(ctx).Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("查询 Agent 工具审批失败: %w", err)
	}
	items := make([]*ToolApproval, 0, len(rows))
	for i := range rows {
		items = append(items, &rows[i])
	}
	return items, nil
}

func (r *Repository) Consume(ctx context.Context, id int64, now time.Time) (bool, error) {
	if id <= 0 {
		return false, nil
	}
	if now.IsZero() {
		now = timeutil.Now()
	}
	res := r.db.GORM().WithContext(ctx).Exec(`
		UPDATE agent_tool_approvals
		SET status='consumed', consumed_at=CURRENT_TIMESTAMP, updated_at=CURRENT_TIMESTAMP
		WHERE id=? AND tenant_id=? AND status='pending' AND consumed_at IS NULL AND expires_at > ?
	`, id, agentpersistence.TenantFromContext(ctx), now)
	if res.Error != nil {
		return false, fmt.Errorf("消费 Agent 工具审批失败: %w", res.Error)
	}
	return res.RowsAffected == 1, nil
}

func (r *Repository) RotateToken(ctx context.Context, id int64, tokenHash string, now time.Time) (bool, error) {
	if id <= 0 || tokenHash == "" {
		return false, nil
	}
	if now.IsZero() {
		now = timeutil.Now()
	}
	res := r.db.GORM().WithContext(ctx).Exec(`
		UPDATE agent_tool_approvals
		SET token_hash=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=? AND tenant_id=? AND status='pending' AND consumed_at IS NULL AND expires_at > ?
	`, tokenHash, id, agentpersistence.TenantFromContext(ctx), now)
	if res.Error != nil {
		return false, fmt.Errorf("更新 Agent 工具审批 token 失败: %w", res.Error)
	}
	return res.RowsAffected == 1, nil
}

func (r *Repository) Reject(ctx context.Context, id int64, now time.Time) (bool, error) {
	if id <= 0 {
		return false, nil
	}
	if now.IsZero() {
		now = timeutil.Now()
	}
	res := r.db.GORM().WithContext(ctx).Exec(`
		UPDATE agent_tool_approvals
		SET status='denied', updated_at=CURRENT_TIMESTAMP
		WHERE id=? AND tenant_id=? AND status='pending' AND consumed_at IS NULL AND expires_at > ?
	`, id, agentpersistence.TenantFromContext(ctx), now)
	if res.Error != nil {
		return false, fmt.Errorf("拒绝 Agent 工具审批失败: %w", res.Error)
	}
	return res.RowsAffected == 1, nil
}

// DeleteExpired 物理清理已过保留期的终结/过期审批行（M-P2-7）。
//
// 删除范围（满足其一即可，且 updated_at 必须早于 now-retention）：
//   - status IN ('consumed', 'denied', 'expired')：已终结行
//   - status='pending' 且 expires_at <= now：已过期未被消费的 pending 行
//     （数据库不物理写 'expired' 状态，过期体现在 expires_at，此处按时间判定）
//
// 绝不删除未过期（expires_at > now）的 pending 行——它们是待审批的活跃请求。
// retention <= 0 时使用默认保留期（30 天）。调用方应挂载到周期任务（如每日一次）；
// 当前 app 装配层尚无对应调度槽位，由接入方按 defaultApprovalCleanupRetention 调度。
// 返回实际删除行数。
func (r *Repository) DeleteExpired(ctx context.Context, now time.Time, retention time.Duration) (int64, error) {
	if r == nil || r.db == nil {
		return 0, nil
	}
	if now.IsZero() {
		now = timeutil.Now()
	}
	if retention <= 0 {
		retention = defaultApprovalCleanupRetention
	}
	res := r.db.GORM().WithContext(ctx).Exec(`
		DELETE FROM agent_tool_approvals
		WHERE tenant_id = ?
		  AND updated_at < ?
		  AND (
		        status IN ('consumed', 'denied', 'expired')
		        OR (status = 'pending' AND consumed_at IS NULL AND expires_at <= ?)
		      )
	`, agentpersistence.TenantFromContext(ctx), now.Add(-retention), now)
	if res.Error != nil {
		return 0, fmt.Errorf("清理过期 Agent 工具审批失败: %w", res.Error)
	}
	return res.RowsAffected, nil
}

func normalizeApprovalStatusFilter(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", statusPending:
		return statusPending
	case statusAll:
		return statusAll
	case statusConsumed:
		return statusConsumed
	case statusDenied, "rejected":
		return statusDenied
	case statusExpired:
		return statusExpired
	default:
		return statusPending
	}
}

func (r *Repository) scanToolApproval(ctx context.Context, query string, args ...any) (*ToolApproval, error) {
	var item ToolApproval
	res := r.db.GORM().WithContext(ctx).Raw(query, args...).Scan(&item)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, dbcompat.ErrRecordNotFound
	}
	return &item, nil
}
