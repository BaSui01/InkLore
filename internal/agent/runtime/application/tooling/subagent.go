package enginetooling

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	agentdomain "InkLore/internal/agent/domain"
)

// SubagentInvoker 子代理底层调用器接口（解耦 Agent 递归调用）
type SubagentInvoker interface {
	InvokeSubagent(ctx context.Context, subagentID int64, subagentName string, taskPrompt string, rc *agentdomain.RuntimeContext) (*agentdomain.SubagentDelegationResult, error)
}

// SubagentDelegationTool 主 Agent 委派子任务给子智能体的标准工具
type SubagentDelegationTool struct {
	invoker    SubagentInvoker
	subagents  []agentdomain.SubagentSpec
	capability agentdomain.ToolCapability
}

// NewSubagentDelegationTool 创建子代理委派工具
func NewSubagentDelegationTool(invoker SubagentInvoker, subagents []agentdomain.SubagentSpec) *SubagentDelegationTool {
	return &SubagentDelegationTool{
		invoker:   invoker,
		subagents: subagents,
		capability: agentdomain.ToolCapability{
			Name:            "delegate_subagent",
			Type:            "subagent",
			RiskLevel:       CapabilityRiskMedium,
			ApprovalPolicy:  ApprovalNever,
			AuditLevel:      AuditSummary,
			SideEffectLevel: ToolSideEffectMutatesState,
			Timeout:         120 * time.Second,
		},
	}
}

func (t *SubagentDelegationTool) Name() string {
	return "delegate_subagent"
}

func (t *SubagentDelegationTool) Description() string {
	var subagentList strings.Builder
	subagentList.WriteString("委派特定子任务给专门的子智能体。可用的子智能体列表:\n")
	for _, sub := range t.subagents {
		if sub.Enabled {
			subagentList.WriteString(fmt.Sprintf("- %s: %s\n", sub.Name, sub.Description))
		}
	}
	return subagentList.String()
}

func (t *SubagentDelegationTool) Parameters() any {
	names := make([]string, 0, len(t.subagents))
	for _, s := range t.subagents {
		if s.Enabled {
			names = append(names, s.Name)
		}
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"subagent_name": map[string]any{
				"type":        "string",
				"enum":        names,
				"description": "目标子智能体名称",
			},
			"task_prompt": map[string]any{
				"type":        "string",
				"description": "委派给子智能体的具体任务描述和执行目标",
			},
			"context_summary": map[string]any{
				"type":        "string",
				"description": "需要同步给子智能体的关键背景上下文或前置结果",
			},
		},
		"required": []string{"subagent_name", "task_prompt"},
	}
}

func (t *SubagentDelegationTool) PromptGuidelines() string {
	return `当面对大型复合任务、专业领域拆解（例如专门的文案生成、视觉分析、代码测试）时，请使用 delegate_subagent 将子任务委派给相应的子智能体。
委派时请提供清晰明确的 task_prompt 与必要的 context_summary。子智能体将在隔离上下文中执行并返回总结。`
}

func (t *SubagentDelegationTool) Capability() agentdomain.ToolCapability {
	return t.capability
}

func (t *SubagentDelegationTool) Execute(ctx context.Context, rc *agentdomain.RuntimeContext, rawArgs json.RawMessage) (*agentdomain.ToolResult, error) {
	var args struct {
		SubagentName   string `json:"subagent_name"`
		TaskPrompt     string `json:"task_prompt"`
		ContextSummary string `json:"context_summary,omitempty"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid delegate_subagent args: %w", err)
	}

	if t.invoker == nil {
		return nil, fmt.Errorf("subagent invoker is not configured")
	}

	var targetSpec *agentdomain.SubagentSpec
	for _, s := range t.subagents {
		if s.Name == args.SubagentName && s.Enabled {
			targetSpec = &s
			break
		}
	}
	if targetSpec == nil {
		return nil, fmt.Errorf("subagent %q not found or not enabled", args.SubagentName)
	}

	prompt := args.TaskPrompt
	if strings.TrimSpace(args.ContextSummary) != "" {
		prompt = fmt.Sprintf("背景上下文:\n%s\n\n执行任务:\n%s", args.ContextSummary, prompt)
	}

	res, err := t.invoker.InvokeSubagent(ctx, targetSpec.ID, targetSpec.Name, prompt, rc)
	if err != nil {
		return &agentdomain.ToolResult{
			ToolName: t.Name(),
			Content:  fmt.Sprintf("子智能体 [%s] 执行失败: %v", args.SubagentName, err),
			IsError:  true,
		}, nil
	}

	summaryContent := fmt.Sprintf("子智能体 [%s] 执行完成 (耗时 %d 轮):\n%s", args.SubagentName, res.Turns, res.Content)
	return &agentdomain.ToolResult{
		ToolName: t.Name(),
		Content:  summaryContent,
		Structured: map[string]any{
			"subagent_id":   res.SubagentID,
			"subagent_name": res.Name,
			"turns":         res.Turns,
			"status":        res.Status,
		},
	}, nil
}
