package engineloop

import (
	"InkLore/internal/platform/timeutil"
	"context"
	"fmt"
	"strings"
	"time"

	agentdomain "InkLore/internal/agent/domain"
	ai "InkLore/internal/llm/domain"
	aigateway "InkLore/internal/llm/domain/gateway"
)

type thinkTurnResult struct {
	Response    *ai.TextGenerationResponse
	TurnUsage   *ai.TokenUsage
	ActiveModel string
	Latency     agentdomain.LatencyBreakdown
	Cost        agentdomain.CostBreakdown
	Span        agentdomain.TraceSpan
}

// executeThinkTurn 执行单轮模型推理（Think 阶段），精确采集分段延迟与链路 Span
func (a *Agent) executeThinkTurn(
	ctx context.Context,
	req *RunRequest,
	turns int,
	activeModel string,
	messages []ai.ChatMessage,
	toolDefs []ai.Tool,
	pricing *agentdomain.ModelPriceInfo,
	modelMW []ModelCallMiddleware,
	emit func(AgentEvent) error,
) (*thinkTurnResult, error) {
	startedAt := timeutil.Now().UTC()
	var (
		firstChunkAt     time.Time
		thinkingFinished time.Time
	)

	if err := emit(AgentEvent{
		Type:    StandardEventThinkingDelta,
		Turn:    turns,
		Content: fmt.Sprintf("第 %d 轮思考...", turns),
		Data:    ThinkingDeltaData{Turn: turns},
	}); err != nil {
		return nil, err
	}

	var (
		thinkingContent  strings.Builder
		streamingContent strings.Builder
		capturedTools    []ai.ToolCall
		turnUsage        *ai.TokenUsage
	)

	toolChoice := interface{}("auto")
	if req.ToolChoice != nil {
		toolChoice = req.ToolChoice
	}
	genReq := &ai.TextGenerationRequest{
		SystemPrompt: req.SystemPrompt,
		Messages:     messages,
		Tools:        toolDefs,
		ToolChoice:   toolChoice,
		Model:        activeModel,
		Temperature:  req.Temperature,
		TopP:         req.TopP,
		MaxTokens:    req.MaxTokens,
		Options:      req.Options,
	}
	turnPromptMessages := append([]ai.ChatMessage(nil), messages...)
	modelCtx := &ModelCallContext{
		Turn:          turns,
		Provider:      a.resolveProvider(req.Provider),
		Request:       req,
		GenerationReq: genReq,
		Messages:      turnPromptMessages,
		Tools:         append([]ai.Tool(nil), toolDefs...),
	}
	for _, middleware := range modelMW {
		if err := middleware.BeforeModelCall(ctx, modelCtx); err != nil {
			if emitErr := emit(ErrorStreamEvent(err.Error(), ErrorCodeInternalError, ErrorReasonModelMiddlewareFailed, map[string]interface{}{"turn": turns})); emitErr != nil {
				return nil, emitErr
			}
			return nil, fmt.Errorf("agent model middleware before turn %d failed: %w", turns, err)
		}
	}

	// 注册流式分块回调并打点首字延迟
	recordFirstChunk := func() {
		if firstChunkAt.IsZero() {
			firstChunkAt = timeutil.Now().UTC()
		}
	}

	genReq.OnReasoningChunk = func(chunk string) error {
		recordFirstChunk()
		thinkingContent.WriteString(chunk)
		return emit(AgentEvent{
			Type:    StandardEventThinkingDelta,
			Turn:    turns,
			Content: chunk,
			Data:    ThinkingDeltaData{Turn: turns, Delta: chunk},
		})
	}

	genReq.OnTextChunk = func(chunk string) error {
		recordFirstChunk()
		if thinkingFinished.IsZero() && thinkingContent.Len() > 0 {
			thinkingFinished = timeutil.Now().UTC()
		}
		streamingContent.WriteString(chunk)
		return emit(AgentEvent{
			Type:    StandardEventTextDelta,
			Turn:    turns,
			Content: chunk,
		})
	}

	genReq.OnToolCalls = func(toolCalls []ai.ToolCall) {
		capturedTools = cloneAgentToolCalls(toolCalls)
	}
	genReq.OnUsage = func(usage *ai.TokenUsage) {
		turnUsage = ai.CloneTokenUsage(usage)
	}

	attempts := req.Retry.attempts()
	fallbackModel := strings.TrimSpace(req.Retry.FallbackModel)
	hasFallback := fallbackModel != ""
	usingFallback := false

	var err error
	for attempt := 1; attempt <= attempts; attempt++ {
		// 取消属于调用生命周期终止，不能被供应商可重试错误覆盖。
		if err = ctx.Err(); err != nil {
			break
		}
		if attempt > 1 || usingFallback {
			thinkingContent.Reset()
			streamingContent.Reset()
			capturedTools = nil
			turnUsage = nil
			firstChunkAt = time.Time{}
			if err = emit(AgentEvent{
				Type:    StandardEventThinkingDelta,
				Turn:    turns,
				Content: fmt.Sprintf("第 %d 轮第 %d 次重试...", turns, attempt),
				Data:    ThinkingDeltaData{Turn: turns, Attempt: attempt, Retry: true},
			}); err != nil {
				break
			}
		}
		gateway := a.agentTextGateway()
		if gateway == nil {
			err = fmt.Errorf("agent text gateway is not configured")
		} else {
			err = gateway.ExecuteTextStream(ctx, aigateway.TextStreamRequest{
				Provider: a.resolveProvider(req.Provider),
				Model:    activeModel,
				Text: &aigateway.TextRequest{
					SystemPrompt: genReq.SystemPrompt,
					Messages:     agentGatewayTextMessages(genReq.Messages),
					Tools:        agentGatewayTextTools(genReq.Tools),
					ToolChoice:   genReq.ToolChoice,
					Model:        genReq.Model,
					MaxTokens:    genReq.MaxTokens,
					Temperature:  genReq.Temperature,
					TopP:         genReq.TopP,
					Options:      cloneAgentGatewayOptions(genReq.Options),
					Raw:          genReq,
				},
				AuditRef: agentRunAuditRef(req),
				OnChunk: func(chunk string, done bool) error {
					if chunk != "" {
						recordFirstChunk()
						streamingContent.WriteString(chunk)
						return emit(AgentEvent{
							Type:    StandardEventTextDelta,
							Turn:    turns,
							Content: chunk,
						})
					}
					return nil
				},
			})
		}
		if cancelErr := ctx.Err(); cancelErr != nil {
			err = cancelErr
			break
		}
		// 已投递的流内容无法撤回；断流后停止本轮，不拼接备用模型或重放工具。
		if err != nil && (streamingContent.Len() > 0 || thinkingContent.Len() > 0 || len(capturedTools) > 0) {
			break
		}
		if err == nil && IsEmptyModelResponse(streamingContent.String(), len(capturedTools)) {
			err = fmt.Errorf("empty response")
		}
		if attempt == attempts && hasFallback && !usingFallback && err != nil && IsRetryableModelError(err) {
			usingFallback = true
			attempt--
			activeModel = fallbackModel
			genReq.Model = activeModel
			if err = emit(AgentEvent{
				Type:    StandardEventThinkingDelta,
				Turn:    turns,
				Content: "主模型失败，切换备用模型重试...",
				Data:    ThinkingDeltaData{Turn: turns, Fallback: true, FallbackModel: fallbackModel},
			}); err != nil {
				break
			}
			continue
		}
		if err == nil || attempt >= attempts || !IsRetryableModelError(err) {
			break
		}
	}

	endedAt := timeutil.Now().UTC()
	totalMs := endedAt.Sub(startedAt).Milliseconds()
	ttftMs := int64(0)
	if !firstChunkAt.IsZero() {
		ttftMs = firstChunkAt.Sub(startedAt).Milliseconds()
	}
	thinkingMs := int64(0)
	if !thinkingFinished.IsZero() {
		thinkingMs = thinkingFinished.Sub(startedAt).Milliseconds()
	}

	latency := agentdomain.LatencyBreakdown{
		TTFTMs:     ttftMs,
		ThinkingMs: thinkingMs,
		StreamMs:   totalMs - ttftMs,
		TotalMs:    totalMs,
	}

	if err != nil {
		for _, middleware := range modelMW {
			_ = middleware.AfterModelCall(ctx, &ModelCallResult{Turn: turns, Request: req, Error: err})
		}
		if emitErr := emit(ErrorStreamEvent(err.Error(), ErrorCodeInternalError, ErrorReasonModelStreamFailed, map[string]interface{}{"turn": turns, "latency_ms": totalMs})); emitErr != nil {
			return nil, emitErr
		}
		return nil, fmt.Errorf("agent think turn %d stream failed: %w", turns, err)
	}

	if turnUsage == nil {
		turnUsage = estimateTurnUsage(req, turnPromptMessages, streamingContent.String())
	}

	cost := agentdomain.CalculateCostWithPricing(pricing, turnUsage)

	resp := &ai.TextGenerationResponse{
		Content:   streamingContent.String(),
		ToolCalls: capturedTools,
		Usage:     turnUsage,
	}
	for _, middleware := range modelMW {
		if err := middleware.AfterModelCall(ctx, &ModelCallResult{Turn: turns, Request: req, Response: resp, Usage: turnUsage}); err != nil {
			if emitErr := emit(ErrorStreamEvent(err.Error(), ErrorCodeInternalError, ErrorReasonModelMiddlewareFailed, map[string]interface{}{"turn": turns})); emitErr != nil {
				return nil, emitErr
			}
			return nil, fmt.Errorf("agent model middleware after turn %d failed: %w", turns, err)
		}
	}

	span := agentdomain.TraceSpan{
		SpanID:     fmt.Sprintf("turn_%d_think", turns),
		Kind:       agentdomain.SpanKindThink,
		Name:       fmt.Sprintf("Think (Model: %s)", activeModel),
		StartedAt:  startedAt,
		EndedAt:    endedAt,
		DurationMs: totalMs,
		Status:     "ok",
		Attributes: map[string]any{
			"model":             activeModel,
			"turn":              turns,
			"ttft_ms":           ttftMs,
			"prompt_tokens":     turnUsage.PromptTokens,
			"completion_tokens": turnUsage.CompletionTokens,
			"cost_usd":          cost.TotalCostUSD,
			"tool_calls_count":  len(capturedTools),
		},
	}

	return &thinkTurnResult{
		Response:    resp,
		TurnUsage:   turnUsage,
		ActiveModel: activeModel,
		Latency:     latency,
		Cost:        cost,
		Span:        span,
	}, nil
}
