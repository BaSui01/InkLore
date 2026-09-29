package agentactivitylog

import (
	"context"
	"fmt"
	"time"

	agentpersistence "InkLore/internal/agent/infrastructure/persistence"
	"InkLore/internal/platform/nulltype"
)

type AgentActivityLogRepository struct {
	db *agentpersistence.DB
}

type AgentActivityLog struct {
	CreatedAt   time.Time
	NodeID      string
	LogPath     string
	Content     string
	Summary     string
	RunID       nulltype.Int64
	ParentRunID nulltype.Int64
	AgentID     nulltype.Int64
	ID          int64
}

type AgentActivityLogCreate struct {
	NodeID      string
	LogPath     string
	Content     string
	Summary     string
	RunID       nulltype.Int64
	ParentRunID nulltype.Int64
	AgentID     nulltype.Int64
}

func NewAgentActivityLogRepository(db any) *AgentActivityLogRepository {
	return &AgentActivityLogRepository{db: agentpersistence.NewDB(db)}
}

// Create 写入一条活动日志；tenant_id 取自请求上下文（INK-049）。
func (r *AgentActivityLogRepository) Create(ctx context.Context, in AgentActivityLogCreate) (int64, error) {
	var id int64
	if err := r.db.GORM().WithContext(ctx).Raw(`
		INSERT INTO agent_activity_logs (tenant_id, run_id, parent_run_id, agent_id, node_id, log_path, content, summary, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		RETURNING id
	`, agentpersistence.TenantFromContext(ctx), nulltype.NullableIntArg(in.RunID), nulltype.NullableIntArg(in.ParentRunID), nulltype.NullableIntArg(in.AgentID), in.NodeID, in.LogPath, in.Content, in.Summary).Scan(&id).Error; err != nil {
		return 0, fmt.Errorf("创建 Agent 活动日志失败: %w", err)
	}
	return id, nil
}

// ListByRunID 按租户读取某次运行的活动日志。
func (r *AgentActivityLogRepository) ListByRunID(ctx context.Context, runID int64) ([]AgentActivityLog, error) {
	if runID <= 0 {
		return nil, nil
	}
	items := make([]AgentActivityLog, 0)
	if err := r.db.GORM().WithContext(ctx).Raw(`
		SELECT id, run_id, parent_run_id, agent_id, node_id, log_path, content, summary, created_at
		FROM agent_activity_logs
		WHERE tenant_id = ? AND (run_id = ? OR parent_run_id = ?)
		ORDER BY id ASC
	`, agentpersistence.TenantFromContext(ctx), runID, runID).Scan(&items).Error; err != nil {
		return nil, fmt.Errorf("查询 Agent 活动日志失败: %w", err)
	}
	return items, nil
}
