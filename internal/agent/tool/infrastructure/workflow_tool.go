package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"InkLore/internal/agent/tool/domain"
	"InkLore/internal/platform/logger"
	"InkLore/internal/platform/timeutil"
)

// WorkflowTool 统一工作流工具封装（Workflow-as-a-Tool）。
type WorkflowTool struct {
	manifest   domain.WorkflowManifest
	generator  domain.TextGenerator
	recorder   domain.WorkflowNodeRecorder
	parameters []domain.ToolParam
}

// NewWorkflowTool 创建通用工作流工具实例。
func NewWorkflowTool(
	manifest domain.WorkflowManifest,
	generator domain.TextGenerator,
	recorder domain.WorkflowNodeRecorder,
	parameters []domain.ToolParam,
) *WorkflowTool {
	return &WorkflowTool{
		manifest:   manifest,
		generator:  generator,
		recorder:   recorder,
		parameters: parameters,
	}
}

func (t *WorkflowTool) Descriptor() domain.ToolDescriptor {
	return domain.ToolDescriptor{
		ID:          t.manifest.WorkflowID,
		Name:        t.manifest.Name,
		Description: t.manifest.Description,
		Category:    "workflow",
		Parameters:  t.parameters,
	}
}

func (t *WorkflowTool) Execute(ctx context.Context, input map[string]any) (domain.ToolResult, error) {
	if t.generator == nil {
		return domain.ToolResult{
			ToolID:  t.manifest.WorkflowID,
			Success: false,
			Summary: fmt.Sprintf("工作流 [%s] 执行失败: 未配置可用的大模型网关，无法驱动子代理流水线", t.manifest.Name),
		}, nil
	}

	nodesState := make(map[string]*domain.WorkflowNodeRun, len(t.manifest.Nodes))
	for _, node := range t.manifest.Nodes {
		nodesState[node.ID] = &domain.WorkflowNodeRun{
			NodeID:   node.ID,
			NodeType: node.Type,
			Name:     node.Name,
			Status:   "pending",
			Input:    input,
		}
	}

	orderedNodes, err := topologicalSort(t.manifest.Nodes)
	if err != nil {
		return domain.ToolResult{
			ToolID:  t.manifest.WorkflowID,
			Success: false,
			Summary: fmt.Sprintf("工作流拓扑排序失败: %v", err),
		}, nil
	}

	var terminalArtifact string
	allSuccess := true

	for _, node := range orderedNodes {
		if err := ctx.Err(); err != nil {
			return domain.ToolResult{
				ToolID:  t.manifest.WorkflowID,
				Success: false,
				Summary: fmt.Sprintf("工作流因上下文取消而中断: %v", err),
			}, err
		}

		state := nodesState[node.ID]

		// 检查前置依赖是否全部成功
		depFailed := false
		for _, depID := range node.Dependencies {
			depState, ok := nodesState[depID]
			if !ok || depState.Status != "success" {
				depFailed = true
				break
			}
		}

		if depFailed {
			state.Status = "failed"
			state.Error = "前置依赖节点执行失败，已跳过此步骤"
			allSuccess = false
			continue
		}

		// 节点开始执行
		state.Status = "running"
		startTime := timeutil.Now()

		var nodeRunID int64
		inputBytes, _ := json.Marshal(input)
		if t.recorder != nil {
			recID, recErr := t.recorder.RecordNodeStart(ctx, 0, node.ID, node.Type, string(inputBytes))
			if recErr == nil {
				nodeRunID = recID
			} else {
				logger.WarnContext(ctx, "记录工作流节点启动失败", "node_id", node.ID, "error", recErr)
			}
		}

		// 组装节点专属 Prompt
		prompt := buildNodePrompt(node, input, nodesState)
		taskKey := node.TaskKey
		if taskKey == "" {
			taskKey = "draft"
		}

		res, genErr := t.generator.GenerateText(ctx, domain.TextRequest{
			Prompt:             prompt,
			TaskKey:            taskKey,
			DefaultTemperature: 0.7,
		})
		duration := time.Since(startTime).Milliseconds()
		state.DurationMs = duration

		if genErr != nil {
			state.Status = "failed"
			state.Error = genErr.Error()
			allSuccess = false
			if t.recorder != nil && nodeRunID > 0 {
				_ = t.recorder.RecordNodeFail(ctx, nodeRunID, "", genErr.Error())
			}
			logger.WarnContext(ctx, "工作流节点执行失败", "node_id", node.ID, "error", genErr)
		} else {
			state.Status = "success"
			state.Output = map[string]any{"text": res.Content, "model": res.Model}
			terminalArtifact = res.Content
			if t.recorder != nil && nodeRunID > 0 {
				outBytes, _ := json.Marshal(state.Output)
				_ = t.recorder.RecordNodeComplete(ctx, nodeRunID, string(outBytes))
			}
		}
	}

	summaryMsg := fmt.Sprintf("工作流 [%s] 执行完成，共执行 %d 个节点", t.manifest.Name, len(orderedNodes))
	if !allSuccess {
		summaryMsg = fmt.Sprintf("工作流 [%s] 部分节点未完成", t.manifest.Name)
	}

	return domain.ToolResult{
		ToolID:  t.manifest.WorkflowID,
		Success: allSuccess,
		Summary: summaryMsg,
		Details: map[string]any{
			"workflow_id":    t.manifest.WorkflowID,
			"manifest":       t.manifest,
			"node_results":   nodesState,
			"final_artifact": terminalArtifact,
		},
	}, nil
}

func buildNodePrompt(node domain.WorkflowNodeDef, input map[string]any, states map[string]*domain.WorkflowNodeRun) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("【工作流节点任务：%s】\n", node.Name))
	if node.Description != "" {
		sb.WriteString(fmt.Sprintf("职责说明：%s\n", node.Description))
	}
	if node.SubagentName != "" {
		sb.WriteString(fmt.Sprintf("执行子代理角色：%s\n", node.SubagentName))
	}
	sb.WriteString(fmt.Sprintf("指令与要求：%s\n\n", node.TaskPrompt))

	sb.WriteString("【全局输入信息】\n")
	for k, v := range input {
		if strVal, ok := v.(string); ok && strVal != "" {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", k, strVal))
		}
	}

	if len(node.Dependencies) > 0 {
		sb.WriteString("\n【前序步骤协同产物】\n")
		for _, depID := range node.Dependencies {
			depState, ok := states[depID]
			if ok && depState.Output != nil {
				if text, hasText := depState.Output["text"].(string); hasText && text != "" {
					sb.WriteString(fmt.Sprintf("--- 前置步骤 [%s (%s)] 产物 ---\n%s\n\n", depState.Name, depID, text))
				}
			}
		}
	}

	return sb.String()
}

// topologicalSort 对有向无环图节点进行拓扑排序
func topologicalSort(nodes []domain.WorkflowNodeDef) ([]domain.WorkflowNodeDef, error) {
	nodeMap := make(map[string]domain.WorkflowNodeDef, len(nodes))
	inDegree := make(map[string]int, len(nodes))
	adjList := make(map[string][]string, len(nodes))

	for _, n := range nodes {
		nodeMap[n.ID] = n
		inDegree[n.ID] = len(n.Dependencies)
		for _, dep := range n.Dependencies {
			adjList[dep] = append(adjList[dep], n.ID)
		}
	}

	queue := make([]string, 0)
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	sorted := make([]domain.WorkflowNodeDef, 0, len(nodes))
	for len(queue) > 0 {
		currID := queue[0]
		queue = queue[1:]

		if n, ok := nodeMap[currID]; ok {
			sorted = append(sorted, n)
		}

		for _, neighbor := range adjList[currID] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	if len(sorted) != len(nodes) {
		return nodes, fmt.Errorf("工作流定义存在循环依赖环")
	}

	return sorted, nil
}
