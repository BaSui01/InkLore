package adapters

import (
	"context"
	"strings"

	agentrun "InkLore/internal/agent/infrastructure/persistence/agentrun"
	"InkLore/internal/agent/runtime/application/ports"
	"InkLore/internal/platform/tenantcontext"
)

// AgentRunLister 把 agent_runs 仓储适配为「Agent 轨迹」列表端口（INK-041）。
//
// 复用既有仓储的分页与租户作用域实现（fail-closed：无租户返回空集），
// 只把持久化模型转换成列表摘要，避免把 JSONB 快照整份塞进列表响应。
// 时间列按库内 TIMESTAMPTZ 原值返回（UTC 瞬时），由前端按用户时区展示。
type AgentRunLister struct {
	repository *agentrun.Repository
}

// NewAgentRunLister 构造列表适配器；repository 为 nil 时退化为空列表。
func NewAgentRunLister(repository *agentrun.Repository) *AgentRunLister {
	return &AgentRunLister{repository: repository}
}

var _ ports.RunLister = (*AgentRunLister)(nil)

// ListRuns 按租户作用域分页列出运行（创建时间倒序）。
func (l *AgentRunLister) ListRuns(ctx context.Context, limit, offset int) ([]ports.RunSummary, int64, error) {
	if l == nil || l.repository == nil {
		return []ports.RunSummary{}, 0, nil
	}
	runs, total, err := l.repository.ListRunsForTenant(tenantcontext.OrDefault(ctx), limit, offset)
	if err != nil {
		return nil, 0, err
	}
	summaries := make([]ports.RunSummary, 0, len(runs))
	for index := range runs {
		run := runs[index]
		summary := ports.RunSummary{
			RunID:      run.ID,
			AgentID:    run.AgentID,
			Status:     strings.TrimSpace(run.Status),
			Error:      strings.TrimSpace(run.Error),
			InputBrief: briefSnapshot(run.InputJSON, 160),
			CreatedAt:  run.CreatedAt,
		}
		if run.FinishedAt.Valid {
			finishedAt := run.FinishedAt.Time
			summary.FinishedAt = &finishedAt
		}
		summaries = append(summaries, summary)
	}
	return summaries, total, nil
}

// ListNodes 列出某次运行的工作流节点执行明细。
func (l *AgentRunLister) ListNodes(ctx context.Context, runID int64) ([]ports.RunNodeSummary, error) {
	if l == nil || l.repository == nil || runID <= 0 {
		return []ports.RunNodeSummary{}, nil
	}
	nodes, err := l.repository.ListNodes(runID)
	if err != nil {
		return nil, err
	}
	summaries := make([]ports.RunNodeSummary, 0, len(nodes))
	for _, n := range nodes {
		item := ports.RunNodeSummary{
			ID:         n.ID,
			RunID:      n.RunID,
			NodeID:     n.NodeID,
			NodeType:   n.NodeType,
			Status:     n.Status,
			InputJSON:  n.InputJSON,
			OutputJSON: n.OutputJSON,
			Error:      n.Error,
			CreatedAt:  n.CreatedAt,
			UpdatedAt:  n.UpdatedAt,
		}
		if n.StartedAt.Valid {
			t := n.StartedAt.Time
			item.StartedAt = &t
		}
		if n.FinishedAt.Valid {
			t := n.FinishedAt.Time
			item.FinishedAt = &t
		}
		summaries = append(summaries, item)
	}
	return summaries, nil
}

// briefSnapshot 把输入快照压成单行摘要；空快照返回空串而不是 "{}"。
func briefSnapshot(raw string, limit int) string {
	trimmed := strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
	if trimmed == "" || trimmed == "{}" || trimmed == "null" {
		return ""
	}
	runes := []rune(trimmed)
	if len(runes) > limit {
		return string(runes[:limit]) + "…"
	}
	return trimmed
}
