package adapters

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"InkLore/internal/platform/scope"

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
	if strings.TrimSpace(input.ExecutionKey) == "" {
		return nil, errors.New("persisted model execution key is required")
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
	var override *llmdomain.EnabledModel
	if selected := strings.TrimSpace(input.Model); selected != "" &&
		selected != policy.ModelName && selected != policy.ModelID {
		model, resolveErr := g.catalog.ResolveEnabledModel(ctx, input.TenantID, selected)
		if resolveErr != nil {
			return nil, resolveErr
		}
		modelID, modelToUse = model.ID, model.Name
		override = &model
	}
	maxTokens := policy.MaxOutputTokens
	if input.MaxOutputTokens > 0 {
		if input.MaxOutputTokens > maxTokens {
			return nil, errors.New("requested model output exceeds frozen policy limit")
		}
		maxTokens = input.MaxOutputTokens
	}
	// The selected override model travels with its own documented ceiling;
	// the frozen policy limit alone must not exceed what that model accepts.
	if override != nil && override.MaxOutputTokens > 0 && int64(maxTokens) > override.MaxOutputTokens {
		maxTokens = int(override.MaxOutputTokens)
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
		ExecutionKey:      "agent/" + input.ExecutionKey,
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
	result := &agentdomain.ModelGenerateOutput{
		InputTokens:  int64(tokenInput(response.Usage)),
		OutputTokens: int64(tokenOutput(response.Usage)),
	}
	if response.Receipt != nil {
		result = agentReceiptOutput(response.Receipt)
	}
	if err != nil {
		return result, err
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
		return result, errors.New("model response contained neither text nor tool calls")
	}
	return result, nil
}

// ListSettledAgentUsage scans settled LLM receipts independently of Run state.
func (g *AgentTaskModelGateway) ListSettledAgentUsage(ctx context.Context, after string, limit int) ([]agentdomain.ModelSettledUsage, error) {
	if g == nil || g.catalog == nil {
		return nil, errors.New("agent usage sweep is not configured")
	}
	rows, err := g.catalog.ListSettledAgentOperations(ctx, after, limit)
	if err != nil {
		return nil, err
	}
	out := make([]agentdomain.ModelSettledUsage, 0, len(rows))
	for _, row := range rows {
		key, ok := strings.CutPrefix(row.OperationKey, "agent/")
		if !ok {
			return nil, errors.New("unexpected agent usage operation")
		}
		node, index, ok := strings.Cut(key, "/")
		if !ok || !scope.ValidID(node) {
			return nil, errors.New("invalid agent usage node")
		}
		iteration, err := strconv.Atoi(index)
		if err != nil || iteration < 0 || iteration > 15 {
			return nil, errors.New("invalid agent usage iteration")
		}
		usage := row.Usage
		out = append(out, agentdomain.ModelSettledUsage{
			Cursor: row.Cursor, TenantID: row.TenantID, NodeID: node,
			Output: agentReceiptOutput(&usage),
		})
	}
	return out, nil
}

// RecoverUsage reads the original operation's ledger only. It never invokes a
// model, executes a tool, or turns an interrupted response into author content.
func (g *AgentTaskModelGateway) RecoverUsage(ctx context.Context, tenantID, executionKey string) (*agentdomain.ModelGenerateOutput, error) {
	if g == nil || g.catalog == nil {
		return nil, errors.New("agent usage recovery is not configured")
	}
	receipt, err := g.catalog.FindOperationUsage(ctx, tenantID, "agent/"+executionKey)
	if err != nil || receipt == nil {
		return nil, err
	}
	return agentReceiptOutput(receipt), nil
}

func agentReceiptOutput(receipt *llmdomain.Usage) *agentdomain.ModelGenerateOutput {
	return &agentdomain.ModelGenerateOutput{
		InputTokens: receipt.InputTokens, OutputTokens: receipt.OutputTokens,
		CostMicros: receipt.CostMicros, Estimated: receipt.Estimated,
		UsageReceiptID: &receipt.ID,
	}
}

func (g *AgentTaskModelGateway) ResolvePolicy(ctx context.Context, tenantID, taskKey string) (string, error) {
	if g == nil || g.catalog == nil {
		return "", errors.New("agent policy resolver is not configured")
	}
	return g.catalog.CurrentPolicyVersion(ctx, tenantID, strings.TrimSpace(taskKey))
}

// RecordPolicyVersionReference forwards the agent-owned pin to the LLM
// catalog so deletion refusal never needs to read Agent private tables.
func (g *AgentTaskModelGateway) RecordPolicyVersionReference(ctx context.Context, tenantID, policyVersionID, referenceKind, referenceID string) error {
	if g == nil || g.catalog == nil {
		return errors.New("agent policy resolver is not configured")
	}
	return g.catalog.RecordPolicyVersionReference(ctx, tenantID, policyVersionID, referenceKind, referenceID)
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
