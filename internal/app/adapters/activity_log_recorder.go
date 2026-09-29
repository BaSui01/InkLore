package adapters

import (
	"context"
	"fmt"
	"strings"

	agentactivitylog "InkLore/internal/agent/infrastructure/persistence/agentactivitylog"
	"InkLore/internal/agent/runtime/application/ports"
	"InkLore/internal/platform/nulltype"
)

// ActivityLogRecorder 把运行活动摘要写入 agent_activity_logs（INK-037）。
//
// 该适配器让 runtime 只依赖自己定义的 ActivityLogStore 端口，具体落库方式由组合根决定。
type ActivityLogRecorder struct {
	repo *agentactivitylog.AgentActivityLogRepository
}

// NewActivityLogRecorder 构造适配器；repo 为空时返回 nil（调用方据此退化为不记录）。
func NewActivityLogRecorder(repo *agentactivitylog.AgentActivityLogRepository) *ActivityLogRecorder {
	if repo == nil {
		return nil
	}
	return &ActivityLogRecorder{repo: repo}
}

var _ ports.ActivityLogStore = (*ActivityLogRecorder)(nil)

// Log 写入一条活动摘要；tenant_id 由仓储从请求上下文取值。
func (r *ActivityLogRecorder) Log(ctx context.Context, record ports.ActivityLogRecord) error {
	if r == nil || r.repo == nil {
		return nil
	}
	if record.RunID <= 0 {
		return fmt.Errorf("活动日志缺少 run_id")
	}
	var runRef nulltype.Int64
	if record.RunID > 0 {
		runRef = nulltype.Int64{Int64: record.RunID, Valid: true}
	}
	var agentRef nulltype.Int64
	if record.AgentID > 0 {
		agentRef = nulltype.Int64{Int64: record.AgentID, Valid: true}
	}
	_, err := r.repo.Create(ctx, agentactivitylog.AgentActivityLogCreate{
		RunID:   runRef,
		AgentID: agentRef,
		NodeID:  strings.TrimSpace(record.NodeID),
		LogPath: strings.TrimSpace(record.LogPath),
		Content: record.Content,
		Summary: strings.TrimSpace(record.Summary),
	})
	if err != nil {
		return fmt.Errorf("写入活动日志失败: %w", err)
	}
	return nil
}

// ListByRunID 读取某次运行的活动日志。
func (r *ActivityLogRecorder) ListByRunID(ctx context.Context, runID int64) ([]ports.ActivityLog, error) {
	if r == nil || r.repo == nil {
		return nil, nil
	}
	rows, err := r.repo.ListByRunID(ctx, runID)
	if err != nil {
		return nil, err
	}
	items := make([]ports.ActivityLog, 0, len(rows))
	for _, row := range rows {
		items = append(items, ports.ActivityLog{
			ID:        row.ID,
			RunID:     row.RunID.Int64,
			NodeID:    row.NodeID,
			LogPath:   row.LogPath,
			Summary:   row.Summary,
			Content:   row.Content,
			CreatedAt: row.CreatedAt,
		})
	}
	return items, nil
}
