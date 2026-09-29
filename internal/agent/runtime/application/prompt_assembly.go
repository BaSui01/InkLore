package runtimeapp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	enginepromptctx "InkLore/internal/agent/runtime/application/promptctx"
	"InkLore/internal/platform/cache"
)

// promptStableSectionTTL 稳定前缀的缓存时长：内容只随 Agent 定义与工具集变化。
const promptStableSectionTTL = 30 * time.Minute

// promptAssembly 把 Agent 定义装配为结构化 Prompt 段，并缓存其中的稳定前缀（INK-014）。
//
// 分工：
//   - 稳定前缀（核心指令 / Agent 定义 / 工具指引）按内容哈希缓存，命中时省掉重建与 token 估算；
//   - 易变段（本轮用户输入）永不缓存，保证每轮都是最新输入。
//
// 缓存走 platform/cache.ServiceCache：Redis 可用时由 Redis 承担；Redis 抖动时按组合根配置
// 回退本机内存并告警，绝不因为缓存不可用而让生成失败。
type promptAssembly struct {
	sections *cache.ServiceCache
}

func newPromptAssembly(sections *cache.ServiceCache) *promptAssembly {
	return &promptAssembly{sections: sections}
}

// promptAssemblyInput 是稳定前缀的缓存键来源。
type promptAssemblyInput struct {
	AgentKey     string
	Model        string
	SystemPrompt string
	ToolIDs      []string
	UserInput    string
}

// Build 返回本次运行使用的 Prompt 段，并回报稳定前缀是否命中缓存。
func (p *promptAssembly) Build(ctx context.Context, in promptAssemblyInput, toolGuidelines []string) ([]enginepromptctx.PromptSection, bool) {
	stable := []enginepromptctx.PromptSection{
		enginepromptctx.Section("core_instruction", enginepromptctx.DefaultCoreAgentInstruction, 10),
		enginepromptctx.Section("agent_definition", strings.TrimSpace(in.SystemPrompt), 20),
		enginepromptctx.Section("tool_guidelines", strings.Join(toolGuidelines, "\n"), 30),
	}
	if p == nil || p.sections == nil {
		return append(stable, volatileSection(in.UserInput)), false
	}

	key := promptStableCacheKey(in)
	if raw, ok := p.sections.Get(ctx, key); ok {
		var cached []enginepromptctx.PromptSection
		if err := json.Unmarshal([]byte(raw), &cached); err == nil && len(cached) > 0 {
			return append(cached, volatileSection(in.UserInput)), true
		}
	}
	if raw, err := json.Marshal(stable); err == nil {
		p.sections.Set(ctx, key, string(raw), promptStableSectionTTL)
	}
	return append(stable, volatileSection(in.UserInput)), false
}

// RedisEnabled 报告缓存后端是否配置了 Redis（用于把降级状态写进运行轨迹）。
func (p *promptAssembly) RedisEnabled() bool {
	return p != nil && p.sections != nil && p.sections.IsRedisEnabled()
}

// volatileSection 用户输入永远作为易变段排在最后，保证稳定前缀是真正的前缀。
func volatileSection(userInput string) enginepromptctx.PromptSection {
	section := enginepromptctx.Section("user_input", strings.TrimSpace(userInput), 100)
	section.Volatile = true
	return section
}

// promptStableCacheKey 只对稳定内容求哈希：用户输入不参与，否则缓存永不命中。
func promptStableCacheKey(in promptAssemblyInput) string {
	h := sha256.New()
	for _, part := range []string{in.AgentKey, in.Model, strings.TrimSpace(in.SystemPrompt), strings.Join(in.ToolIDs, ",")} {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	return "agent:prompt:stable:" + hex.EncodeToString(h.Sum(nil))
}

// promptAssemblyMetadata 汇总装配结果，供运行轨迹记录与验收核对。
func promptAssemblyMetadata(sections []enginepromptctx.PromptSection, cacheHit bool, redisEnabled bool) map[string]any {
	names := make([]string, 0, len(sections))
	volatile := make([]string, 0, len(sections))
	for _, section := range sections {
		names = append(names, section.Name)
		if section.Volatile {
			volatile = append(volatile, section.Name)
		}
	}
	return map[string]any{
		"status":            "prompt_assembled",
		"sections":          names,
		"volatile_sections": volatile,
		"stable_cache_hit":  cacheHit,
		"redis_enabled":     redisEnabled,
		"prompt_cache":      enginepromptctx.BuildPromptCacheMetadata(sections, ""),
	}
}

// toolGuidelines 把已解析的工具集渲染为稳定前缀里的一段文字说明。
func toolGuidelines(entries []string, describe func(id string) (string, bool)) []string {
	lines := make([]string, 0, len(entries))
	for _, id := range entries {
		if describe != nil {
			if text, ok := describe(id); ok && strings.TrimSpace(text) != "" {
				lines = append(lines, text)
				continue
			}
		}
		lines = append(lines, fmt.Sprintf("- %s", id))
	}
	return lines
}
