package adapters

import (
	"context"
	"errors"
	"strings"

	agentdomain "InkLore/internal/agent/domain"
	llmapp "InkLore/internal/llm/application"
	llmdomain "InkLore/internal/llm/domain"
)

// AgentTaskModelGateway is the composition-root adapter between agent runs and
// the LLM gateway. It never manufactures a result: missing providers and
// upstream errors are returned to the run scheduler unchanged.
type AgentTaskModelGateway struct {
	llm     *llmapp.GatewayService
	catalog *llmapp.Catalog
}

func NewAgentTaskModelGateway(llm *llmapp.GatewayService, catalog *llmapp.Catalog) *AgentTaskModelGateway {
	return &AgentTaskModelGateway{llm: llm, catalog: catalog}
}

func (g *AgentTaskModelGateway) Generate(ctx context.Context, input agentdomain.ModelGenerateInput) (*agentdomain.ModelGenerateOutput, error) {
	if g == nil || g.llm == nil || g.catalog == nil {
		return nil, errors.New("agent model gateway is not configured")
	}
	if strings.TrimSpace(input.PolicyVersionID) == "" {
		return nil, errors.New("frozen model policy version is required")
	}
	policy, err := g.catalog.ReadFrozenPolicy(ctx, input.TenantID, input.PolicyVersionID)
	if err != nil {
		return nil, err
	}
	if policy.TaskKey != input.TaskKey {
		return nil, errors.New("frozen model policy does not match the run task")
	}

	modelID, modelToUse := policy.ModelID, policy.ModelName
	// A saved novel guide repeats the frozen model name. Keep its exact model
	// ID instead of re-resolving a provider-ambiguous upstream name.
	if selected := strings.TrimSpace(input.Model); selected != "" &&
		selected != policy.ModelName && selected != policy.ModelID {
		model, resolveErr := g.catalog.ResolveEnabledModel(ctx, input.TenantID, selected)
		if resolveErr != nil {
			return nil, resolveErr
		}
		modelID, modelToUse = model.ID, model.Name
	}
	maxTokens := policy.MaxOutputTokens
	if input.MaxOutputTokens > 0 {
		if input.MaxOutputTokens > maxTokens {
			return nil, errors.New("requested model output exceeds frozen policy limit")
		}
		maxTokens = input.MaxOutputTokens
	}
	request := llmdomain.TextGenerationRequest{
		TenantID:          input.TenantID,
		PolicyVersionID:   input.PolicyVersionID,
		ModelID:           modelID,
		Model:             modelToUse,
		SystemPrompt:      input.SystemInstruction,
		MaxTokens:         maxTokens,
		Temperature:       policy.Temperature,
		TaskKey:           policy.TaskKey,
		RequestID:         input.RunID,
		ToolChoice:        input.ToolChoice,
		ParallelToolCalls: input.ParallelToolCalls,
	}
	if strings.TrimSpace(input.ContextText) != "" {
		request.Messages = append(request.Messages, llmdomain.ChatMessage{Role: llmdomain.ChatMessageRoleUser, Content: input.ContextText})
	}
	if strings.TrimSpace(input.UserInstruction) != "" {
		request.Messages = append(request.Messages, llmdomain.ChatMessage{Role: llmdomain.ChatMessageRoleUser, Content: input.UserInstruction})
	}
	for _, tool := range input.Tools {
		request.Tools = append(request.Tools, llmdomain.Tool{
			Type: "function",
			Function: llmdomain.ToolFunction{
				Name:        tool.ToolKey,
				Description: tool.Title,
				Parameters:  tool.InputSchema,
			},
		})
	}

	response, err := g.llm.GenerateChat(ctx, request)
	if err != nil {
		return nil, err
	}

	result := &agentdomain.ModelGenerateOutput{
		InputTokens:  int64(tokenInput(response.Usage)),
		OutputTokens: int64(tokenOutput(response.Usage)),
	}
	if response.Content != "" {
		text := response.Content
		result.FinalText = &text
	}
	for _, call := range response.ToolCalls {
		toolKey, toolVersion := resolveAgentTool(call.Function.Name, input.Tools)
		result.ToolCalls = append(result.ToolCalls, agentdomain.ToolCall{
			InvocationID: call.ID,
			ToolKey:      toolKey,
			ToolVersion:  toolVersion,
			Arguments:    []byte(call.Function.Arguments),
		})
	}
	if result.FinalText == nil && len(result.ToolCalls) == 0 {
		return nil, errors.New("model response contained neither text nor tool calls")
	}
	return result, nil
}

func (g *AgentTaskModelGateway) ResolvePolicy(ctx context.Context, tenantID, taskKey string) (string, error) {
	if g == nil || g.catalog == nil {
		return "", errors.New("agent policy resolver is not configured")
	}
	return g.catalog.CurrentPolicyVersion(ctx, tenantID, strings.TrimSpace(taskKey))
}

func (g *AgentTaskModelGateway) ResolvePolicySnapshot(ctx context.Context, tenantID, taskKey string) (agentdomain.NovelGuidePolicySnapshot, error) {
	if g == nil || g.catalog == nil {
		return agentdomain.NovelGuidePolicySnapshot{}, errors.New("agent policy resolver is not configured")
	}
	policy, err := g.catalog.CurrentPolicySnapshot(ctx, tenantID, taskKey)
	if err != nil {
		return agentdomain.NovelGuidePolicySnapshot{}, err
	}
	return agentdomain.NovelGuidePolicySnapshot{
		VersionID: policy.VersionID, ModelName: policy.ModelName,
		MaxOutputTokens: policy.MaxOutputTokens, Temperature: policy.Temperature,
	}, nil
}

func resolveAgentTool(name string, tools []agentdomain.ToolView) (string, string) {
	for _, tool := range tools {
		if tool.ToolKey == name || tool.Title == name {
			return tool.ToolKey, tool.CurrentVersionID
		}
	}
	return name, ""
}

func tokenInput(usage *llmdomain.TokenUsage) int {
	if usage == nil {
		return 0
	}
	if usage.InputTokens > 0 {
		return usage.InputTokens
	}
	return usage.PromptTokens
}

func tokenOutput(usage *llmdomain.TokenUsage) int {
	if usage == nil {
		return 0
	}
	if usage.OutputTokens > 0 {
		return usage.OutputTokens
	}
	return usage.CompletionTokens
}
