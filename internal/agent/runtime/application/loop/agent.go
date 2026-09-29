// package agentapp 提供 TAO（Think-Act-Observe）循环的 AI Agent 引擎。
// 支持 SSE 流式推送执行过程。
package engineloop

import (
	"context"
	"fmt"
	"maps"
	"strconv"
	"strings"

	enginedomain "InkLore/internal/agent/domain"
	enginepromptctx "InkLore/internal/agent/runtime/application/promptctx"
	ai "InkLore/internal/llm/domain"
	aigateway "InkLore/internal/llm/domain/gateway"
	"InkLore/internal/platform/logger"
	"InkLore/internal/platform/timeutil"
)

const (
	permissionAlwaysAsk  = "always_ask"
	permissionFullAccess = "full_access"
)

// ToolHandler 工具处理函数（指向 domain 统一事实来源）
type ToolHandler = enginedomain.ToolHandler

// StreamCallback 流式回调（返回 error 可中止 Agent 执行）。
// 事件即信封：所有事件统一为 AgentEvent（见 event_envelope.go）。
type StreamCallback func(event AgentEvent) error

// Agent TAO 循环引擎
type Agent struct {
	textGateway aigateway.ModelExecutionGateway
	tools       map[string]ToolEntry
	provider    string
}

// ToolEntry 注册的工具（指向 domain 统一事实来源）
type ToolEntry = enginedomain.ToolEntry

// RetryPolicy 模型调用重试策略。
type RetryPolicy struct {
	FallbackModel string
	MaxAttempts   int
}

func (p RetryPolicy) attempts() int {
	if p.MaxAttempts < 1 {
		return 1
	}
	if p.MaxAttempts > 10 {
		return 10
	}
	return p.MaxAttempts
}

// RunRequest Agent 执行请求
type RunRequest struct {
	Approvals            ToolApprovalService
	Permissions          ToolPermissionChecker
	Options              map[string]interface{}
	ToolChoice           interface{}
	AgentID              *int64
	OnEvent              StreamCallback
	SessionID            *int64
	Capabilities         *ToolCapabilityRegistry
	RunID                *int64
	Retry                RetryPolicy
	RequestID            string
	TenantID             string
	SystemPrompt         string
	Provider             string
	Model                string
	PermissionLevel      string
	Tools                []ToolEntry
	AllowedTools         []string
	Messages             []ai.ChatMessage
	ApprovalTokens       []string
	Middlewares          []Middleware
	TopP                 float64
	UserID               int64
	BillingReservationID int64
	MaxContextTokens     int
	MaxTokens            int
	Temperature          float64
	MaxTurns             int
	AllowFullAccess      bool
	Pricing              *enginedomain.ModelPriceInfo
	PricingResolver      enginedomain.ModelPricingResolver
	ExchangeRate         float64
	RepeatCallThreshold  int
	Compaction           enginepromptctx.AuxiliaryTaskBinding
	DisableCompaction    bool
}

// RunResponse Agent 执行结果
type RunResponse struct {
	TokenUsage *ai.TokenUsage                `json:"usage,omitempty"`
	Content    string                        `json:"content"`
	Messages   []ai.ChatMessage              `json:"messages,omitempty"`
	TotalTurns int                           `json:"total_turns"`
	Latency    enginedomain.LatencyBreakdown `json:"latency"`
	Cost       enginedomain.CostBreakdown    `json:"cost"`
	TraceSpans []enginedomain.TraceSpan      `json:"trace_spans,omitempty"`
}

// estimateMessagesTokens 估算一组消息（含工具调用）+ 系统提示词的 token 数。
func estimateMessagesTokens(req *RunRequest, messages []ai.ChatMessage, skipIndex int) int {
	modelName := ""
	if req != nil {
		modelName = req.Model
	}
	tokens := 0
	if req != nil && strings.TrimSpace(req.SystemPrompt) != "" {
		tokens += enginedomain.EstimateTokenCountForModel(req.SystemPrompt, modelName) + 4
	}
	for i, msg := range messages {
		if i == skipIndex {
			continue
		}
		tokens += enginedomain.EstimateTokenCountForModel(msg.Content, modelName) + 4
		for _, toolCall := range msg.ToolCalls {
			tokens += enginedomain.EstimateTokenCountForModel(toolCall.Function.Name, modelName)
			tokens += enginedomain.EstimateTokenCountForModel(toolCall.Function.Arguments, modelName)
		}
	}
	return tokens
}

func estimateRunUsage(req *RunRequest, messages []ai.ChatMessage, completion string) *ai.TokenUsage {
	modelName := ""
	if req != nil {
		modelName = req.Model
	}
	finalAssistantIndex := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == ai.ChatMessageRoleAssistant {
			finalAssistantIndex = i
			break
		}
	}
	promptTokens := estimateMessagesTokens(req, messages, finalAssistantIndex)
	completionTokens := enginedomain.EstimateTokenCountForModel(completion, modelName)
	if completionTokens == 0 {
		for _, msg := range messages {
			if msg.Role == ai.ChatMessageRoleAssistant {
				completionTokens += enginedomain.EstimateTokenCountForModel(msg.Content, modelName) + 4
			}
		}
	}
	return &ai.TokenUsage{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      promptTokens + completionTokens,
		Estimated:        true,
	}
}

func estimateTurnUsage(req *RunRequest, promptMessages []ai.ChatMessage, completion string) *ai.TokenUsage {
	modelName := ""
	if req != nil {
		modelName = req.Model
	}
	promptTokens := estimateMessagesTokens(req, promptMessages, -1)
	completionTokens := enginedomain.EstimateTokenCountForModel(completion, modelName)
	return &ai.TokenUsage{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      promptTokens + completionTokens,
		InputTokens:      promptTokens,
		OutputTokens:     completionTokens,
		Estimated:        true,
	}
}

func agentRunAuditRef(req *RunRequest) aigateway.AuditRef {
	if req == nil {
		return aigateway.AuditRef{Resource: "agent_run"}
	}
	ref := aigateway.AuditRef{
		RequestID: strings.TrimSpace(req.RequestID),
		ActorID:   req.UserID,
		Resource:  "agent_run",
		Metadata: map[string]string{
			"tenant_id":              strings.TrimSpace(req.TenantID),
			"billing_reservation_id": strconv.FormatInt(req.BillingReservationID, 10),
		},
	}
	if req.RunID != nil && *req.RunID > 0 {
		ref.ResourceID = *req.RunID
		ref.RunID = *req.RunID
	}
	return ref
}

// New 创建 Agent
func New(textGateway aigateway.ModelExecutionGateway, provider string) *Agent {
	return &Agent{
		textGateway: textGateway,
		provider:    provider,
		tools:       make(map[string]ToolEntry),
	}
}

// SetTextGateway 注入 Agent LLM 执行网关。
func (a *Agent) SetTextGateway(gateway aigateway.ModelExecutionGateway) {
	a.textGateway = gateway
}

func (a *Agent) agentTextGateway() aigateway.ModelExecutionGateway {
	if a == nil {
		return nil
	}
	return a.textGateway
}

func agentGatewayTextMessages(messages []ai.ChatMessage) []aigateway.TextMessage {
	if len(messages) == 0 {
		return nil
	}
	out := make([]aigateway.TextMessage, 0, len(messages))
	for _, msg := range messages {
		out = append(out, aigateway.TextMessage{
			Role:       msg.Role,
			Content:    msg.Content,
			ToolCalls:  agentGatewayToolCalls(msg.ToolCalls),
			ToolCallID: msg.ToolCallID,
		})
	}
	return out
}

func agentGatewayTextTools(tools []ai.Tool) []aigateway.TextTool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]aigateway.TextTool, 0, len(tools))
	for _, tool := range tools {
		out = append(out, aigateway.TextTool{
			Type: tool.Type,
			Function: aigateway.TextToolFunction{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				Parameters:  tool.Function.Parameters,
			},
		})
	}
	return out
}

func cloneAgentToolCalls(calls []ai.ToolCall) []ai.ToolCall {
	if calls == nil {
		return nil
	}
	out := make([]ai.ToolCall, len(calls))
	for i, call := range calls {
		out[i] = call.Clone()
	}
	return out
}

func agentGatewayToolCalls(calls []ai.ToolCall) []aigateway.TextToolCall {
	if len(calls) == 0 {
		return nil
	}
	out := make([]aigateway.TextToolCall, 0, len(calls))
	for _, call := range calls {
		out = append(out, aigateway.TextToolCall{
			ProviderMetadata: maps.Clone(call.ProviderMetadata),
			ID:               call.ID,
			Type:             call.Type,
			Function: aigateway.TextToolCallFunction{
				Name:      call.Function.Name,
				Arguments: call.Function.Arguments,
			},
		})
	}
	return out
}

func cloneAgentGatewayOptions(options map[string]interface{}) map[string]interface{} {
	if len(options) == 0 {
		return nil
	}
	cloned := make(map[string]interface{}, len(options))
	for key, value := range options {
		cloned[key] = value
	}
	return cloned
}

// RegisterTool 全局注册工具
func (a *Agent) RegisterTool(entry ToolEntry) {
	a.tools[entry.Definition.Function.Name] = entry
}

// getTool 获取工具（先查本次注册，再查全局）
func (a *Agent) getTool(name string, reqTools map[string]ToolEntry) (ToolEntry, bool) {
	if t, ok := reqTools[name]; ok {
		return t, true
	}
	t, ok := a.tools[name]
	return t, ok
}

// emit 发送流式事件：补全信封字段（RunID/AgentID/Sequence/CreatedAt）→ 中间件链 → 回调。
func (a *Agent) emit(ctx context.Context, req *RunRequest, cb StreamCallback, event AgentEvent) error {
	if req != nil {
		if event.RunID == nil {
			event.RunID = req.RunID
		}
		if event.AgentID == nil {
			event.AgentID = req.AgentID
		}
	}
	StampEvent(&event)
	var middlewares []Middleware
	if req != nil {
		middlewares = req.Middlewares
	}
	for _, middleware := range eventMiddlewares(middlewares) {
		next, err := middleware.OnEvent(ctx, &event)
		if err != nil {
			return err
		}
		if next == nil {
			return nil
		}
		event = *next
	}
	if cb == nil {
		return nil
	}
	if err := cb(event); err != nil {
		logger.ErrorContext(ctx, "流式回调错误", "error", err)
		return err
	}
	return nil
}

func NormalizePermissionLevel(permissionLevel string, allowFullAccess bool) string {
	switch strings.TrimSpace(permissionLevel) {
	case permissionAlwaysAsk:
		return permissionAlwaysAsk
	case permissionFullAccess:
		if allowFullAccess {
			return permissionFullAccess
		}
		return ApprovalAskRisk
	case ApprovalAskRisk, "":
		return ApprovalAskRisk
	default:
		return ApprovalAskRisk
	}
}

// Run 执行 TAO 循环（支持流式推送）
func (a *Agent) Run(ctx context.Context, req *RunRequest) (*RunResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	maxTurns := req.MaxTurns
	unlimitedTurns := maxTurns <= 0
	if unlimitedTurns {
		logger.InfoContext(ctx, "Agent Run 启动: 无轮数限制模式", "unlimited_turns", true, "session_id", req.SessionID, "agent_id", req.AgentID, "model", req.Model)
	} else {
		logger.InfoContext(ctx, "Agent Run 启动: 指定最大轮数限制", "max_turns", maxTurns, "session_id", req.SessionID, "agent_id", req.AgentID, "model", req.Model)
	}

	// 构建本次工具索引
	reqTools := make(map[string]ToolEntry, len(req.Tools))
	for _, t := range req.Tools {
		reqTools[t.Definition.Function.Name] = t
	}
	allowedTools := make(map[string]struct{}, len(req.AllowedTools))
	for _, name := range req.AllowedTools {
		if name == "" {
			continue
		}
		allowedTools[name] = struct{}{}
	}
	isAllowedTool := func(name string) bool {
		_, ok := allowedTools[name]
		return ok
	}

	// 构建完整消息列表 + 工具定义
	capacity := len(req.Messages) + 32
	if !unlimitedTurns {
		capacity = len(req.Messages) + maxTurns*2
	}
	messages := make([]ai.ChatMessage, 0, capacity)
	messages = append(messages, req.Messages...)

	// 收集允许的候选工具
	allCandidateTools := make([]ToolEntry, 0, len(a.tools)+len(req.Tools))
	for _, t := range a.tools {
		if isAllowedTool(t.Definition.Function.Name) {
			allCandidateTools = append(allCandidateTools, t)
		}
	}
	for _, t := range req.Tools {
		if isAllowedTool(t.Definition.Function.Name) {
			allCandidateTools = append(allCandidateTools, t)
		}
	}

	// 提取用户最新意图并根据相关度动态预选工具，防止海量 MCP/自定义工具撑爆上下文
	var lastUserQuery string
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == ai.ChatMessageRoleUser {
			lastUserQuery = req.Messages[i].Content
			break
		}
	}
	if lastUserQuery == "" && req.SystemPrompt != "" {
		lastUserQuery = req.SystemPrompt
	}

	toolSelector := NewToolSelector()
	selectedTools := toolSelector.SelectTools(allCandidateTools, lastUserQuery)

	toolDefs := make([]ai.Tool, 0, len(selectedTools))
	for _, t := range selectedTools {
		toolDefs = append(toolDefs, t.Definition)
	}

	var totalUsage *ai.TokenUsage
	turns := 0

	cb := req.OnEvent
	capabilities := req.Capabilities
	if capabilities == nil {
		capabilities = DefaultToolCapabilities()
	}
	permissionLevel := NormalizePermissionLevel(req.PermissionLevel, req.AllowFullAccess)
	approvalGate := NewApprovalGate(capabilities, req.Approvals)
	activeModel := req.Model
	modelMW := modelMiddlewares(req.Middlewares)
	toolMW := toolMiddlewares(req.Middlewares)
	contextManager := NewContextManager(ContextManagerOptions{
		MaxContextTokens:  req.MaxContextTokens,
		ModelName:         req.Model,
		DisableCompaction: req.DisableCompaction,
		Compactor:         enginepromptctx.NewConfiguredContextCompactor(a.agentTextGateway(), req.Compaction),
	})
	loopGuard := NewLoopGuard(maxTurns, WithMaxRepeatCalls(req.RepeatCallThreshold))

	runStartedAt := timeutil.Now().UTC()
	allSpans := make([]enginedomain.TraceSpan, 0)
	var accumulatedLatency enginedomain.LatencyBreakdown
	var accumulatedCost enginedomain.CostBreakdown
	accumulatedCost.Currency = "USD"

	emit := func(event AgentEvent) error {
		return a.emit(ctx, req, cb, event)
	}

	buildRunResponse := func(content string) *RunResponse {
		runEndedAt := timeutil.Now().UTC()
		totalRunMs := runEndedAt.Sub(runStartedAt).Milliseconds()
		if accumulatedLatency.TotalMs < totalRunMs {
			accumulatedLatency.TotalMs = totalRunMs
		}
		if totalUsage == nil {
			totalUsage = estimateRunUsage(req, messages, content)
		}
		return &RunResponse{
			Content:    content,
			Messages:   messages,
			TotalTurns: turns,
			TokenUsage: totalUsage,
			Latency:    accumulatedLatency,
			Cost:       accumulatedCost,
			TraceSpans: allSpans,
		}
	}

	for unlimitedTurns || turns < maxTurns {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		turns++
		logger.InfoContext(ctx, "Agent 轮次推进", "turn", turns, "unlimited", unlimitedTurns, "max_turns", maxTurns)

		// 上下文 Token 80% 阈值交接压缩与分级折叠
		if !req.DisableCompaction {
			optResult := contextManager.OptimizeMessages(ctx, toRequestMeta(req), messages)
			messages = optResult.Messages
			if optResult.HandoffDocGenerated {
				_ = emit(AgentEvent{
					Type:    StandardEventContextUsage,
					Turn:    turns,
					Content: fmt.Sprintf("上下文已达 80%% 阈值，已压缩 %d 条历史对话并生成交接文档", optResult.CompactedCount),
					Data: map[string]interface{}{
						"compacted_messages": optResult.CompactedCount,
						"handoff_generated":  true,
						"handoff_document":   optResult.HandoffDocument,
						"estimated_tokens":   optResult.EstimatedTokens,
						"max_context_tokens": optResult.MaxTokens,
					},
				})
			} else if optResult.FoldedCount > 0 {
				_ = emit(AgentEvent{
					Type:    StandardEventContextUsage,
					Turn:    turns,
					Content: fmt.Sprintf("已折叠 %d 条历史工具结果以控制上下文", optResult.FoldedCount),
					Data: map[string]interface{}{
						"folded":             optResult.FoldedCount,
						"estimated_tokens":   optResult.EstimatedTokens,
						"max_context_tokens": optResult.MaxTokens,
					},
				})
			}
		}

		// 接近最大轮数时的收敛指引 (Graceful Convergence)
		messages = loopGuard.ApplyConvergenceHint(turns, messages)

		// 动态解析模型定价（优先使用请求指定，其次尝试 PricingResolver 查询，未命中保持客观统计）
		pricing := req.Pricing
		if pricing == nil && req.PricingResolver != nil {
			if resolved, err := req.PricingResolver.GetModelPricing(ctx, activeModel); err == nil && resolved != nil {
				pricing = resolved
			}
		}
		if pricing != nil && req.ExchangeRate > 0 && pricing.ExchangeRate <= 0 {
			pricing.ExchangeRate = req.ExchangeRate
		}

		// ===== Think 阶段 =====
		thinkRes, err := a.executeThinkTurn(ctx, req, turns, activeModel, messages, toolDefs, pricing, modelMW, emit)
		if err != nil {
			return nil, err
		}

		activeModel = thinkRes.ActiveModel
		totalUsage = ai.AddTokenUsage(totalUsage, thinkRes.TurnUsage)
		resp := thinkRes.Response

		// 累计链路 Span 与度量
		allSpans = append(allSpans, thinkRes.Span)
		accumulatedLatency.TTFTMs += thinkRes.Latency.TTFTMs
		accumulatedLatency.ThinkingMs += thinkRes.Latency.ThinkingMs
		accumulatedLatency.StreamMs += thinkRes.Latency.StreamMs
		accumulatedLatency.TotalMs += thinkRes.Latency.TotalMs

		accumulatedCost.InputTokens += thinkRes.Cost.InputTokens
		accumulatedCost.OutputTokens += thinkRes.Cost.OutputTokens
		accumulatedCost.ReasoningTokens += thinkRes.Cost.ReasoningTokens
		accumulatedCost.CachedTokens += thinkRes.Cost.CachedTokens
		accumulatedCost.InputCost += thinkRes.Cost.InputCost
		accumulatedCost.OutputCost += thinkRes.Cost.OutputCost
		accumulatedCost.TotalCostUSD += thinkRes.Cost.TotalCostUSD
		accumulatedCost.TotalCostCNY += thinkRes.Cost.TotalCostCNY

		// 构建 assistant 消息
		assistantMsg := ai.ChatMessage{
			Role:      ai.ChatMessageRoleAssistant,
			Content:   resp.Content,
			ToolCalls: cloneAgentToolCalls(resp.ToolCalls),
		}
		messages = append(messages, assistantMsg)

		if err := emit(AgentEvent{
			Type:    StandardEventTurnDone,
			Turn:    turns,
			Content: fmt.Sprintf("第 %d 轮思考完成", turns),
			Data: TurnDoneData{
				Turn:         turns,
				ToolCalls:    len(resp.ToolCalls),
				Content:      resp.Content,
				FinishReason: resp.FinishReason,
				Usage:        thinkRes.TurnUsage,
			},
		}); err != nil {
			return nil, err
		}

		if err := ctx.Err(); err != nil {
			return nil, err
		}
		// 如果没有 tool_calls → 结束
		if len(resp.ToolCalls) == 0 {
			logger.InfoContext(ctx, "无工具调用，结束", "turn", turns)
			result := buildRunResponse(resp.Content)
			if err := emit(AgentEvent{Type: StandardEventRunDone, Data: result}); err != nil {
				return nil, err
			}
			return result, nil
		}

		// ===== Act + Observe 阶段 =====
		actRes, err := a.executeActTurn(
			ctx,
			req,
			turns,
			resp.ToolCalls,
			reqTools,
			capabilities,
			permissionLevel,
			approvalGate,
			loopGuard,
			toolMW,
			isAllowedTool,
			emit,
		)
		if err != nil {
			return nil, err
		}

		messages = append(messages, actRes.ToolMessages...)
		allSpans = append(allSpans, actRes.ToolSpans...)
		for _, ts := range actRes.ToolSpans {
			accumulatedLatency.TotalMs += ts.DurationMs
		}

		if actRes.ShouldSuspend {
			logger.InfoContext(ctx, "触发挂起(ask_user或授权)，暂停执行并返回给用户", "turn", turns)
			result := buildRunResponse(resp.Content)
			if err := emit(AgentEvent{Type: StandardEventRunDone, Data: result}); err != nil {
				return nil, err
			}
			return result, nil
		}

		logger.InfoContext(ctx, "工具完成，进入下一轮", "turn", turns, "tool_call_count", len(resp.ToolCalls))
	}

	// 能走到循环末尾只可能是有限轮数达到上限；无限轮数会在循环体内返回。
	logger.WarnContext(ctx, "达到最大轮数限制，强制结束", "max_turns", maxTurns, "total_turns", turns)
	lastContent := "Agent 未产生输出"
	if len(messages) > 0 {
		lastContent = messages[len(messages)-1].Content
	}
	result := buildRunResponse(lastContent)
	if err := emit(AgentEvent{Type: StandardEventRunDone, Data: result}); err != nil {
		return nil, err
	}
	return result, nil
}

// resolveProvider 获取使用的提供商
func (a *Agent) resolveProvider(provider string) string {
	if provider != "" {
		return provider
	}
	return a.provider
}
