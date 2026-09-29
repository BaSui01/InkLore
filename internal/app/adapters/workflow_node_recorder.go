package adapters

import (
	"context"

	agentpersistence "InkLore/internal/agent/infrastructure/persistence/agentrun"
	tooldomain "InkLore/internal/agent/tool/domain"
)

// WorkflowNodeRecorderAdapter 把 agentrun.Repository 适配为工作流节点的落库端口。
type WorkflowNodeRecorderAdapter struct {
	repo *agentpersistence.Repository
}

// NewWorkflowNodeRecorder 创建工作流节点持久化记录适配器。
func NewWorkflowNodeRecorder(repo *agentpersistence.Repository) tooldomain.WorkflowNodeRecorder {
	if repo == nil {
		return nil
	}
	return &WorkflowNodeRecorderAdapter{repo: repo}
}

var _ tooldomain.WorkflowNodeRecorder = (*WorkflowNodeRecorderAdapter)(nil)

func (a *WorkflowNodeRecorderAdapter) RecordNodeStart(ctx context.Context, runID int64, nodeID, nodeType, inputJSON string) (int64, error) {
	if a == nil || a.repo == nil || runID <= 0 {
		return 0, nil
	}
	return a.repo.CreateNode(agentpersistence.AgentRunNodeCreate{
		RunID:     runID,
		NodeID:    nodeID,
		NodeType:  nodeType,
		InputJSON: inputJSON,
	})
}

func (a *WorkflowNodeRecorderAdapter) RecordNodeComplete(ctx context.Context, nodeRunID int64, outputJSON string) error {
	if a == nil || a.repo == nil || nodeRunID <= 0 {
		return nil
	}
	return a.repo.CompleteNode(nodeRunID, outputJSON)
}

func (a *WorkflowNodeRecorderAdapter) RecordNodeFail(ctx context.Context, nodeRunID int64, outputJSON, errText string) error {
	if a == nil || a.repo == nil || nodeRunID <= 0 {
		return nil
	}
	return a.repo.FailNode(nodeRunID, outputJSON, errText)
}
