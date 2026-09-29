package ports

import (
	"context"
	"time"
)

// RunSummary 是一次运行的**列表摘要**，供前端「Agent 轨迹」视图使用（INK-041）。
type RunSummary struct {
	CreatedAt  time.Time  `json:"created_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Status     string     `json:"status"`
	InputBrief string     `json:"input_brief"`
	Error      string     `json:"error"`
	RunID      int64      `json:"run_id"`
	AgentID    int64      `json:"agent_id"`
}

// RunNodeSummary 描述工作流或子代理在某次运行中的节点执行记录（agent_run_nodes）。
type RunNodeSummary struct {
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	NodeID     string     `json:"node_id"`
	NodeType   string     `json:"node_type"`
	Status     string     `json:"status"`
	InputJSON  string     `json:"input_json"`
	OutputJSON string     `json:"output_json"`
	Error      string     `json:"error"`
	ID         int64      `json:"id"`
	RunID      int64      `json:"run_id"`
}

// RunLister 分页列出**当前租户**的运行以及单次运行包含的工作流节点。
type RunLister interface {
	ListRuns(ctx context.Context, limit, offset int) ([]RunSummary, int64, error)
	ListNodes(ctx context.Context, runID int64) ([]RunNodeSummary, error)
}
