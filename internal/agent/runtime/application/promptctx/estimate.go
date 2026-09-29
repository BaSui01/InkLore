package enginepromptctx

import (
	"strings"

	agentdomain "InkLore/internal/agent/domain"
	ai "InkLore/internal/llm/domain"
)

// EstimateMessagesTokens 估算一组消息（含工具调用）+ 系统提示词的 token 数。
func EstimateMessagesTokens(modelName, systemPrompt string, messages []ai.ChatMessage, skipIndex int) int {
	tokens := 0
	if strings.TrimSpace(systemPrompt) != "" {
		tokens += agentdomain.EstimateTokenCountForModel(systemPrompt, modelName) + 4
	}
	for i, msg := range messages {
		if i == skipIndex {
			continue
		}
		tokens += agentdomain.EstimateTokenCountForModel(msg.Content, modelName) + 4
		for _, toolCall := range msg.ToolCalls {
			tokens += agentdomain.EstimateTokenCountForModel(toolCall.Function.Name, modelName)
			tokens += agentdomain.EstimateTokenCountForModel(toolCall.Function.Arguments, modelName)
		}
	}
	return tokens
}
