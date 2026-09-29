package adapters

import (
	"context"
	"fmt"
	"strings"

	enginedomain "InkLore/internal/agent/domain"
	enginepromptctx "InkLore/internal/agent/runtime/application/promptctx"
	llmdomain "InkLore/internal/llm/domain"
	"InkLore/internal/platform/config"
)

// ResolveCompressionBinding 把配置里的上下文压缩辅助任务解析成可用绑定（INK-050）。
//
// 模型只按 model_key 声明，模型 ID 与供应商到 Provider 注册表里查：
//   - 未启用 → (空绑定, false, nil)，调用方继续使用本地确定性交接文档；
//   - 启用但配置缺 model_key / 注册表不可用 / 模型不存在或已停用 → 返回错误，
//     由组合根记录并降级，绝不让压缩在运行到一半时才失败。
func ResolveCompressionBinding(ctx context.Context, cfg config.AuxiliaryTaskConfig, registry llmdomain.ProviderRegistryRepository) (enginepromptctx.AuxiliaryTaskBinding, bool, error) {
	empty := enginepromptctx.AuxiliaryTaskBinding{}
	if !cfg.Enabled {
		return empty, false, nil
	}
	modelKey := strings.TrimSpace(cfg.ModelKey)
	if modelKey == "" {
		return empty, false, fmt.Errorf("agent_auxiliary.compression.model_key is required when enabled")
	}
	if registry == nil {
		return empty, false, fmt.Errorf("provider registry is unavailable")
	}

	providers, err := registry.ListProviders(ctx)
	if err != nil {
		return empty, false, fmt.Errorf("读取 Provider 列表失败: %w", err)
	}
	providerCodes := make(map[int64]string, len(providers))
	for _, provider := range providers {
		if provider != nil {
			providerCodes[provider.ID] = provider.Code
		}
	}

	models, err := registry.ListAllModels(ctx)
	if err != nil {
		return empty, false, fmt.Errorf("读取模型列表失败: %w", err)
	}
	for _, model := range models {
		if model == nil || !strings.EqualFold(strings.TrimSpace(model.ModelKey), modelKey) {
			continue
		}
		if !model.Enabled {
			return empty, false, fmt.Errorf("辅助模型 %q 已停用", modelKey)
		}
		modelID := model.ID
		return enginepromptctx.AuxiliaryTaskBinding{
			Task: enginedomain.AgentAuxiliaryTask{
				Enabled:      true,
				ModelID:      &modelID,
				SystemPrompt: cfg.SystemPrompt,
				UserPrompt:   cfg.UserPrompt,
				MaxTokens:    cfg.MaxTokens,
				Temperature:  cfg.Temperature,
			},
			Model: enginepromptctx.AuxiliaryModel{
				ID:       modelID,
				Model:    model.ModelKey,
				Provider: providerCodes[model.ProviderID],
			},
		}, true, nil
	}
	return empty, false, fmt.Errorf("辅助模型 %q 不在 Provider 注册表中", modelKey)
}
