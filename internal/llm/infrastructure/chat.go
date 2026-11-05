package infrastructure

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"InkLore/internal/llm/domain"
	platformerr "InkLore/internal/platform/errors"
)

type openAIToolCallPayload struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type openAIToolMessagePayload struct {
	Role             string                  `json:"role"`
	Content          string                  `json:"content"`
	ReasoningContent string                  `json:"reasoning_content,omitempty"`
	ToolCalls        []openAIToolCallPayload `json:"tool_calls,omitempty"`
	ToolCallID       string                  `json:"tool_call_id,omitempty"`
	Name             string                  `json:"name,omitempty"`
}

type openAIToolDefinitionPayload struct {
	Type     string `json:"type"`
	Function struct {
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		Parameters  any    `json:"parameters,omitempty"`
	} `json:"function"`
}

type streamOptionsPayload struct {
	IncludeUsage bool `json:"include_usage"`
}

type openAIToolChatRequest struct {
	Model               string                        `json:"model"`
	Messages            []openAIToolMessagePayload    `json:"messages"`
	Tools               []openAIToolDefinitionPayload `json:"tools,omitempty"`
	ToolChoice          any                           `json:"tool_choice,omitempty"`
	ResponseFormat      *domain.ResponseFormat        `json:"response_format,omitempty"`
	Thinking            any                           `json:"thinking,omitempty"`
	ReasoningEffort     string                        `json:"reasoning_effort,omitempty"`
	Temperature         float64                       `json:"temperature,omitempty"`
	TopP                float64                       `json:"top_p,omitempty"`
	MaxTokens           int                           `json:"max_tokens,omitempty"`
	MaxCompletionTokens int                           `json:"max_completion_tokens,omitempty"`
	Stop                []string                      `json:"stop,omitempty"`
	PresencePenalty     float64                       `json:"presence_penalty,omitempty"`
	FrequencyPenalty    float64                       `json:"frequency_penalty,omitempty"`
	Seed                *int                          `json:"seed,omitempty"`
	ParallelToolCalls   *bool                         `json:"parallel_tool_calls,omitempty"`
	ServiceTier         string                        `json:"service_tier,omitempty"`
	Stream              bool                          `json:"stream"`
	StreamOptions       *streamOptionsPayload         `json:"stream_options,omitempty"`
}

type openAIToolChatResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content          string                  `json:"content"`
			ReasoningContent string                  `json:"reasoning_content,omitempty"`
			ToolCalls        []openAIToolCallPayload `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens          int `json:"prompt_tokens"`
		CompletionTokens      int `json:"completion_tokens"`
		TotalTokens           int `json:"total_tokens"`
		PromptCacheHitTokens  int `json:"prompt_cache_hit_tokens,omitempty"`
		PromptCacheMissTokens int `json:"prompt_cache_miss_tokens,omitempty"`
		PromptTokensDetails   struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"prompt_tokens_details,omitempty"`
		CompletionTokensDetails struct {
			ReasoningTokens int `json:"reasoning_tokens"`
		} `json:"completion_tokens_details,omitempty"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func isReasoningModel(model string) bool {
	m := strings.ToLower(model)
	return strings.HasPrefix(m, "o1") ||
		strings.HasPrefix(m, "o3") ||
		strings.HasPrefix(m, "o4") ||
		strings.HasPrefix(m, "gpt-5") ||
		strings.HasPrefix(m, "gpt-6") ||
		strings.Contains(m, "-reasoner") ||
		strings.Contains(m, "deepseek-r1") ||
		strings.Contains(m, "deepseek-r2") ||
		strings.Contains(m, "deepseek-v4") ||
		strings.Contains(m, "glm-5") ||
		strings.Contains(m, "kimi-k2") ||
		strings.Contains(m, "kimi-k3") ||
		strings.Contains(m, "qwq") ||
		strings.Contains(m, "qwen3-max")
}

// Chat-compatible providers do not share OpenAI's reasoning token parameter.
// Send only one output limit; DeepSeek, Ollama and other compatible models
// continue to use max_tokens even when their names describe reasoning models.
func chatTokenLimits(protocol, model string, limit int) (maxTokens, maxCompletionTokens int) {
	m := strings.ToLower(model)
	if normalizeProtocol(protocol) == protocolOpenAI &&
		(strings.HasPrefix(m, "o1") || strings.HasPrefix(m, "o3") ||
			strings.HasPrefix(m, "o4") || strings.HasPrefix(m, "gpt-5") || strings.HasPrefix(m, "gpt-6")) {
		return 0, limit
	}
	return limit, 0
}

func (r *ProviderRouter) GenerateChat(ctx context.Context, req domain.TextGenerationRequest) (response domain.TextGenerationResponse, err error) {
	if r == nil || !r.isLiveAPI {
		return domain.TextGenerationResponse{}, domain.ErrProviderNotConfigured
	}
	target, model, targetErr := r.resolveChatTarget(ctx, req)
	if targetErr != nil {
		return domain.TextGenerationResponse{}, targetErr
	}
	if nativeProtocol(target.protocol) {
		return r.generateNativeToolChat(ctx, target, req, model)
	}

	startedAt := time.Now()
	defer func() {
		r.recordUpstream(target, err)
		inputTokens, outputTokens := 0, 0
		if err == nil && response.Usage != nil {
			inputTokens = response.Usage.PromptTokens
			outputTokens = response.Usage.CompletionTokens
		}
		r.recordUsage(ctx, req.RequestID, req.TaskKey, model, target, startedAt, inputTokens, outputTokens, err)
	}()

	var thinking any
	if req.ThinkingEnabled != nil {
		if *req.ThinkingEnabled {
			thinking = map[string]string{"type": "enabled"}
		} else {
			thinking = map[string]string{"type": "disabled"}
		}
	}

	reasoning := isReasoningModel(model)
	temp := req.Temperature
	maxTokens, maxCompletionTokens := chatTokenLimits(target.protocol, model, req.MaxTokens)
	if reasoning {
		// Reasoning models may reject custom sampling parameters.
		temp = 0
	}

	payload := openAIToolChatRequest{
		Model:               model,
		Messages:            toOpenAIToolMessages(req),
		Tools:               toOpenAIToolDefinitions(req.Tools),
		ToolChoice:          req.ToolChoice,
		ResponseFormat:      req.ResponseFormat,
		Thinking:            thinking,
		ReasoningEffort:     req.ReasoningEffort,
		Temperature:         temp,
		TopP:                req.TopP,
		MaxTokens:           maxTokens,
		MaxCompletionTokens: maxCompletionTokens,
		Stop:                req.Stop,
		PresencePenalty:     req.PresencePenalty,
		FrequencyPenalty:    req.FrequencyPenalty,
		Seed:                req.Seed,
		ParallelToolCalls:   req.ParallelToolCalls,
		ServiceTier:         resolveServiceTier(req.ServiceTier, req.FastMode),
		Stream:              false,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return domain.TextGenerationResponse{}, platformerr.Internal("llm_marshal_failed", "llm.generation_failed", err.Error())
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", target.baseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return domain.TextGenerationResponse{}, platformerr.Internal("llm_request_failed", "llm.generation_failed", err.Error())
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+target.apiKey)

	resp, err := r.client.Do(httpReq)
	if err != nil {
		return domain.TextGenerationResponse{}, platformerr.Internal("llm_call_failed", "llm.generation_failed", err.Error())
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return domain.TextGenerationResponse{}, platformerr.Wrap(parseUpstreamHTTPError(resp), http.StatusInternalServerError, "upstream_error", "llm.generation_failed", resp.StatusCode)
	}

	var chatResp openAIToolChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return domain.TextGenerationResponse{}, platformerr.Internal("llm_decode_failed", "llm.generation_failed", err.Error())
	}
	if chatResp.Error != nil {
		return domain.TextGenerationResponse{}, platformerr.Wrap(newUpstreamStatusError(http.StatusBadGateway, chatResp.Error.Message), http.StatusInternalServerError, "upstream_error", "llm.generation_failed")
	}
	if len(chatResp.Choices) == 0 {
		return domain.TextGenerationResponse{}, platformerr.Wrap(newUpstreamStatusError(http.StatusBadGateway, "missing chat completion choices"), http.StatusInternalServerError, "upstream_error", "llm.generation_failed")
	}

	cached := chatResp.Usage.PromptCacheHitTokens
	if cached == 0 {
		cached = chatResp.Usage.PromptTokensDetails.CachedTokens
	}
	reasoningTokens := chatResp.Usage.CompletionTokensDetails.ReasoningTokens

	result := domain.TextGenerationResponse{
		Model: model,
		Usage: &domain.TokenUsage{
			PromptTokens:          chatResp.Usage.PromptTokens,
			CompletionTokens:      chatResp.Usage.CompletionTokens,
			TotalTokens:           chatResp.Usage.TotalTokens,
			ReasoningTokens:       reasoningTokens,
			CachedTokens:          cached,
			PromptCacheHitTokens:  chatResp.Usage.PromptCacheHitTokens,
			PromptCacheMissTokens: chatResp.Usage.PromptCacheMissTokens,
			InputTokens:           chatResp.Usage.PromptTokens,
			OutputTokens:          chatResp.Usage.CompletionTokens,
		},
	}
	if len(chatResp.Choices) > 0 {
		choice := chatResp.Choices[0]
		result.Content = choice.Message.Content
		result.Thinking = choice.Message.ReasoningContent
		if result.Thinking == "" && strings.Contains(result.Content, "<think>") {
			result.Thinking, result.Content = extractThinkTag(result.Content)
		}
		result.FinishReason = choice.FinishReason
		result.ToolCalls = fromOpenAIToolCalls(choice.Message.ToolCalls)
	}
	if chatResp.Model != "" {
		result.Model = chatResp.Model
	}
	return result, nil
}

func extractThinkTag(content string) (thinking, cleanContent string) {
	start := strings.Index(content, "<think>")
	if start == -1 {
		return "", content
	}
	end := strings.Index(content, "</think>")
	if end == -1 {
		return strings.TrimSpace(content[start+len("<think>"):]), strings.TrimSpace(content[:start])
	}
	thinking = strings.TrimSpace(content[start+len("<think>") : end])
	cleanContent = strings.TrimSpace(content[:start] + content[end+len("</think>"):])
	return thinking, cleanContent
}

func toOpenAIToolMessages(req domain.TextGenerationRequest) []openAIToolMessagePayload {
	messages := make([]openAIToolMessagePayload, 0, len(req.Messages)+1)
	if strings.TrimSpace(req.SystemPrompt) != "" {
		messages = append(messages, openAIToolMessagePayload{Role: domain.ChatMessageRoleSystem, Content: req.SystemPrompt})
	}
	for _, message := range req.Messages {
		content := message.Content
		reasoning := message.ReasoningContent
		// 针对 DeepSeek-Reasoner 等多轮思考模型：若历史 assistant 消息中思考过程内嵌在 content 里，自动提取并分流回传
		if message.Role == domain.ChatMessageRoleAssistant && reasoning == "" && strings.Contains(content, "<think>") {
			reasoning, content = extractThinkTag(content)
		}

		payload := openAIToolMessagePayload{
			Role:             message.Role,
			Content:          content,
			ReasoningContent: reasoning,
			ToolCallID:       message.ToolCallID,
			Name:             message.Name,
		}
		for _, call := range message.ToolCalls {
			payload.ToolCalls = append(payload.ToolCalls, openAIToolCallPayload{
				ID:   call.ID,
				Type: normalizeToolCallType(call.Type),
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{Name: call.Function.Name, Arguments: call.Function.Arguments},
			})
		}
		messages = append(messages, payload)
	}
	return messages
}

func toOpenAIToolDefinitions(tools []domain.Tool) []openAIToolDefinitionPayload {
	if len(tools) == 0 {
		return nil
	}
	definitions := make([]openAIToolDefinitionPayload, 0, len(tools))
	for _, tool := range tools {
		definition := openAIToolDefinitionPayload{Type: normalizeToolCallType(tool.Type)}
		definition.Function.Name = tool.Function.Name
		definition.Function.Description = tool.Function.Description
		definition.Function.Parameters = tool.Function.Parameters
		definitions = append(definitions, definition)
	}
	return definitions
}

func fromOpenAIToolCalls(calls []openAIToolCallPayload) []domain.ToolCall {
	if len(calls) == 0 {
		return nil
	}
	result := make([]domain.ToolCall, 0, len(calls))
	for _, call := range calls {
		result = append(result, domain.ToolCall{
			ID:   call.ID,
			Type: normalizeToolCallType(call.Type),
			Function: domain.ToolCallFunction{
				Name:      call.Function.Name,
				Arguments: call.Function.Arguments,
			},
		})
	}
	return result
}

func normalizeToolCallType(value string) string {
	if strings.TrimSpace(value) == "" {
		return "function"
	}
	return value
}

func resolveServiceTier(tier string, fastMode bool) string {
	t := strings.TrimSpace(tier)
	if t != "" {
		return t
	}
	if fastMode {
		return "fast"
	}
	return ""
}

// openAIStreamChunk 覆盖 OpenAI 及 DeepSeek 最新流式协议规范，
// 包含 reasoning_content、tool_calls 增量、stream_options 触发的细粒度用量与缓存统计，以及中途错误块。
type openAIStreamChunk struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int    `json:"index"`
		FinishReason string `json:"finish_reason"`
		Delta        struct {
			Role             string                  `json:"role"`
			Content          string                  `json:"content"`
			ReasoningContent string                  `json:"reasoning_content"`
			ToolCalls        []openAIStreamToolDelta `json:"tool_calls"`
		} `json:"delta"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens          int `json:"prompt_tokens"`
		CompletionTokens      int `json:"completion_tokens"`
		TotalTokens           int `json:"total_tokens"`
		PromptCacheHitTokens  int `json:"prompt_cache_hit_tokens,omitempty"`
		PromptCacheMissTokens int `json:"prompt_cache_miss_tokens,omitempty"`
		PromptTokensDetails   struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"prompt_tokens_details,omitempty"`
		CompletionTokensDetails struct {
			ReasoningTokens int `json:"reasoning_tokens"`
		} `json:"completion_tokens_details,omitempty"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    any    `json:"code,omitempty"`
	} `json:"error,omitempty"`
}

type openAIStreamToolDelta struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type toolCallBuilder struct {
	id        string
	callType  string
	name      string
	arguments strings.Builder
}

type toolCallAccumulator struct {
	builders map[int]*toolCallBuilder
}

func newToolCallAccumulator() *toolCallAccumulator {
	return &toolCallAccumulator{builders: make(map[int]*toolCallBuilder)}
}

func (a *toolCallAccumulator) Process(deltas []openAIStreamToolDelta) {
	for _, d := range deltas {
		b, exists := a.builders[d.Index]
		if !exists {
			b = &toolCallBuilder{}
			a.builders[d.Index] = b
		}
		if d.ID != "" {
			b.id = d.ID
		}
		if d.Type != "" {
			b.callType = d.Type
		}
		if d.Function.Name != "" {
			b.name = d.Function.Name
		}
		if d.Function.Arguments != "" {
			b.arguments.WriteString(d.Function.Arguments)
		}
	}
}

func (a *toolCallAccumulator) Build() []domain.ToolCall {
	if len(a.builders) == 0 {
		return nil
	}
	maxIdx := -1
	for idx := range a.builders {
		if idx > maxIdx {
			maxIdx = idx
		}
	}
	result := make([]domain.ToolCall, 0, len(a.builders))
	for i := 0; i <= maxIdx; i++ {
		b, ok := a.builders[i]
		if !ok || b == nil {
			continue
		}
		callType := b.callType
		if callType == "" {
			callType = "function"
		}
		result = append(result, domain.ToolCall{
			ID:   b.id,
			Type: callType,
			Function: domain.ToolCallFunction{
				Name:      b.name,
				Arguments: b.arguments.String(),
			},
		})
	}
	return result
}

// decodeOpenAIChatStream 解码 OpenAI/DeepSeek 兼容的 SSE 生成流，
// 支持实时提取思考过程、打字机正文、流式累加工具调用、截取中途错误块并捕获终态用量统计。
// 接入 context 级联超时与取消检测，扩充 scanner 缓冲区防止长文本崩溃。
func decodeOpenAIChatStream(ctx context.Context, body io.Reader, req domain.TextGenerationRequest) (*domain.TokenUsage, error) {
	thinkFilter := domain.NewThinkStreamFilter()
	accumulator := newToolCallAccumulator()
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	var finalUsage *domain.TokenUsage
	completed := false
	seenChoice := false

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return finalUsage, ctx.Err()
		default:
		}

		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			completed = true
			break
		}

		var chunk openAIStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return finalUsage, platformerr.Wrap(newUpstreamStatusError(http.StatusBadGateway, "invalid chat stream event"), http.StatusInternalServerError, "upstream_stream_error", "llm.generation_failed")
		}

		// 拦截流式传输中途上游返回的错误块
		if chunk.Error != nil {
			return finalUsage, platformerr.Wrap(newUpstreamStatusError(http.StatusBadGateway, chunk.Error.Message), http.StatusInternalServerError, "upstream_stream_error", "llm.generation_failed")
		}

		if chunk.Usage != nil {
			cached := chunk.Usage.PromptCacheHitTokens
			if cached == 0 {
				cached = chunk.Usage.PromptTokensDetails.CachedTokens
			}
			reasoning := chunk.Usage.CompletionTokensDetails.ReasoningTokens
			finalUsage = &domain.TokenUsage{
				PromptTokens:          chunk.Usage.PromptTokens,
				CompletionTokens:      chunk.Usage.CompletionTokens,
				TotalTokens:           chunk.Usage.TotalTokens,
				ReasoningTokens:       reasoning,
				CachedTokens:          cached,
				PromptCacheHitTokens:  chunk.Usage.PromptCacheHitTokens,
				PromptCacheMissTokens: chunk.Usage.PromptCacheMissTokens,
				InputTokens:           chunk.Usage.PromptTokens,
				OutputTokens:          chunk.Usage.CompletionTokens,
			}
		}

		if len(chunk.Choices) > 0 {
			seenChoice = true
			choice := chunk.Choices[0]
			if len(choice.Delta.ToolCalls) > 0 {
				accumulator.Process(choice.Delta.ToolCalls)
			}

			rawReasoning := choice.Delta.ReasoningContent
			rawContent := choice.Delta.Content
			thinkDelta, contentDelta := thinkFilter.ProcessChunk(rawReasoning, rawContent)

			if thinkDelta != "" && req.OnReasoningChunk != nil {
				if err := req.OnReasoningChunk(thinkDelta); err != nil {
					return finalUsage, err
				}
			}
			if contentDelta != "" && req.OnTextChunk != nil {
				if err := req.OnTextChunk(contentDelta); err != nil {
					return finalUsage, err
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return finalUsage, err
	}
	if err := ctx.Err(); err != nil {
		return finalUsage, err
	}
	if !completed {
		return finalUsage, io.ErrUnexpectedEOF
	}
	if !seenChoice {
		return finalUsage, platformerr.Wrap(newUpstreamStatusError(http.StatusBadGateway, "missing chat stream choices"), http.StatusInternalServerError, "upstream_stream_error", "llm.generation_failed")
	}

	flushThink, flushContent := thinkFilter.Flush()
	if flushThink != "" && req.OnReasoningChunk != nil {
		if err := req.OnReasoningChunk(flushThink); err != nil {
			return finalUsage, err
		}
	}
	if flushContent != "" && req.OnTextChunk != nil {
		if err := req.OnTextChunk(flushContent); err != nil {
			return finalUsage, err
		}
	}

	calls := accumulator.Build()
	if len(calls) > 0 && req.OnToolCalls != nil {
		req.OnToolCalls(calls)
	}

	if finalUsage != nil && req.OnUsage != nil {
		req.OnUsage(finalUsage)
	}

	return finalUsage, nil
}

// parseUpstreamHTTPError 提取上游非 200 HTTP 响应中的结构化错误详情，兼容各供应商响应格式
func parseUpstreamHTTPError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16384))
	msg := ""

	// 1. 标准 OpenAI / DeepSeek / 通用格式
	var errPayload struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    any    `json:"code"`
		} `json:"error"`
		ErrorMsg string `json:"error_msg"`
		Message  string `json:"message"`
		Detail   string `json:"detail"`
	}
	if json.Unmarshal(body, &errPayload) == nil {
		msg = errPayload.Error.Message
		if msg == "" {
			msg = errPayload.ErrorMsg
		}
		if msg == "" {
			msg = errPayload.Message
		}
		if msg == "" {
			msg = errPayload.Detail
		}
	}

	// 2. Google Gemini 数组格式或带 status 格式
	if msg == "" {
		var geminiArr []struct {
			Error struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
				Status  string `json:"status"`
			} `json:"error"`
		}
		if json.Unmarshal(body, &geminiArr) == nil && len(geminiArr) > 0 && geminiArr[0].Error.Message != "" {
			msg = geminiArr[0].Error.Message
		}
	}

	if msg == "" {
		trimmed := strings.TrimSpace(string(body))
		if len(trimmed) > 0 && len(trimmed) < 2000 {
			msg = trimmed
		}
	}
	if msg == "" {
		msg = http.StatusText(resp.StatusCode)
	}
	return newUpstreamStatusError(resp.StatusCode, msg)
}
