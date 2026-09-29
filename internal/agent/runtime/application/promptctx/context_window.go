package enginepromptctx

import (
	"fmt"
	"strings"

	ai "InkLore/internal/llm/domain"
)

const (
	contextFoldKeepRecent  = 8   // 最近 N 条消息不折叠（约等于最近 2 轮）
	contextFoldMinChars    = 400 // 小于该长度的工具结果不值得折叠
	contextFoldReserveFrac = 10  // 预留 1/N 的窗口给本轮生成
)

// FoldOldToolResults 当消息估算 token 超出窗口预算时，把最旧的大块工具结果
// 替换为占位摘要，防止长会话撑爆上下文。
func FoldOldToolResults(modelName, systemPrompt string, messages []ai.ChatMessage, maxContextTokens int) ([]ai.ChatMessage, int) {
	if maxContextTokens <= 0 || len(messages) == 0 {
		return messages, 0
	}
	budget := maxContextTokens - maxContextTokens/contextFoldReserveFrac
	if EstimateMessagesTokens(modelName, systemPrompt, messages, -1) <= budget {
		return messages, 0
	}
	out := append([]ai.ChatMessage(nil), messages...)
	folded := 0
	for i := 0; i < len(out)-contextFoldKeepRecent; i++ {
		if EstimateMessagesTokens(modelName, systemPrompt, out, -1) <= budget {
			break
		}
		msg := out[i]
		if msg.Role != ai.ChatMessageRoleTool {
			continue
		}
		if len(msg.Content) < contextFoldMinChars || strings.Contains(msg.Content, "pending_approval") {
			continue
		}
		runes := []rune(msg.Content)
		preview := runes
		if len(preview) > 80 {
			preview = preview[:80]
		}
		out[i].Content = fmt.Sprintf("[工具结果已折叠以控制上下文，原 %d 字符，开头: %s…]", len(runes), string(preview))
		folded++
	}
	return out, folded
}
