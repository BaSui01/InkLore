package infrastructure

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"InkLore/internal/llm/domain"
	platformerr "InkLore/internal/platform/errors"
)

const (
	protocolOpenAI             = "openai"
	protocolDeepSeek           = "deepseek"
	protocolAnthropic          = "anthropic"
	protocolGemini             = "gemini"
	protocolGeminiInteractions = "gemini_interactions"
	protocolGeminiOpenAI       = "gemini_openai"
	protocolOllama             = "ollama"
	protocolResponses          = "openai_responses"
	openAIResponsesEnv         = "responses"
)

func normalizeProtocol(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case protocolOpenAI, protocolDeepSeek, protocolAnthropic, protocolGemini, protocolGeminiInteractions, protocolGeminiOpenAI, protocolOllama, protocolResponses:
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return protocolOpenAI
	}
}

func nativeProtocol(protocol string) bool {
	switch normalizeProtocol(protocol) {
	case protocolAnthropic, protocolGemini, protocolGeminiInteractions, protocolResponses:
		return true
	default:
		return false
	}
}

func configuredProvider() (protocol, key, baseURL string) {
	protocol = normalizeProtocol(osValue("LLM_PROTOCOL"))
	key = strings.TrimSpace(osValue("LLM_API_KEY"))
	baseURL = strings.TrimRight(strings.TrimSpace(osValue("LLM_BASE_URL")), "/")

	if osValue("LLM_PROTOCOL") == "" {
		switch {
		case osValue("ANTHROPIC_API_KEY") != "":
			protocol, key = protocolAnthropic, strings.TrimSpace(osValue("ANTHROPIC_API_KEY"))
		case osValue("GEMINI_API_KEY") != "":
			protocol, key = protocolGemini, strings.TrimSpace(osValue("GEMINI_API_KEY"))
		case osValue("DEEPSEEK_API_KEY") != "":
			protocol, key = protocolDeepSeek, strings.TrimSpace(osValue("DEEPSEEK_API_KEY"))
		case osValue("OPENAI_API_KEY") != "":
			protocol, key = protocolOpenAI, strings.TrimSpace(osValue("OPENAI_API_KEY"))
		case osValue("OLLAMA_BASE_URL") != "":
			protocol, key = protocolOllama, "ollama"
		}
	}

	if osValue("LLM_PROTOCOL") != "" && key == "" {
		switch protocol {
		case protocolAnthropic:
			key = strings.TrimSpace(osValue("ANTHROPIC_API_KEY"))
		case protocolGemini, protocolGeminiInteractions, protocolGeminiOpenAI:
			key = strings.TrimSpace(osValue("GEMINI_API_KEY"))
		case protocolDeepSeek:
			key = strings.TrimSpace(osValue("DEEPSEEK_API_KEY"))
		case protocolOpenAI, protocolResponses:
			key = strings.TrimSpace(osValue("OPENAI_API_KEY"))
		case protocolOllama:
			key = "ollama"
		}
	}

	if baseURL == "" {
		switch protocol {
		case protocolAnthropic:
			baseURL = "https://api.anthropic.com/v1"
		case protocolGemini, protocolGeminiInteractions:
			baseURL = "https://generativelanguage.googleapis.com/v1beta"
		case protocolGeminiOpenAI:
			baseURL = "https://generativelanguage.googleapis.com/v1beta/openai"
		case protocolDeepSeek:
			baseURL = "https://api.deepseek.com"
		case protocolOllama:
			baseURL = strings.TrimRight(strings.TrimSpace(osValue("OLLAMA_BASE_URL")), "/")
			if baseURL != "" && !strings.HasSuffix(baseURL, "/v1") {
				baseURL += "/v1"
			}
		case protocolOpenAI, protocolResponses:
			baseURL = strings.TrimRight(strings.TrimSpace(osValue("OPENAI_BASE_URL")), "/")
		}
	}
	if baseURL == "" && (protocol == protocolOpenAI || protocol == protocolResponses) {
		baseURL = "https://api.openai.com/v1"
	}
	if protocol == protocolOpenAI && strings.EqualFold(strings.TrimSpace(osValue("LLM_OPENAI_API")), openAIResponsesEnv) {
		protocol = protocolResponses
	}
	return protocol, key, baseURL
}

func osValue(key string) string { return os.Getenv(key) }

func defaultModelForProtocol(protocol string, live bool) string {
	if !live {
		return ""
	}
	switch normalizeProtocol(protocol) {
	case protocolDeepSeek:
		return "deepseek-chat"
	case protocolAnthropic:
		return "claude-3-7-sonnet-latest"
	case protocolGemini, protocolGeminiOpenAI:
		return "gemini-2.0-flash"
	case protocolGeminiInteractions:
		return "gemini-2.5-flash"
	case protocolOllama:
		return strings.TrimSpace(osValue("LLM_MODEL"))
	default:
		return "gpt-4o"
	}
}

func nativeRequest(req domain.GenerateRequest, model string) nativeTextRequest {
	messages := make([]domain.ChatMessage, 0, len(req.Messages))
	for _, message := range req.Messages {
		messages = append(messages, domain.ChatMessage{Role: message.Role, Content: message.Content})
	}
	return nativeTextRequest{
		Model:        model,
		SystemPrompt: req.SystemPrompt,
		Messages:     messages,
		Temperature:  req.Temperature,
		MaxTokens:    req.MaxTokens,
	}
}

func nativeToolRequest(req domain.TextGenerationRequest, model string) nativeTextRequest {
	return nativeTextRequest{
		Model:             model,
		SystemPrompt:      req.SystemPrompt,
		Messages:          req.Messages,
		Tools:             req.Tools,
		ToolChoice:        req.ToolChoice,
		ResponseFormat:    req.ResponseFormat,
		ReasoningEffort:   req.ReasoningEffort,
		ThinkingEnabled:   req.ThinkingEnabled,
		Stop:              req.Stop,
		Temperature:       req.Temperature,
		TopP:              req.TopP,
		PresencePenalty:   req.PresencePenalty,
		FrequencyPenalty:  req.FrequencyPenalty,
		Seed:              req.Seed,
		ParallelToolCalls: req.ParallelToolCalls,
		MaxTokens:         req.MaxTokens,
	}
}

func (r *ProviderRouter) generateNativeToolChat(ctx context.Context, target upstreamTarget, req domain.TextGenerationRequest, model string) (response domain.TextGenerationResponse, err error) {
	startedAt := time.Now()
	defer func() {
		r.recordUpstream(target, err)
		input, output := 0, 0
		if err == nil && response.Usage != nil {
			input, output = response.Usage.PromptTokens, response.Usage.CompletionTokens
		}
		r.recordUsage(ctx, req.RequestID, req.TaskKey, model, target, startedAt, input, output, err)
	}()
	return r.nativeChat(ctx, target, nativeToolRequest(req, model))
}

func (r *ProviderRouter) generateNative(ctx context.Context, req domain.GenerateRequest) (result domain.GenerateResult, err error) {
	target, err := r.resolveUpstream(ctx)
	if err != nil {
		return domain.GenerateResult{}, err
	}
	model := req.Model
	if model == "" || strings.HasPrefix(model, "novel-") {
		model = r.config.ActiveModel
	}
	if model == "" {
		model = defaultModelForProtocol(r.protocol, true)
	}
	startedAt := time.Now()
	defer func() {
		r.recordUpstream(target, err)
		input, output := 0, 0
		if err == nil {
			input, output = result.Usage.InputTokens, result.Usage.OutputTokens
		}
		r.recordUsage(ctx, req.RequestID, req.TaskKey, model, target, startedAt, input, output, err)
	}()
	response, err := r.nativeChat(ctx, target, nativeRequest(req, model))
	if err != nil {
		return domain.GenerateResult{}, err
	}
	usage := domain.GenerationUsage{}
	if response.Usage != nil {
		usage = domain.GenerationUsage{InputTokens: response.Usage.PromptTokens, OutputTokens: response.Usage.CompletionTokens, TotalTokens: response.Usage.TotalTokens}
	}
	return domain.GenerateResult{Content: response.Content, Model: response.Model, Usage: usage}, nil
}

func (r *ProviderRouter) generateNativeStream(ctx context.Context, req domain.GenerateRequest, onEvent domain.StreamEventHandler) (err error) {
	target, err := r.resolveUpstream(ctx)
	if err != nil {
		return err
	}
	model := req.Model
	if model == "" || strings.HasPrefix(model, "novel-") {
		model = r.config.ActiveModel
	}
	if model == "" {
		model = defaultModelForProtocol(r.protocol, true)
	}
	var usage domain.GenerationUsage
	startedAt := time.Now()
	defer func() {
		r.recordUpstream(target, err)
		r.recordUsage(ctx, req.RequestID, req.TaskKey, model, target, startedAt, usage.InputTokens, usage.OutputTokens, err)
	}()
	return r.nativeStream(ctx, target, nativeRequest(req, model), func(event domain.StreamEvent) error {
		if event.Usage != nil {
			usage = *event.Usage
		}
		return onEvent(event)
	})
}

type nativeTextRequest struct {
	Model             string
	SystemPrompt      string
	Messages          []domain.ChatMessage
	Tools             []domain.Tool
	ToolChoice        any
	ResponseFormat    *domain.ResponseFormat
	ReasoningEffort   string
	ThinkingEnabled   *bool
	Stop              []string
	Temperature       float64
	TopP              float64
	PresencePenalty   float64
	FrequencyPenalty  float64
	Seed              *int
	ParallelToolCalls *bool
	MaxTokens         int
}

func (r *ProviderRouter) nativeChat(ctx context.Context, target upstreamTarget, req nativeTextRequest) (domain.TextGenerationResponse, error) {
	switch normalizeProtocol(r.protocol) {
	case protocolAnthropic:
		return r.anthropicChat(ctx, target, req)
	case protocolGemini:
		return r.geminiChat(ctx, target, req)
	case protocolGeminiInteractions:
		return r.geminiInteractionsChat(ctx, target, req)
	case protocolResponses:
		return r.responsesChat(ctx, target, req)
	default:
		return domain.TextGenerationResponse{}, fmt.Errorf("unsupported native protocol %q", r.protocol)
	}
}

func (r *ProviderRouter) nativeStream(ctx context.Context, target upstreamTarget, req nativeTextRequest, onEvent domain.StreamEventHandler) error {
	switch normalizeProtocol(r.protocol) {
	case protocolAnthropic:
		return r.anthropicStream(ctx, target, req, onEvent)
	case protocolGemini:
		return r.geminiStream(ctx, target, req, onEvent)
	case protocolGeminiInteractions:
		return r.geminiInteractionsStream(ctx, target, req, onEvent)
	case protocolResponses:
		return r.responsesStream(ctx, target, req, onEvent)
	default:
		return fmt.Errorf("unsupported native protocol %q", r.protocol)
	}
}

// --- Anthropic Native Protocol ---

type anthropicMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type anthropicTool struct {
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	InputSchema  any    `json:"input_schema"`
	CacheControl any    `json:"cache_control,omitempty"`
}

type anthropicThinking struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens,omitempty"`
}

type anthropicRequest struct {
	Model         string             `json:"model"`
	System        any                `json:"system,omitempty"`
	Messages      []anthropicMessage `json:"messages"`
	Tools         []anthropicTool    `json:"tools,omitempty"`
	ToolChoice    any                `json:"tool_choice,omitempty"`
	Thinking      *anthropicThinking `json:"thinking,omitempty"`
	MaxTokens     int                `json:"max_tokens"`
	Temperature   *float64           `json:"temperature,omitempty"`
	TopP          *float64           `json:"top_p,omitempty"`
	StopSequences []string           `json:"stop_sequences,omitempty"`
	Stream        bool               `json:"stream,omitempty"`
}

type anthropicResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Stop    string `json:"stop_reason"`
	Content []struct {
		Type     string          `json:"type"`
		Text     string          `json:"text"`
		Thinking string          `json:"thinking,omitempty"`
		ID       string          `json:"id"`
		Name     string          `json:"name"`
		Input    json.RawMessage `json:"input"`
	} `json:"content"`
	Usage struct {
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
	} `json:"usage"`
}

func isAnthropicThinkingEnabled(req nativeTextRequest) bool {
	if req.ThinkingEnabled != nil {
		return *req.ThinkingEnabled
	}
	m := strings.ToLower(req.Model)
	return strings.Contains(m, "claude-3-7") || strings.Contains(m, "claude-4")
}

func calculateAnthropicBudget(effort string, maxTokens int) int {
	budget := 4096
	switch strings.ToLower(strings.TrimSpace(effort)) {
	case "minimal":
		budget = 1024
	case "low":
		budget = 2048
	case "medium":
		budget = 5000
	case "high":
		budget = 10000
	case "xhigh":
		budget = 24000
	case "max", "ultra":
		budget = 48000
	}
	if budget >= maxTokens {
		budget = maxTokens - 1024
	}
	if budget < 1024 {
		budget = 1024
	}
	return budget
}

func toAnthropicMessages(messages []domain.ChatMessage) []anthropicMessage {
	result := make([]anthropicMessage, 0, len(messages))
	for _, message := range messages {
		if message.Role == domain.ChatMessageRoleSystem {
			continue
		}
		if message.Role == domain.ChatMessageRoleTool {
			toolResultBlock := map[string]any{
				"type":        "tool_result",
				"tool_use_id": message.ToolCallID,
				"content":     message.Content,
			}
			if len(result) > 0 && result[len(result)-1].Role == "user" {
				if existingBlocks, ok := result[len(result)-1].Content.([]map[string]any); ok {
					result[len(result)-1].Content = append(existingBlocks, toolResultBlock)
					continue
				}
			}
			result = append(result, anthropicMessage{
				Role:    "user",
				Content: []map[string]any{toolResultBlock},
			})
			continue
		}

		if len(message.ToolCalls) == 0 {
			result = append(result, anthropicMessage{Role: message.Role, Content: message.Content})
			continue
		}

		blocks := make([]map[string]any, 0, len(message.ToolCalls)+1)
		if strings.TrimSpace(message.Content) != "" {
			blocks = append(blocks, map[string]any{"type": "text", "text": message.Content})
		}
		for _, call := range message.ToolCalls {
			var input any
			if json.Unmarshal([]byte(call.Function.Arguments), &input) != nil {
				input = map[string]any{}
			}
			blocks = append(blocks, map[string]any{
				"type":  "tool_use",
				"id":    call.ID,
				"name":  call.Function.Name,
				"input": input,
			})
		}
		result = append(result, anthropicMessage{Role: "assistant", Content: blocks})
	}
	return result
}

func toAnthropicTools(tools []domain.Tool) []anthropicTool {
	if len(tools) == 0 {
		return nil
	}
	result := make([]anthropicTool, 0, len(tools))
	for i, tool := range tools {
		t := anthropicTool{Name: tool.Function.Name, Description: tool.Function.Description, InputSchema: tool.Function.Parameters}
		if i == len(tools)-1 {
			t.CacheControl = map[string]string{"type": "ephemeral"}
		}
		result = append(result, t)
	}
	return result
}

func toAnthropicToolChoice(choice any) any {
	if choice == nil {
		return nil
	}
	if s, ok := choice.(string); ok {
		switch strings.ToLower(strings.TrimSpace(s)) {
		case "auto":
			return map[string]string{"type": "auto"}
		case "required", "any":
			return map[string]string{"type": "any"}
		case "none":
			return map[string]string{"type": "none"}
		default:
			return nil
		}
	}
	return choice
}

func (r *ProviderRouter) anthropicChat(ctx context.Context, target upstreamTarget, req nativeTextRequest) (domain.TextGenerationResponse, error) {
	maxTokens := req.MaxTokens
	if maxTokens < 1 {
		maxTokens = 8192
	}

	var thinking *anthropicThinking
	var tempPtr *float64
	var topPPtr *float64

	if isAnthropicThinkingEnabled(req) {
		if maxTokens < 4096 {
			maxTokens = 4096
		}
		budget := calculateAnthropicBudget(req.ReasoningEffort, maxTokens)
		thinking = &anthropicThinking{Type: "enabled", BudgetTokens: budget}
		tempPtr = nil
		topPPtr = nil
	} else {
		if req.Temperature > 0 {
			t := req.Temperature
			tempPtr = &t
		}
		if req.TopP > 0 {
			tp := req.TopP
			topPPtr = &tp
		}
	}

	var systemPrompt any
	if strings.TrimSpace(req.SystemPrompt) != "" {
		systemPrompt = []map[string]any{{
			"type":          "text",
			"text":          req.SystemPrompt,
			"cache_control": map[string]string{"type": "ephemeral"},
		}}
	}

	body, err := json.Marshal(anthropicRequest{
		Model:         req.Model,
		System:        systemPrompt,
		Messages:      toAnthropicMessages(req.Messages),
		Tools:         toAnthropicTools(req.Tools),
		ToolChoice:    toAnthropicToolChoice(req.ToolChoice),
		Thinking:      thinking,
		MaxTokens:     maxTokens,
		Temperature:   tempPtr,
		TopP:          topPPtr,
		StopSequences: req.Stop,
		Stream:        false,
	})
	if err != nil {
		return domain.TextGenerationResponse{}, err
	}

	var decoded anthropicResponse
	if _, err = r.doJSON(ctx, target, "/messages", body, func(request *http.Request) {
		request.Header.Set("x-api-key", target.apiKey)
		request.Header.Set("anthropic-version", "2023-06-01")
	}, &decoded); err != nil {
		return domain.TextGenerationResponse{}, err
	}

	cached := decoded.Usage.CacheReadInputTokens
	result := domain.TextGenerationResponse{
		Model:        decoded.Model,
		FinishReason: decoded.Stop,
		Usage: &domain.TokenUsage{
			PromptTokens:          decoded.Usage.InputTokens,
			CompletionTokens:      decoded.Usage.OutputTokens,
			TotalTokens:           decoded.Usage.InputTokens + decoded.Usage.OutputTokens,
			CachedTokens:          cached,
			PromptCacheHitTokens:  cached,
			PromptCacheMissTokens: decoded.Usage.InputTokens - cached,
			InputTokens:           decoded.Usage.InputTokens,
			OutputTokens:          decoded.Usage.OutputTokens,
		},
	}
	for _, block := range decoded.Content {
		if block.Type == "thinking" {
			result.Thinking += block.Thinking
		}
		if block.Type == "text" {
			result.Content += block.Text
		}
		if block.Type == "tool_use" {
			result.ToolCalls = append(result.ToolCalls, domain.ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: domain.ToolCallFunction{
					Name:      block.Name,
					Arguments: string(block.Input),
				},
			})
		}
	}
	return result, nil
}

func (r *ProviderRouter) anthropicStream(ctx context.Context, target upstreamTarget, req nativeTextRequest, onEvent domain.StreamEventHandler) error {
	maxTokens := maxInt(req.MaxTokens, 8192)
	var thinking *anthropicThinking
	var tempPtr *float64
	var topPPtr *float64

	if isAnthropicThinkingEnabled(req) {
		if maxTokens < 4096 {
			maxTokens = 4096
		}
		budget := calculateAnthropicBudget(req.ReasoningEffort, maxTokens)
		thinking = &anthropicThinking{Type: "enabled", BudgetTokens: budget}
		tempPtr = nil
		topPPtr = nil
	} else {
		if req.Temperature > 0 {
			t := req.Temperature
			tempPtr = &t
		}
		if req.TopP > 0 {
			tp := req.TopP
			topPPtr = &tp
		}
	}

	var systemPrompt any
	if strings.TrimSpace(req.SystemPrompt) != "" {
		systemPrompt = []map[string]any{{
			"type":          "text",
			"text":          req.SystemPrompt,
			"cache_control": map[string]string{"type": "ephemeral"},
		}}
	}

	body, err := json.Marshal(anthropicRequest{
		Model:         req.Model,
		System:        systemPrompt,
		Messages:      toAnthropicMessages(req.Messages),
		Tools:         toAnthropicTools(req.Tools),
		ToolChoice:    toAnthropicToolChoice(req.ToolChoice),
		Thinking:      thinking,
		MaxTokens:     maxTokens,
		Temperature:   tempPtr,
		TopP:          topPPtr,
		StopSequences: req.Stop,
		Stream:        true,
	})
	if err != nil {
		return err
	}

	usage := domain.GenerationUsage{}
	doneSent := false
	toolBuilders := make(map[int]*toolCallBuilder)

	err = r.nativeSSE(ctx, target, "/messages", body, func(request *http.Request) {
		request.Header.Set("anthropic-version", "2023-06-01")
	}, func(event string, data []byte) error {
		if event == "error" {
			var errPayload struct {
				Error struct {
					Type    string `json:"type"`
					Message string `json:"message"`
				} `json:"error"`
			}
			_ = json.Unmarshal(data, &errPayload)
			msg := errPayload.Error.Message
			if msg == "" {
				msg = string(data)
			}
			return platformerr.New(http.StatusInternalServerError, "upstream_stream_error", msg)
		}

		var payload struct {
			Index        int `json:"index"`
			ContentBlock struct {
				Type string `json:"type"`
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"content_block"`
			Message struct {
				Usage struct {
					InputTokens int `json:"input_tokens"`
				} `json:"usage"`
			} `json:"message"`
			Delta struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				Thinking    string `json:"thinking"`
				PartialJSON string `json:"partial_json"`
			} `json:"delta"`
			Usage struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal(data, &payload) != nil {
			return nil
		}

		if payload.Message.Usage.InputTokens > 0 {
			usage.InputTokens = payload.Message.Usage.InputTokens
		}
		if payload.Usage.InputTokens > 0 {
			usage.InputTokens = payload.Usage.InputTokens
		}
		if payload.Usage.OutputTokens > 0 {
			usage.OutputTokens = payload.Usage.OutputTokens
		}

		if event == "content_block_start" && payload.ContentBlock.Type == "tool_use" {
			toolBuilders[payload.Index] = &toolCallBuilder{
				id:   payload.ContentBlock.ID,
				name: payload.ContentBlock.Name,
			}
		}
		if event == "content_block_delta" {
			if payload.Delta.Type == "input_json_delta" {
				if b, ok := toolBuilders[payload.Index]; ok {
					b.arguments.WriteString(payload.Delta.PartialJSON)
				}
			}
			if payload.Delta.Text != "" {
				if err := onEvent(domain.StreamEvent{Type: domain.StreamEventMessage, Delta: payload.Delta.Text}); err != nil {
					return err
				}
			}
			if payload.Delta.Thinking != "" {
				if err := onEvent(domain.StreamEvent{Type: domain.StreamEventThink, Delta: payload.Delta.Thinking}); err != nil {
					return err
				}
			}
		}
		if event == "message_stop" {
			for _, b := range toolBuilders {
				_ = onEvent(domain.StreamEvent{
					Type:       domain.StreamEventToolCall,
					ToolCallID: b.id,
					ToolName:   b.name,
					ToolArgs:   b.arguments.String(),
				})
			}
			doneSent = true
			usage.TotalTokens = usage.InputTokens + usage.OutputTokens
			return onEvent(domain.StreamEvent{Type: domain.StreamEventDone, Usage: &usage})
		}
		return nil
	})
	if err == nil && !doneSent {
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens
		err = onEvent(domain.StreamEvent{Type: domain.StreamEventDone, Usage: &usage})
	}
	return err
}

// --- OpenAI Responses API Protocol ---

type responsesInput struct {
	Role      string `json:"role,omitempty"`
	Content   any    `json:"content,omitempty"`
	Type      string `json:"type,omitempty"`
	CallID    string `json:"call_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
	Output    string `json:"output,omitempty"`
}

type responsesTool struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

type responsesReasoning struct {
	Effort  string `json:"effort,omitempty"`
	Summary string `json:"summary,omitempty"`
}

type responsesRequest struct {
	Model             string              `json:"model"`
	Instructions      string              `json:"instructions,omitempty"`
	Input             []responsesInput    `json:"input"`
	Tools             []responsesTool     `json:"tools,omitempty"`
	ToolChoice        any                 `json:"tool_choice,omitempty"`
	ParallelToolCalls bool                `json:"parallel_tool_calls"`
	Reasoning         *responsesReasoning `json:"reasoning,omitempty"`
	Temperature       float64             `json:"temperature,omitempty"`
	TopP              float64             `json:"top_p,omitempty"`
	PresencePenalty   float64             `json:"presence_penalty,omitempty"`
	FrequencyPenalty  float64             `json:"frequency_penalty,omitempty"`
	Seed              *int                `json:"seed,omitempty"`
	Stop              []string            `json:"stop,omitempty"`
	MaxOutputTokens   int                 `json:"max_output_tokens,omitempty"`
	Stream            bool                `json:"stream,omitempty"`
}

type responsesResponse struct {
	ID     string `json:"id"`
	Model  string `json:"model"`
	Output []struct {
		Type      string `json:"type"`
		Name      string `json:"name"`
		CallID    string `json:"call_id"`
		Arguments string `json:"arguments"`
		Content   []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
	Usage struct {
		InputTokens       int `json:"input_tokens"`
		OutputTokens      int `json:"output_tokens"`
		TotalTokens       int `json:"total_tokens"`
		InputTokenDetails struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"input_token_details,omitempty"`
		OutputTokenDetails struct {
			ReasoningTokens int `json:"reasoning_tokens"`
		} `json:"output_token_details,omitempty"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

func toResponsesInput(req nativeTextRequest) []responsesInput {
	result := make([]responsesInput, 0, len(req.Messages))
	for _, message := range req.Messages {
		if message.Role == domain.ChatMessageRoleTool {
			result = append(result, responsesInput{Type: "function_call_output", CallID: message.ToolCallID, Output: message.Content})
			continue
		}
		result = append(result, responsesInput{Role: message.Role, Content: []map[string]string{{"type": "input_text", "text": message.Content}}})
		for _, call := range message.ToolCalls {
			result = append(result, responsesInput{Type: "function_call", CallID: call.ID, Name: call.Function.Name, Arguments: call.Function.Arguments})
		}
	}
	return result
}

func (r *ProviderRouter) responsesChat(ctx context.Context, target upstreamTarget, req nativeTextRequest) (domain.TextGenerationResponse, error) {
	maxTokens := req.MaxTokens
	if maxTokens < 1 {
		maxTokens = 8192
	}
	tools := make([]responsesTool, 0, len(req.Tools))
	for _, tool := range req.Tools {
		tools = append(tools, responsesTool{Type: "function", Name: tool.Function.Name, Description: tool.Function.Description, Parameters: tool.Function.Parameters})
	}
	var reasoning *responsesReasoning
	if req.ReasoningEffort != "" {
		reasoning = &responsesReasoning{Effort: req.ReasoningEffort, Summary: "auto"}
	}

	parallelTools := true
	if req.ParallelToolCalls != nil {
		parallelTools = *req.ParallelToolCalls
	}
	body, err := json.Marshal(responsesRequest{
		Model:             req.Model,
		Instructions:      req.SystemPrompt,
		Input:             toResponsesInput(req),
		Tools:             tools,
		ToolChoice:        req.ToolChoice,
		ParallelToolCalls: parallelTools,
		Reasoning:         reasoning,
		Temperature:       req.Temperature,
		TopP:              req.TopP,
		PresencePenalty:   req.PresencePenalty,
		FrequencyPenalty:  req.FrequencyPenalty,
		Seed:              req.Seed,
		Stop:              req.Stop,
		MaxOutputTokens:   maxTokens,
	})
	if err != nil {
		return domain.TextGenerationResponse{}, err
	}
	var decoded responsesResponse
	if _, err = r.doJSON(ctx, target, "/responses", body, nil, &decoded); err != nil {
		return domain.TextGenerationResponse{}, err
	}
	if decoded.Error != nil && decoded.Error.Message != "" {
		return domain.TextGenerationResponse{}, platformerr.New(http.StatusInternalServerError, "upstream_error", decoded.Error.Message)
	}

	cached := decoded.Usage.InputTokenDetails.CachedTokens
	reasoningTokens := decoded.Usage.OutputTokenDetails.ReasoningTokens

	result := domain.TextGenerationResponse{
		Model: decoded.Model,
		Usage: &domain.TokenUsage{
			PromptTokens:          decoded.Usage.InputTokens,
			CompletionTokens:      decoded.Usage.OutputTokens,
			TotalTokens:           decoded.Usage.TotalTokens,
			ReasoningTokens:       reasoningTokens,
			CachedTokens:          cached,
			PromptCacheHitTokens:  cached,
			PromptCacheMissTokens: decoded.Usage.InputTokens - cached,
			InputTokens:           decoded.Usage.InputTokens,
			OutputTokens:          decoded.Usage.OutputTokens,
		},
	}
	if result.Model == "" {
		result.Model = req.Model
	}
	for _, item := range decoded.Output {
		if item.Type == "message" {
			for _, part := range item.Content {
				if part.Type == "output_text" {
					result.Content += part.Text
				}
			}
		}
		if item.Type == "function_call" {
			result.ToolCalls = append(result.ToolCalls, domain.ToolCall{
				ID:   item.CallID,
				Type: "function",
				Function: domain.ToolCallFunction{
					Name:      item.Name,
					Arguments: item.Arguments,
				},
			})
		}
	}
	return result, nil
}

func (r *ProviderRouter) responsesStream(ctx context.Context, target upstreamTarget, req nativeTextRequest, onEvent domain.StreamEventHandler) error {
	tools := make([]responsesTool, 0, len(req.Tools))
	for _, tool := range req.Tools {
		tools = append(tools, responsesTool{Type: "function", Name: tool.Function.Name, Description: tool.Function.Description, Parameters: tool.Function.Parameters})
	}
	var reasoning *responsesReasoning
	if req.ReasoningEffort != "" {
		reasoning = &responsesReasoning{Effort: req.ReasoningEffort, Summary: "auto"}
	}

	parallelTools := true
	if req.ParallelToolCalls != nil {
		parallelTools = *req.ParallelToolCalls
	}
	body, err := json.Marshal(responsesRequest{
		Model:             req.Model,
		Instructions:      req.SystemPrompt,
		Input:             toResponsesInput(req),
		Tools:             tools,
		ToolChoice:        req.ToolChoice,
		ParallelToolCalls: parallelTools,
		Reasoning:         reasoning,
		Temperature:       req.Temperature,
		TopP:              req.TopP,
		PresencePenalty:   req.PresencePenalty,
		FrequencyPenalty:  req.FrequencyPenalty,
		Seed:              req.Seed,
		Stop:              req.Stop,
		MaxOutputTokens:   maxInt(req.MaxTokens, 8192),
		Stream:            true,
	})
	if err != nil {
		return err
	}
	usage := domain.GenerationUsage{}
	doneSent := false
	toolCallArgs := make(map[string]*strings.Builder)

	err = r.nativeSSE(ctx, target, "/responses", body, nil, func(event string, data []byte) error {
		if event == "error" {
			var errPayload struct {
				Error struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			_ = json.Unmarshal(data, &errPayload)
			msg := errPayload.Error.Message
			if msg == "" {
				msg = string(data)
			}
			return platformerr.New(http.StatusInternalServerError, "upstream_stream_error", msg)
		}

		var payload struct {
			Delta    string            `json:"delta"`
			CallID   string            `json:"call_id"`
			Name     string            `json:"name"`
			Response responsesResponse `json:"response"`
		}
		if json.Unmarshal(data, &payload) != nil {
			return nil
		}

		switch event {
		case "response.reasoning.delta":
			if payload.Delta != "" {
				if err := onEvent(domain.StreamEvent{Type: domain.StreamEventThink, Delta: payload.Delta}); err != nil {
					return err
				}
			}
		case "response.output_text.delta":
			if payload.Delta != "" {
				if err := onEvent(domain.StreamEvent{Type: domain.StreamEventMessage, Delta: payload.Delta}); err != nil {
					return err
				}
			}
		case "response.function_call_arguments.delta":
			if payload.CallID != "" {
				b, exists := toolCallArgs[payload.CallID]
				if !exists {
					b = &strings.Builder{}
					toolCallArgs[payload.CallID] = b
				}
				b.WriteString(payload.Delta)
			}
		case "response.output_item.done":
			if payload.CallID != "" {
				args := ""
				if b, exists := toolCallArgs[payload.CallID]; exists {
					args = b.String()
				}
				_ = onEvent(domain.StreamEvent{
					Type:       domain.StreamEventToolCall,
					ToolCallID: payload.CallID,
					ToolName:   payload.Name,
					ToolArgs:   args,
				})
			}
		case "response.completed":
			usage = domain.GenerationUsage{
				InputTokens:  payload.Response.Usage.InputTokens,
				OutputTokens: payload.Response.Usage.OutputTokens,
				TotalTokens:  payload.Response.Usage.TotalTokens,
			}
			doneSent = true
			return onEvent(domain.StreamEvent{Type: domain.StreamEventDone, Usage: &usage})
		}

		if payload.Delta != "" && event == "" {
			return onEvent(domain.StreamEvent{Type: domain.StreamEventMessage, Delta: payload.Delta})
		}
		return nil
	})
	if err == nil && !doneSent {
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens
		err = onEvent(domain.StreamEvent{Type: domain.StreamEventDone, Usage: &usage})
	}
	return err
}

func (r *ProviderRouter) doJSON(ctx context.Context, target upstreamTarget, path string, body []byte, decorate func(*http.Request), out any) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(target.baseURL, "/")+path, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	if normalizeProtocol(r.protocol) == protocolAnthropic {
		request.Header.Set("x-api-key", target.apiKey)
	} else if normalizeProtocol(r.protocol) != protocolGemini {
		request.Header.Set("Authorization", "Bearer "+target.apiKey)
	}
	if decorate != nil {
		decorate(request)
	}
	response, err := r.client.Do(request)
	if err != nil {
		return "", err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", parseUpstreamHTTPError(response)
	}
	if err := json.NewDecoder(response.Body).Decode(out); err != nil {
		return "", err
	}
	return response.Header.Get("x-request-id"), nil
}

func (r *ProviderRouter) nativeSSE(ctx context.Context, target upstreamTarget, path string, body []byte, decorate func(*http.Request), consume func(string, []byte) error) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(target.baseURL, "/")+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "text/event-stream")
	if normalizeProtocol(r.protocol) == protocolAnthropic {
		request.Header.Set("x-api-key", target.apiKey)
	} else if normalizeProtocol(r.protocol) != protocolGemini {
		request.Header.Set("Authorization", "Bearer "+target.apiKey)
	}
	if decorate != nil {
		decorate(request)
	}
	response, err := r.client.Do(request)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return parseUpstreamHTTPError(response)
	}
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	eventName := "message"
	data := bytes.Buffer{}
	flush := func() error {
		if data.Len() == 0 {
			return nil
		}
		err := consume(eventName, append([]byte(nil), data.Bytes()...))
		data.Reset()
		eventName = "message"
		return err
	}
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "event:"):
			eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			if data.Len() > 0 {
				data.WriteByte(byte('\n'))
			}
			data.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		case strings.TrimSpace(line) == "":
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return flush()
}

func maxInt(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func (r *ProviderRouter) generateNativeChatStream(ctx context.Context, target upstreamTarget, req domain.TextGenerationRequest, model string) (err error) {
	startedAt := time.Now()
	var finalUsage domain.GenerationUsage
	collectedToolCalls := make([]domain.ToolCall, 0)

	defer func() {
		r.recordUpstream(target, err)
		r.recordUsage(ctx, req.RequestID, req.TaskKey, model, target, startedAt, finalUsage.InputTokens, finalUsage.OutputTokens, err)
	}()

	err = r.nativeStream(ctx, target, nativeToolRequest(req, model), func(event domain.StreamEvent) error {
		switch event.Type {
		case domain.StreamEventThink:
			if req.OnReasoningChunk != nil {
				return req.OnReasoningChunk(event.Delta)
			}
		case domain.StreamEventMessage:
			if req.OnTextChunk != nil {
				return req.OnTextChunk(event.Delta)
			}
		case domain.StreamEventToolCall:
			collectedToolCalls = append(collectedToolCalls, domain.ToolCall{
				ID:   event.ToolCallID,
				Type: "function",
				Function: domain.ToolCallFunction{
					Name:      event.ToolName,
					Arguments: event.ToolArgs,
				},
			})
		case domain.StreamEventDone:
			if event.Usage != nil {
				finalUsage = *event.Usage
				if req.OnUsage != nil {
					req.OnUsage(&domain.TokenUsage{
						PromptTokens:     event.Usage.InputTokens,
						CompletionTokens: event.Usage.OutputTokens,
						TotalTokens:      event.Usage.TotalTokens,
						InputTokens:      event.Usage.InputTokens,
						OutputTokens:     event.Usage.OutputTokens,
					})
				}
			}
		}
		return nil
	})

	if len(collectedToolCalls) > 0 && req.OnToolCalls != nil {
		req.OnToolCalls(collectedToolCalls)
	}
	return err
}
