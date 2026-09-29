package adapters

import (
	"context"
	"fmt"
	"os"
	"strings"

	tooldomain "InkLore/internal/agent/tool/domain"
	llmapp "InkLore/internal/llm/application"
	llmdomain "InkLore/internal/llm/domain"
)

// 写作工具端口到模型基座域的适配器（INK-072）。
//
// 默认模型从环境变量 WRITING_TOOL_MODEL 读取（组合根构造时注入），与 SUMMARY_MODEL /
// EMBEDDING_MODEL 的处理一致：模型名不是密钥，但由部署侧声明，不在代码里写死。
// 未声明且工具也没传 model 时**明确报错**，由工具把它变成 Success=false 的观察结果，
// 而不是悄悄换一个模型（ADR-0005：不伪造来源）。
type AgentToolGenerator struct {
	llm          *llmapp.GatewayService
	defaultModel string
	// policies 是可选的任务级模型策略（INK-066）。解析顺序恒为：
	// 工具显式传 model > 任务策略 > WRITING_TOOL_MODEL 环境默认值。
	policies TaskModelResolver
}

func NewAgentToolGenerator(llm *llmapp.GatewayService, policies TaskModelResolver) *AgentToolGenerator {
	return &AgentToolGenerator{
		llm:          llm,
		defaultModel: strings.TrimSpace(os.Getenv("WRITING_TOOL_MODEL")),
		policies:     policies,
	}
}

func (g *AgentToolGenerator) GenerateText(ctx context.Context, request tooldomain.TextRequest) (tooldomain.TextResponse, error) {
	taskKey := strings.TrimSpace(request.TaskKey)
	model := strings.TrimSpace(request.Model)
	// 解析顺序固定为：工具显式传 model > 任务策略 > WRITING_TOOL_MODEL 环境默认值。
	// 温度同理，只是多一层工具自带的建议值垫底（见 TextRequest.DefaultTemperature）。
	var policy *llmdomain.TaskPolicy
	if g.policies != nil && taskKey != "" {
		if resolved, ok := g.policies.ResolvePolicy(ctx, taskKey); ok {
			policy = resolved
			if model == "" {
				model = strings.TrimSpace(policy.ModelKey)
			}
		}
	}
	if model == "" {
		model = g.defaultModel
	}
	// 自动级联回退到网关当前生效模型或环境配置模型，避免未配独立写作模型时阻断运行
	if model == "" && g.llm != nil {
		model = strings.TrimSpace(g.llm.GetRuntimeConfig(ctx).ActiveModel)
	}
	if model == "" {
		model = strings.TrimSpace(os.Getenv("LLM_MODEL"))
	}
	if model == "" {
		model = "deepseek-chat"
	}
	if g.llm == nil {
		return tooldomain.TextResponse{}, fmt.Errorf("模型网关未装配")
	}

	temperature := request.Temperature
	if temperature <= 0 && policy != nil {
		temperature = policy.Temperature
	}
	if temperature <= 0 {
		temperature = request.DefaultTemperature
	}
	if temperature <= 0 {
		temperature = 0.7
	}
	maxTokens := 0
	if policy != nil {
		maxTokens = policy.MaxTokens
	}

	var result llmdomain.GenerateResult
	err := callWithPolicy(ctx, policy, func(callCtx context.Context) error {
		var callErr error
		result, callErr = g.llm.Generate(callCtx, llmdomain.GenerateRequest{
			Model:        model,
			SystemPrompt: request.SystemPrompt,
			Messages:     []llmdomain.Message{{Role: "user", Content: request.Prompt}},
			Temperature:  temperature,
			MaxTokens:    maxTokens,
			RequestID:    request.RequestID,
			TaskKey:      taskKey,
		})
		return callErr
	})
	if err != nil {
		return tooldomain.TextResponse{}, err
	}
	return tooldomain.TextResponse{Content: result.Content, Model: result.Model}, nil
}
