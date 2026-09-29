package enginepromptctx

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	ai "InkLore/internal/llm/domain"
)

const (
	defaultCompactionThresholdRatio = 0.80 // 默认 80% 上下文阈值触发交接压缩
	defaultContextKeepRecentTurns   = 2    // 压缩时保留最近 2 轮（约 4 条）最新消息
	defaultContextFoldMinChars      = 250  // 超过 250 字符的早期工具结果进入折叠候选
	defaultReserveRatio             = 0.15 // 预留 15% 窗口给模型生成与系统指令
)

type RequestMeta struct {
	Model            string
	SystemPrompt     string
	MaxContextTokens int
}

type ContextManagerOptions struct {
	MaxContextTokens         int
	KeepRecentTurns          int
	ReserveTokens            int
	CompactionThresholdRatio float64
	DisableCompaction        bool
	ModelName                string
	Compactor                *ContextCompactor
}

type ContextManager struct {
	opts ContextManagerOptions
}

func NewContextManager(opts ContextManagerOptions) *ContextManager {
	if opts.KeepRecentTurns <= 0 {
		opts.KeepRecentTurns = defaultContextKeepRecentTurns
	}
	if opts.CompactionThresholdRatio <= 0 || opts.CompactionThresholdRatio > 1.0 {
		opts.CompactionThresholdRatio = defaultCompactionThresholdRatio
	}
	return &ContextManager{opts: opts}
}

type ContextOptimizationResult struct {
	HandoffDocument     string
	Messages            []ai.ChatMessage
	FoldedCount         int
	CompactedCount      int
	EstimatedTokens     int
	MaxTokens           int
	HandoffDocGenerated bool
	BudgetExceeded      bool
}

func (m *ContextManager) OptimizeMessages(ctx context.Context, meta RequestMeta, messages []ai.ChatMessage) ContextOptimizationResult {
	maxTokens := m.opts.MaxContextTokens
	if meta.MaxContextTokens > 0 {
		maxTokens = meta.MaxContextTokens
	}

	if maxTokens <= 0 || len(messages) == 0 {
		return ContextOptimizationResult{
			Messages:        messages,
			EstimatedTokens: EstimateMessagesTokens(meta.Model, meta.SystemPrompt, messages, -1),
			MaxTokens:       maxTokens,
		}
	}

	currentTokens := EstimateMessagesTokens(meta.Model, meta.SystemPrompt, messages, -1)
	compactionThreshold := int(float64(maxTokens) * m.opts.CompactionThresholdRatio)

	keepCount := m.opts.KeepRecentTurns * 2
	if !m.opts.DisableCompaction && currentTokens >= compactionThreshold && len(messages) > keepCount+1 {
		cutoff := len(messages) - keepCount
		if cutoff > 0 {
			toCompress := messages[:cutoff]
			keptRecent := messages[cutoff:]

			modelName := m.opts.ModelName
			if meta.Model != "" {
				modelName = meta.Model
			}

			compactor := m.opts.Compactor
			if compactor == nil {
				compactor = NewContextCompactor(nil)
			}

			// 模型侧压缩需要绑定「辅助任务 + 辅助模型」（见 AGENTS.md 与 INK-050）。
			// 未绑定时退化为本地确定性交接文档：上下文压缩不至于完全失效，
			// 更不会因为辅助模型缺失而阻断主流程。
			handoffDoc, err := compactor.GenerateHandoffDocument(ctx, toCompress, modelName)
			if err != nil || strings.TrimSpace(handoffDoc) == "" {
				handoffDoc = fallbackExtractHandoffDocument(toCompress)
			}
			if strings.TrimSpace(handoffDoc) != "" {
				newMessages := make([]ai.ChatMessage, 0, 1+len(keptRecent))
				newMessages = append(newMessages, ai.ChatMessage{
					Role:    ai.ChatMessageRoleSystem,
					Content: fmt.Sprintf("[前序对话已压缩至上下文交接文档，已忽略前 %d 条旧历史]\n%s", len(toCompress), handoffDoc),
				})
				newMessages = append(newMessages, keptRecent...)

				newEstimated := EstimateMessagesTokens(meta.Model, meta.SystemPrompt, newMessages, -1)
				return ContextOptimizationResult{
					Messages:            newMessages,
					CompactedCount:      len(toCompress),
					HandoffDocGenerated: true,
					HandoffDocument:     handoffDoc,
					EstimatedTokens:     newEstimated,
					MaxTokens:           maxTokens,
					BudgetExceeded:      newEstimated > maxTokens,
				}
			}
		}
	}

	reserve := int(float64(maxTokens) * defaultReserveRatio)
	if m.opts.ReserveTokens > 0 {
		reserve = m.opts.ReserveTokens
	}
	budget := maxTokens - reserve

	if currentTokens <= budget {
		return ContextOptimizationResult{
			Messages:        messages,
			EstimatedTokens: currentTokens,
			MaxTokens:       maxTokens,
		}
	}

	out := append([]ai.ChatMessage(nil), messages...)
	folded := 0

	cutoffIndex := len(out) - keepCount
	if cutoffIndex < 0 {
		cutoffIndex = 0
	}

	for i := 0; i < cutoffIndex; i++ {
		if EstimateMessagesTokens(meta.Model, meta.SystemPrompt, out, -1) <= budget {
			break
		}
		msg := out[i]
		if msg.Role != ai.ChatMessageRoleTool {
			continue
		}
		if len(msg.Content) < defaultContextFoldMinChars || strings.Contains(msg.Content, "pending_approval") {
			continue
		}

		out[i].Content = summarizeToolContent(msg.Content)
		folded++
	}

	finalTokens := EstimateMessagesTokens(meta.Model, meta.SystemPrompt, out, -1)
	return ContextOptimizationResult{
		Messages:        out,
		FoldedCount:     folded,
		EstimatedTokens: finalTokens,
		MaxTokens:       maxTokens,
		BudgetExceeded:  finalTokens > maxTokens,
	}
}

func summarizeToolContent(raw string) string {
	raw = strings.TrimSpace(raw)
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err == nil {
		status, _ := obj["status"].(string)
		msg, _ := obj["message"].(string)
		summary := fmt.Sprintf("[工具结果已折叠 | 状态: %s", status)
		if msg != "" {
			summary += fmt.Sprintf(" | 描述: %s", msg)
		}
		if total, ok := obj["total"].(float64); ok {
			summary += fmt.Sprintf(" | 数量: %d", int(total))
		}
		summary += fmt.Sprintf(" | 原长度: %d 字符]", len(raw))
		return summary
	}

	runes := []rune(raw)
	previewLen := 60
	if len(runes) > previewLen {
		return fmt.Sprintf("[工具结果已折叠 | 原 %d 字符，前缀: %s…]", len(runes), string(runes[:previewLen]))
	}
	return raw
}
