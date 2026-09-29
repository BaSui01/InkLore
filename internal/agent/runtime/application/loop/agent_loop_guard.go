package engineloop

import (
	"fmt"
	"strings"

	ai "InkLore/internal/llm/domain"
)

const (
	defaultMaxRepeatCallThreshold = 3 // 默认连续 3 次相同签名调用触发拦截
)

// LoopGuard 负责检测工具死循环、参数振荡并在接近最大轮次时注入收敛引导
type LoopGuard struct {
	toolCallSignatures map[string]int
	maxTurns           int
	maxRepeatCalls     int
}

// LoopGuardOption 配置函数
type LoopGuardOption func(*LoopGuard)

// WithMaxRepeatCalls 设置最大允许重复调用相同工具参数的次数
func WithMaxRepeatCalls(threshold int) LoopGuardOption {
	return func(g *LoopGuard) {
		if threshold > 0 {
			g.maxRepeatCalls = threshold
		}
	}
}

// NewLoopGuard 创建 LoopGuard
func NewLoopGuard(maxTurns int, opts ...LoopGuardOption) *LoopGuard {
	g := &LoopGuard{
		toolCallSignatures: make(map[string]int),
		maxTurns:           maxTurns,
		maxRepeatCalls:     defaultMaxRepeatCallThreshold,
	}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

// CheckRepeatCall 检查并记录单次工具调用是否构成死循环
func (g *LoopGuard) CheckRepeatCall(toolName, args string) (bool, string) {
	sig := toolName + ":" + strings.TrimSpace(args)
	g.toolCallSignatures[sig]++
	if g.toolCallSignatures[sig] >= g.maxRepeatCalls {
		guardMsg := fmt.Sprintf("[Loop Guard 拦截] 检测到连续 %d 次重复调用相同参数的工具 %s。请停止重复调用，基于已有观察结果或调整参数后继续。", g.maxRepeatCalls, toolName)
		return true, guardMsg
	}
	return false, ""
}

// ApplyConvergenceHint 如果明确指定了最大轮次且接近上限，在消息末尾追加系统收敛指引
func (g *LoopGuard) ApplyConvergenceHint(turn int, messages []ai.ChatMessage) []ai.ChatMessage {
	if g.maxTurns > 0 && turn == g.maxTurns-1 && g.maxTurns > 3 {
		copied := append([]ai.ChatMessage(nil), messages...)
		copied = append(copied, ai.ChatMessage{
			Role:    ai.ChatMessageRoleSystem,
			Content: "[系统提示: 执行即将达到最大轮次上限，请基于当前已获得的观察结果完成最终总结输出，避免发起新的工具调用]",
		})
		return copied
	}
	return messages
}
