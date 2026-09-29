package adapters

import (
	"context"
	"errors"
	"strings"

	agentdomain "InkLore/internal/agent/domain"
	llmapp "InkLore/internal/llm/application"
	llmdomain "InkLore/internal/llm/domain"
	"InkLore/internal/platform/uow"
)

// AgentTaskModelGateway is the composition-root adapter between agent runs and
// the LLM gateway. It never manufactures a result: missing providers and
// upstream errors are returned to the run scheduler unchanged.
type AgentTaskModelGateway struct {
	llm     *llmapp.GatewayService
	manager *uow.Manager
}

func NewAgentTaskModelGateway(llm *llmapp.GatewayService, manager *uow.Manager) *AgentTaskModelGateway {
	return &AgentTaskModelGateway{llm: llm, manager: manager}
}

func (g *AgentTaskModelGateway) Generate(ctx context.Context, input agentdomain.ModelGenerateInput) (*agentdomain.ModelGenerateOutput, error) {
	if g == nil || g.llm == nil {
		return nil, errors.New("agent model gateway is not configured")
	}

	modelToUse := ""
	if strings.TrimSpace(input.Model) != "" {
		if g.manager == nil {
			return nil, errors.New("selected model cannot be authorized")
		}
		db, err := g.manager.DB(ctx)
		if err != nil {
			return nil, err
		}
		var upstreamName string
		err = db.Raw(`
			SELECT upstream_name
			FROM llm_models
			WHERE tenant_id = ? AND (id::text = ? OR upstream_name = ?) AND enabled = TRUE AND deleted_at IS NULL
			LIMIT 1`, input.TenantID, strings.TrimSpace(input.Model), strings.TrimSpace(input.Model)).Scan(&upstreamName).Error
		if err != nil {
			return nil, err
		}
		if upstreamName == "" {
			return nil, errors.New("selected model is not enabled for this tenant")
		}
		modelToUse = upstreamName
	}

	request := llmdomain.TextGenerationRequest{
		Model:             modelToUse,
		SystemPrompt:      input.SystemInstruction,
		MaxTokens:         input.MaxOutputTokens,
		Temperature:       input.Temperature,
		TaskKey:           input.TaskKey,
		RequestID:         input.RunID,
		ToolChoice:        input.ToolChoice,
		ParallelToolCalls: input.ParallelToolCalls,
	}
	if request.TaskKey == "" {
		request.TaskKey = input.NodeKey
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
	if g == nil || g.manager == nil {
		return "", errors.New("agent policy resolver is not configured")
	}
	db, err := g.manager.DB(ctx)
	if err != nil {
		return "", err
	}
	var versionID string
	err = db.Raw(`
		SELECT current_version_id::text
		FROM llm_task_policies
		WHERE tenant_id = ? AND task_key = ? AND enabled = TRUE
		  AND deleted_at IS NULL AND current_version_id IS NOT NULL`, tenantID, strings.TrimSpace(taskKey)).Scan(&versionID).Error
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(versionID), nil
}

func (g *AgentTaskModelGateway) ResolvePolicySnapshot(ctx context.Context, tenantID, taskKey string) (agentdomain.NovelGuidePolicySnapshot, error) {
	var out agentdomain.NovelGuidePolicySnapshot
	if g == nil || g.manager == nil {
		return out, errors.New("agent policy resolver is not configured")
	}
	db, err := g.manager.DB(ctx)
	if err != nil {
		return out, err
	}
	var row struct {
		VersionID      string
		ModelName      string
		MaxOutputTokens int
		Temperature    float64
	}
	result := db.Raw(`SELECT v.id::text AS version_id,m.upstream_name AS model_name,v.max_output_tokens,v.temperature
		FROM llm_task_policies p
		JOIN llm_task_policy_versions v ON v.tenant_id=p.tenant_id AND v.id=p.current_version_id
		JOIN llm_models m ON m.tenant_id=v.tenant_id AND m.id=v.primary_model_id
		JOIN llm_providers pr ON pr.tenant_id=m.tenant_id AND pr.id=m.provider_id
		WHERE p.tenant_id=? AND p.task_key=? AND p.enabled AND p.deleted_at IS NULL
		AND m.enabled AND m.deleted_at IS NULL AND pr.enabled AND pr.deleted_at IS NULL`, tenantID, taskKey).Scan(&row)
	if result.Error != nil {
		return out, result.Error
	}
	if result.RowsAffected != 1 || row.VersionID == "" || row.ModelName == "" {
		return out, errors.New("novel creation task policy is unavailable")
	}
	return agentdomain.NovelGuidePolicySnapshot{VersionID: row.VersionID, ModelName: row.ModelName, MaxOutputTokens: row.MaxOutputTokens, Temperature: row.Temperature}, nil
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
