package domain

import "context"

// WorkflowNodeDef 描述工作流中的单个步骤节点定义（DAG 拓扑节点）。
type WorkflowNodeDef struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Type         string   `json:"type"` // "subagent" | "llm_task" | "validator"
	SubagentName string   `json:"subagent_name,omitempty"`
	Dependencies []string `json:"dependencies"` // 前置依赖节点 ID 列表
	TaskPrompt   string   `json:"task_prompt,omitempty"`
	TaskKey      string   `json:"task_key,omitempty"`
	Description  string   `json:"description,omitempty"`
}

// WorkflowManifest 描述完整的工作流图规范。
type WorkflowManifest struct {
	WorkflowID  string            `json:"workflow_id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Category    string            `json:"category"` // "workflow"
	Nodes       []WorkflowNodeDef `json:"nodes"`
}

// WorkflowNodeRun 描述工作流中单个节点的运行状态与输出结果。
type WorkflowNodeRun struct {
	NodeID     string         `json:"node_id"`
	NodeType   string         `json:"node_type"`
	Name       string         `json:"name"`
	Status     string         `json:"status"` // "pending" | "running" | "success" | "failed"
	Input      map[string]any `json:"input,omitempty"`
	Output     map[string]any `json:"output,omitempty"`
	Error      string         `json:"error,omitempty"`
	DurationMs int64          `json:"duration_ms,omitempty"`
}

// WorkflowExecutionResult 工作流执行的最终汇总结果。
type WorkflowExecutionResult struct {
	WorkflowID    string                      `json:"workflow_id"`
	Manifest      WorkflowManifest            `json:"manifest"`
	Nodes         map[string]*WorkflowNodeRun `json:"nodes"`
	FinalArtifact string                      `json:"final_artifact"`
	Success       bool                        `json:"success"`
	Error         string                      `json:"error,omitempty"`
}

// WorkflowNodeRecorder 允许工作流将步骤节点实时落库至 agent_run_nodes。
type WorkflowNodeRecorder interface {
	RecordNodeStart(ctx context.Context, runID int64, nodeID, nodeType, inputJSON string) (int64, error)
	RecordNodeComplete(ctx context.Context, nodeRunID int64, outputJSON string) error
	RecordNodeFail(ctx context.Context, nodeRunID int64, outputJSON, errText string) error
}
