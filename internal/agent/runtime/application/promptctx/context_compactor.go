package enginepromptctx

import (
	ai "InkLore/internal/llm/domain"
	aigateway "InkLore/internal/llm/domain/gateway"
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type ContextCompactor struct {
	gateway aigateway.ModelExecutionGateway
	binding AuxiliaryTaskBinding
}

// 未绑定发布配置时不选择编译期默认模型和提示词。
func NewContextCompactor(gateway aigateway.ModelExecutionGateway, customPrompt ...string) *ContextCompactor {
	return &ContextCompactor{gateway: gateway}
}
func NewConfiguredContextCompactor(gateway aigateway.ModelExecutionGateway, binding AuxiliaryTaskBinding) *ContextCompactor {
	return &ContextCompactor{gateway: gateway, binding: binding}
}
func (c *ContextCompactor) GenerateHandoffDocument(ctx context.Context, messages []ai.ChatMessage, modelName string) (string, error) {
	if len(messages) == 0 {
		return "", nil
	}
	if c == nil || !c.binding.Task.Enabled || c.gateway == nil || c.binding.Model.ID <= 0 {
		return "", fmt.Errorf("上下文压缩未配置或已禁用")
	}
	raw, err := json.Marshal(messages)
	if err != nil {
		return "", err
	}
	task, model := c.binding.Task, c.binding.Model
	prompt, err := RenderAuxiliaryPrompt(task.UserPrompt, string(raw), "")
	if err != nil {
		return "", err
	}
	result, err := c.gateway.Execute(ctx, aigateway.ExecutionRequest{Capability: aigateway.CapabilityText, Action: "agent.context.handoff_compaction", Model: model.Model, Provider: model.Provider, Text: &aigateway.TextRequest{Prompt: prompt, SystemPrompt: task.SystemPrompt, Model: model.Model, MaxTokens: task.MaxTokens, Temperature: task.Temperature}, AuditRef: aigateway.AuditRef{Resource: "agent_context_compaction"}})
	if err != nil {
		return "", err
	}
	if result == nil {
		return "", fmt.Errorf("上下文压缩未返回结果")
	}
	return strings.TrimSpace(result.Content), nil
}

func fallbackExtractHandoffDocument(messages []ai.ChatMessage) string {
	userIntents := make([]string, 0)
	completedTools := make([]string, 0)
	constraints := make([]string, 0)
	recentTasks := make([]string, 0)

	for _, msg := range messages {
		switch msg.Role {
		case ai.ChatMessageRoleUser:
			clean := strings.TrimSpace(msg.Content)
			if clean != "" {
				userIntents = append(userIntents, clean)
			}
		case ai.ChatMessageRoleAssistant:
			for _, tc := range msg.ToolCalls {
				completedTools = append(completedTools, fmt.Sprintf("%s(%s)", tc.Function.Name, tc.Function.Arguments))
			}
		case ai.ChatMessageRoleTool:
			if strings.Contains(msg.Content, "error") || strings.Contains(msg.Content, "failed") {
				recentTasks = append(recentTasks, fmt.Sprintf("工具调用异常待处理: %s", msg.Content))
			}
		}
	}

	var doc strings.Builder
	doc.WriteString("# [上下文交接文档]\n\n")

	doc.WriteString("## 1. 核心目标与用户意图 (User Intent & Objectives)\n")
	if len(userIntents) > 0 {
		for _, intent := range userIntents {
			doc.WriteString(fmt.Sprintf("- %s\n", intent))
		}
	} else {
		doc.WriteString("- 维持当前会话任务推进\n")
	}
	doc.WriteString("\n")

	doc.WriteString("## 2. 已完成工作与关键产物 (Completed Milestones & Artifacts)\n")
	if len(completedTools) > 0 {
		limit := len(completedTools)
		if limit > 6 {
			limit = 6
		}
		for _, tool := range completedTools[len(completedTools)-limit:] {
			doc.WriteString(fmt.Sprintf("- 已执行工具: %s\n", tool))
		}
	} else {
		doc.WriteString("- 已完成前序对话与任务分析\n")
	}
	doc.WriteString("\n")

	doc.WriteString("## 3. 关键约束与用户偏好 (Constraints & Decisions)\n")
	if len(constraints) > 0 {
		for _, c := range constraints {
			doc.WriteString(fmt.Sprintf("- %s\n", c))
		}
	} else {
		doc.WriteString("- 严格遵循业务上下文与工具权限限制\n")
	}
	doc.WriteString("\n")

	doc.WriteString("## 4. 当前进行中任务与后续待办 (Pending Tasks & Next Steps)\n")
	if len(recentTasks) > 0 {
		for _, t := range recentTasks {
			doc.WriteString(fmt.Sprintf("- %s\n", t))
		}
	} else {
		doc.WriteString("- 根据用户最新输入继续执行后续步骤\n")
	}

	return doc.String()
}
