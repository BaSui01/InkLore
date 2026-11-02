package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"InkLore/internal/llm/domain"
	"InkLore/internal/llm/routing"
	platformerr "InkLore/internal/platform/errors"
	"InkLore/internal/platform/logger"
)

type ProviderRouter struct {
	mu        sync.RWMutex
	client    *http.Client
	apiKey    string
	baseURL   string
	provider  string
	protocol  string
	isLiveAPI bool
	// pool 为可选的多渠道选路池。
	pool   *routing.Pool
	config domain.RuntimeConfig
	// usageRecorder 为可选的调用用量审计落库（INK-018）；未注入时不记录。
	usageRecorder domain.UsageRecorder
	// metricsRecorder 为可选的进程内指标消费者（INK-056）；未注入时不记录。
	// 与审计解耦：没有数据库时指标依然可用。
	metricsRecorder domain.MetricsRecorder
	tenantResolver  TenantChannelResolver
}

func NewProviderRouter() *ProviderRouter {
	protocol, apiKey, baseURL := configuredProvider()
	provider := protocol

	isLive := apiKey != ""
	initialModel := defaultModelForProtocol(protocol, isLive)
	if envModel := strings.TrimSpace(os.Getenv("LLM_MODEL")); envModel != "" {
		initialModel = envModel
	} else if envModel := strings.TrimSpace(os.Getenv("DEFAULT_MODEL")); envModel != "" {
		initialModel = envModel
	} else if envModel := strings.TrimSpace(os.Getenv("WRITING_TOOL_MODEL")); envModel != "" {
		initialModel = envModel
	}

	if !isLive {
		logger.Info("初始化模型 Provider Router（等待管理员数据库渠道配置注入）")
	}

	return &ProviderRouter{
		client: &http.Client{
			Timeout: 90 * time.Second,
		},
		apiKey:    apiKey,
		baseURL:   strings.TrimRight(baseURL, "/"),
		provider:  provider,
		protocol:  protocol,
		isLiveAPI: isLive,
		config: domain.RuntimeConfig{
			ActiveModel:        initialModel,
			DefaultTemperature: 0.7,
			BaseURL:            baseURL,
			ProviderType:       provider,
			StreamEnabled:      true,
		},
	}
}

func (r *ProviderRouter) GetRuntimeConfig(ctx context.Context) domain.RuntimeConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config
}

func (r *ProviderRouter) UpdateRuntimeConfig(ctx context.Context, cfg domain.RuntimeConfig) domain.RuntimeConfig {
	r.mu.Lock()
	defer r.mu.Unlock()
	if cfg.ActiveModel != "" {
		r.config.ActiveModel = cfg.ActiveModel
	}
	if cfg.DefaultTemperature > 0 {
		r.config.DefaultTemperature = cfg.DefaultTemperature
	}
	if cfg.BaseURL != "" {
		r.config.BaseURL = cfg.BaseURL
		r.baseURL = strings.TrimRight(cfg.BaseURL, "/")
	}
	if cfg.ProviderType != "" {
		r.config.ProviderType = cfg.ProviderType
		r.protocol = normalizeProtocol(cfg.ProviderType)
	}
	r.config.StreamEnabled = cfg.StreamEnabled
	return r.config
}

func (r *ProviderRouter) GetProviderInfo(ctx context.Context) domain.ProviderInfo {
	if r.isLiveAPI {
		return domain.ProviderInfo{
			Name:         r.provider,
			Type:         r.protocol,
			IsLiveAPI:    true,
			DefaultModel: r.config.ActiveModel,
			Capabilities: []string{"text", "stream", "structured_output"},
		}
	}
	return domain.ProviderInfo{
		Name:         "未配置模型 Provider",
		Type:         "none",
		IsLiveAPI:    false,
		DefaultModel: "",
		Capabilities: []string{},
	}
}

// SetKeyLimiters 覆盖密钥级 RPM 限流与并发槽实现（组合根调用）。
//
// 默认是进程内实现：单实例正确，多实例部署下每个进程各算一份配额（INK-058）。
// 因此在 Redis 可用时由组合根注入跨进程实现，让 `channel_keys.rate_limit` 与
// `concurrency_limit` 成为全局约束；Redis 不可用时保持内存实现，不阻断启动。
func (r *ProviderRouter) SetKeyLimiters(rateLimiter routing.KeyRateLimiter, concurrencyLimiter routing.KeyConcurrencyLimiter) {
	if r == nil || r.pool == nil {
		return
	}
	if rateLimiter != nil {
		r.pool.SetKeyRateLimiter(rateLimiter)
	}
	if concurrencyLimiter != nil {
		r.pool.SetKeyConcurrencyLimiter(concurrencyLimiter)
	}
}

// BreakerSnapshot 转发渠道池的密钥级熔断快照；未启用多渠道池时返回 nil。
//
// 供 /metrics 的拉取式采集使用（INK-056）：返回的是渠道名与计数，不含密钥内容。
func (r *ProviderRouter) BreakerSnapshot() []routing.ChannelBreakerSnapshot {
	if r == nil || r.pool == nil {
		return nil
	}
	return r.pool.BreakerSnapshot()
}

func (r *ProviderRouter) Generate(ctx context.Context, req domain.GenerateRequest) (domain.GenerateResult, error) {
	if !r.isLiveAPI {
		return domain.GenerateResult{}, domain.ErrProviderNotConfigured
	}

	return r.generateUpstream(ctx, req)
}

func (r *ProviderRouter) GenerateStream(ctx context.Context, req domain.GenerateRequest, onChunk domain.StreamChunkHandler) error {
	chatReq := domain.TextGenerationRequest{
		Model:        req.Model,
		SystemPrompt: req.SystemPrompt,
		Messages:     toChatMessages(req.Messages),
		Temperature:  req.Temperature,
		MaxTokens:    req.MaxTokens,
		RequestID:    req.RequestID,
		TaskKey:      req.TaskKey,
		OnTextChunk:  onChunk,
	}
	return r.GenerateChatStream(ctx, chatReq)
}

func (r *ProviderRouter) GenerateEventsStream(ctx context.Context, req domain.GenerateRequest, onEvent domain.StreamEventHandler) error {
	var doneOnce sync.Once
	emitDone := func(usage *domain.GenerationUsage) {
		doneOnce.Do(func() {
			_ = onEvent(domain.StreamEvent{
				Type:  domain.StreamEventDone,
				Usage: usage,
			})
		})
	}

	chatReq := domain.TextGenerationRequest{
		Model:        req.Model,
		SystemPrompt: req.SystemPrompt,
		Messages:     toChatMessages(req.Messages),
		Temperature:  req.Temperature,
		MaxTokens:    req.MaxTokens,
		RequestID:    req.RequestID,
		TaskKey:      req.TaskKey,
		OnReasoningChunk: func(chunk string) error {
			return onEvent(domain.StreamEvent{
				Type:  domain.StreamEventThink,
				Delta: chunk,
			})
		},
		OnTextChunk: func(chunk string) error {
			return onEvent(domain.StreamEvent{
				Type:  domain.StreamEventMessage,
				Delta: chunk,
			})
		},
		OnToolCalls: func(calls []domain.ToolCall) {
			for _, call := range calls {
				_ = onEvent(domain.StreamEvent{
					Type:       domain.StreamEventToolCall,
					ToolCallID: call.ID,
					ToolName:   call.Function.Name,
					ToolArgs:   call.Function.Arguments,
				})
			}
		},
		OnUsage: func(u *domain.TokenUsage) {
			if u != nil {
				emitDone(&domain.GenerationUsage{
					InputTokens:  u.PromptTokens,
					OutputTokens: u.CompletionTokens,
					TotalTokens:  u.TotalTokens,
				})
			}
		},
	}

	err := r.GenerateChatStream(ctx, chatReq)
	emitDone(nil)
	return err
}

func toChatMessages(messages []domain.Message) []domain.ChatMessage {
	res := make([]domain.ChatMessage, 0, len(messages))
	for _, m := range messages {
		res = append(res, domain.ChatMessage{Role: m.Role, Content: m.Content})
	}
	return res
}

func (r *ProviderRouter) GenerateChatStream(ctx context.Context, req domain.TextGenerationRequest) (err error) {
	if !r.isLiveAPI {
		return domain.ErrProviderNotConfigured
	}

	target, model, targetErr := r.resolveChatTarget(ctx, req)
	if targetErr != nil {
		return targetErr
	}
	if nativeProtocol(target.protocol) {
		return r.generateNativeChatStream(ctx, target, req, model)
	}

	startedAt := time.Now()
	var finalUsage *domain.TokenUsage
	defer func() {
		r.recordUpstream(target, err)
		inputTokens, outputTokens := 0, 0
		if finalUsage != nil {
			inputTokens = finalUsage.PromptTokens
			outputTokens = finalUsage.CompletionTokens
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
	maxTokens := req.MaxTokens
	maxCompletionTokens := req.MaxTokens
	if reasoning {
		temp = 0
		maxTokens = 0
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
		Stream:              true,
		StreamOptions:       &streamOptionsPayload{IncludeUsage: true},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return platformerr.Internal("llm_marshal_failed", "llm.generation_failed", err.Error())
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", target.baseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return platformerr.Internal("llm_request_failed", "llm.generation_failed", err.Error())
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+target.apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := r.client.Do(httpReq)
	if err != nil {
		return platformerr.Internal("llm_call_failed", "llm.generation_failed", err.Error())
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return platformerr.Wrap(parseUpstreamHTTPError(resp), http.StatusInternalServerError, "upstream_error", "llm.generation_failed", resp.StatusCode)
	}

	finalUsage, err = decodeOpenAIChatStream(ctx, resp.Body, req)
	return err
}

// --- Live Upstream (OpenAI / DeepSeek compatible) ---

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	// MaxTokens 来自任务级模型策略（INK-066）：0 时 omitempty 会整段省略，
	// 上游按自己的默认上限处理。
	MaxTokens int  `json:"max_tokens,omitempty"`
	Stream    bool `json:"stream"`
}

type openAIChatResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

func (r *ProviderRouter) buildMessages(req domain.GenerateRequest) []openAIMessage {
	messages := make([]openAIMessage, 0, len(req.Messages)+1)
	if req.SystemPrompt != "" {
		messages = append(messages, openAIMessage{Role: "system", Content: req.SystemPrompt})
	}
	for _, m := range req.Messages {
		messages = append(messages, openAIMessage{Role: m.Role, Content: m.Content})
	}
	return messages
}

func (r *ProviderRouter) generateUpstream(ctx context.Context, req domain.GenerateRequest) (result domain.GenerateResult, err error) {
	if nativeProtocol(r.protocol) {
		return r.generateNative(ctx, req)
	}
	target, targetErr := r.resolveUpstream(ctx)
	if targetErr != nil {
		return domain.GenerateResult{}, targetErr
	}
	model := req.Model
	if model == "" || strings.HasPrefix(model, "novel-") {
		model = r.config.ActiveModel
	}
	if model == "" {
		model = defaultModelForProtocol(r.protocol, true)
	}

	payload := openAIChatRequest{
		Model:       model,
		Messages:    r.buildMessages(req),
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      false,
	}

	startedAt := time.Now()
	defer func() {
		r.recordUpstream(target, err)
		inputTokens, outputTokens := 0, 0
		if err == nil {
			inputTokens = result.Usage.InputTokens
			outputTokens = result.Usage.OutputTokens
		}
		r.recordUsage(ctx, req.RequestID, req.TaskKey, model, target, startedAt, inputTokens, outputTokens, err)
	}()

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return domain.GenerateResult{}, platformerr.Internal("llm_marshal_failed", "llm.generation_failed", err.Error())
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", target.baseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return domain.GenerateResult{}, platformerr.Internal("llm_request_failed", "llm.generation_failed", err.Error())
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+target.apiKey)

	resp, err := r.client.Do(httpReq)
	if err != nil {
		return domain.GenerateResult{}, platformerr.Internal("llm_call_failed", "llm.generation_failed", err.Error())
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return domain.GenerateResult{}, platformerr.Wrap(parseUpstreamHTTPError(resp), http.StatusInternalServerError, "upstream_error", "llm.generation_failed", resp.StatusCode)
	}

	var chatResp openAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return domain.GenerateResult{}, platformerr.Internal("llm_decode_failed", "llm.generation_failed", err.Error())
	}

	content := ""
	if len(chatResp.Choices) > 0 {
		content = chatResp.Choices[0].Message.Content
	}

	return domain.GenerateResult{
		Content:           content,
		Model:             model,
		UpstreamRequestID: chatResp.ID,
		Usage: domain.GenerationUsage{
			InputTokens:  chatResp.Usage.PromptTokens,
			OutputTokens: chatResp.Usage.CompletionTokens,
			TotalTokens:  chatResp.Usage.TotalTokens,
		},
	}, nil
}

// upstreamStatusError 携带上游 HTTP 状态码。
//
// 它同时满足 llm/domain/gateway 的 httpStatusCarrier 与 llm/ports.HTTPStatusError，
// 是 ClassifyExecutionError 能把 5xx 归类为"可重试/上游不可用"的前提，
// 也是密钥级熔断能够累加计数的前提。
func newUpstreamStatusError(status int, reason string) error {
	return &upstreamStatusError{status: status, reason: strings.TrimSpace(reason)}
}

type upstreamStatusError struct {
	reason string
	status int
}

func (e *upstreamStatusError) Error() string {
	return fmt.Sprintf("上游返回 %d %s", e.status, e.reason)
}

// ProviderHTTPStatus 返回上游 HTTP 状态码。
func (e *upstreamStatusError) ProviderHTTPStatus() int {
	return e.status
}

// upstreamTarget 描述一次上游调用最终落到的渠道与密钥。
//
// managed 表示该目标来自 routing.Pool 的选路结果：只有 managed 目标才需要
// 在调用结束后回报结果（驱动密钥级熔断与统计），直连模式无需回报。
type upstreamTarget struct {
	baseURL   string
	apiKey    string
	channel   string
	channelID int64
	leaseID   int64
	managed   bool
	protocol  string
}

// resolveUpstream is the legacy unscoped pool/environment route. Tenant-bound
// requests use resolveChatTarget and must never reach this fallback.

// SetPool 注入可选的多渠道选路池。
func (r *ProviderRouter) SetPool(pool *routing.Pool) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pool = pool
	if pool != nil {
		r.isLiveAPI = true
	}
}

// SetTenantResolver selects a channel using an authenticated tenant and exact model ID.
func (r *ProviderRouter) SetTenantResolver(resolver TenantChannelResolver) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tenantResolver = resolver
	if resolver != nil {
		r.isLiveAPI = true
	}
}

// resolveChatTarget never falls back to global credentials for tenant-bound work.
func (r *ProviderRouter) resolveChatTarget(ctx context.Context, req domain.TextGenerationRequest) (upstreamTarget, string, error) {
	if req.TenantID != "" || req.ModelID != "" || req.PolicyVersionID != "" {
		if strings.TrimSpace(req.TenantID) == "" || strings.TrimSpace(req.ModelID) == "" ||
			strings.TrimSpace(req.PolicyVersionID) == "" || r.tenantResolver == nil {
			return upstreamTarget{}, "", fmt.Errorf("tenant model route requires tenant, model and frozen policy")
		}
		resolved, err := r.tenantResolver(ctx, req.TenantID, req.ModelID)
		if err != nil {
			return upstreamTarget{}, "", err
		}
		if resolved.BaseURL == "" || resolved.APIKey == "" || resolved.UpstreamModel == "" || resolved.Protocol == "" {
			return upstreamTarget{}, "", fmt.Errorf("tenant model route unavailable")
		}
		return upstreamTarget{
			baseURL: resolved.BaseURL, apiKey: resolved.APIKey,
			protocol: resolved.Protocol, leaseID: -1,
		}, resolved.UpstreamModel, nil
	}
	target, err := r.resolveUpstream(ctx, req.Model)
	if err != nil {
		return upstreamTarget{}, "", err
	}
	model := req.Model
	if model == "" || strings.HasPrefix(model, "novel-") {
		model = r.config.ActiveModel
	}
	if model == "" {
		model = defaultModelForProtocol(target.protocol, true)
	}
	return target, model, nil
}

func (r *ProviderRouter) resolveUpstream(ctx context.Context, requestedModel ...string) (upstreamTarget, error) {
	if r == nil {
		return upstreamTarget{}, fmt.Errorf("provider router is nil")
	}
	selector := r.provider
	if len(requestedModel) > 0 && strings.TrimSpace(requestedModel[0]) != "" {
		selector = strings.TrimSpace(requestedModel[0])
	} else {
		r.mu.RLock()
		if activeModel := strings.TrimSpace(r.config.ActiveModel); activeModel != "" {
			selector = activeModel
		}
		r.mu.RUnlock()
	}
	// 非租户请求只能使用明确配置的进程级渠道池或环境变量。
	if r.pool != nil {
		channel, decryptedKey, leaseID, err := r.pool.GetChannelKeyContext(ctx, selector)
		if err == nil {
			baseURL := r.baseURL
			channelName := ""
			var channelID int64
			protocol := ""
			if channel != nil {
				channelName = channel.Name
				channelID = channel.ID
				if trimmed := strings.TrimRight(strings.TrimSpace(channel.BaseURL), "/"); trimmed != "" {
					baseURL = trimmed
				}
				if selectedProtocol := strings.TrimSpace(channel.Protocol); selectedProtocol != "" {
					protocol = normalizeProtocol(selectedProtocol)
				} else {
					protocol = normalizeProtocol(channel.Provider)
				}
			}
			return upstreamTarget{
				baseURL:   baseURL,
				apiKey:    decryptedKey,
				channel:   channelName,
				channelID: channelID,
				leaseID:   leaseID,
				managed:   true,
				protocol:  protocol,
			}, nil
		}
	}
	if r.apiKey != "" {
		return upstreamTarget{baseURL: r.baseURL, apiKey: r.apiKey, leaseID: -1, protocol: r.protocol}, nil
	}
	return upstreamTarget{}, domain.ErrProviderNotConfigured
}

// recordUpstream 回报调用结果；未走选路的直连目标无需回报。
func (r *ProviderRouter) recordUpstream(target upstreamTarget, err error) {
	if r == nil || r.pool == nil || !target.managed {
		return
	}
	r.pool.RecordKeyCall(target.leaseID, err == nil, err)
}

// recordUsage 记录一次上游调用的用量（尽力而为）。
//
// 之所以放在上游调用路径而不是网关层：只有这里才知道真正落到了哪个渠道、
// 耗时多少、以及 token 的真实来源。非流式、流式、工具调用三条路径共用本方法。
func (r *ProviderRouter) recordUsage(
	ctx context.Context,
	requestID string,
	taskKey string,
	model string,
	target upstreamTarget,
	startedAt time.Time,
	inputTokens int,
	outputTokens int,
	callErr error,
) {
	if r == nil {
		return
	}
	status := domain.UsageStatusSucceeded
	errorText := ""
	if callErr != nil {
		status = domain.UsageStatusFailed
		errorText = callErr.Error()
	}
	latency := time.Since(startedAt).Milliseconds()
	if latency < 0 {
		latency = 0
	}
	// 进程内指标（INK-056）：与审计落库解耦，无数据库时同样可用。
	if r.metricsRecorder != nil {
		r.metricsRecorder.RecordCall(domain.CallMetric{
			Model:        strings.TrimSpace(model),
			Channel:      target.channel,
			ChannelID:    target.channelID,
			Status:       status,
			LatencyMS:    latency,
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
		})
	}
	if r.usageRecorder == nil {
		return
	}
	r.usageRecorder.Record(ctx, domain.UsageRecord{
		RequestID:    strings.TrimSpace(requestID),
		TaskKey:      strings.TrimSpace(taskKey),
		Model:        strings.TrimSpace(model),
		ChannelID:    target.channelID,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		LatencyMS:    latency,
		Status:       status,
		ErrorText:    errorText,
	})
}

// SetMetricsRecorder 注入进程内指标消费者（组合根调用）。
//
// 与 SetUsageRecorder 同样采用 setter：指标是可观测性能力，不是模型调用的必需依赖。
func (r *ProviderRouter) SetMetricsRecorder(recorder domain.MetricsRecorder) {
	if r == nil {
		return
	}
	r.metricsRecorder = recorder
}

// SetUsageRecorder 注入用量记录器（组合根调用）。
//
// 采用 setter 而不是构造参数：ProviderRouter 在无数据库的开发环境仍需可用，
// 审计是可选能力而非必需依赖。
func (r *ProviderRouter) SetUsageRecorder(recorder domain.UsageRecorder) {
	if r == nil {
		return
	}
	r.usageRecorder = recorder
}

// openAIEmbeddingRequest / Response 是 OpenAI 兼容的嵌入协议形状。
type openAIEmbeddingRequest struct {
	Model      string   `json:"model"`
	Input      []string `json:"input"`
	Dimensions int      `json:"dimensions,omitempty"`
}

type openAIEmbeddingResponse struct {
	Model string `json:"model"`
	Data  []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

// Embeddings 调用 OpenAI 兼容的 /embeddings 端点（INK-017）。
//
// 与生成路径共用同一套渠道解析与鉴权：未配置 Provider 时明确报错，绝不返回伪造向量。
func (r *ProviderRouter) Embeddings(ctx context.Context, req domain.EmbeddingRequest) (domain.EmbeddingResult, error) {
	if r == nil || !r.isLiveAPI {
		return domain.EmbeddingResult{}, domain.ErrProviderNotConfigured
	}
	inputs := make([]string, 0, len(req.Inputs))
	for _, input := range req.Inputs {
		trimmed := strings.TrimSpace(input)
		if trimmed == "" {
			continue
		}
		inputs = append(inputs, trimmed)
	}
	if len(inputs) == 0 {
		return domain.EmbeddingResult{Model: req.Model}, nil
	}

	target, targetErr := r.resolveUpstream(ctx)
	if targetErr != nil {
		return domain.EmbeddingResult{}, targetErr
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = defaultEmbeddingModel
	}

	bodyBytes, err := json.Marshal(openAIEmbeddingRequest{Model: model, Input: inputs, Dimensions: req.Dimensions})
	if err != nil {
		return domain.EmbeddingResult{}, platformerr.Internal("llm_marshal_failed", "llm.generation_failed", err.Error())
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, target.baseURL+"/embeddings", bytes.NewReader(bodyBytes))
	if err != nil {
		return domain.EmbeddingResult{}, platformerr.Internal("llm_request_failed", "llm.generation_failed", err.Error())
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+target.apiKey)

	resp, err := r.client.Do(httpReq)
	if err != nil {
		r.recordUpstream(target, err)
		return domain.EmbeddingResult{}, platformerr.Internal("llm_call_failed", "llm.generation_failed", err.Error())
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		upstreamErr := newUpstreamStatusError(resp.StatusCode, http.StatusText(resp.StatusCode))
		r.recordUpstream(target, upstreamErr)
		return domain.EmbeddingResult{}, platformerr.Wrap(upstreamErr, http.StatusInternalServerError, "upstream_error", "llm.generation_failed", resp.StatusCode)
	}
	var decoded openAIEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		r.recordUpstream(target, err)
		return domain.EmbeddingResult{}, platformerr.Internal("llm_decode_failed", "llm.generation_failed", err.Error())
	}
	r.recordUpstream(target, nil)

	vectors := make([][]float32, len(inputs))
	for _, item := range decoded.Data {
		if item.Index < 0 || item.Index >= len(vectors) {
			continue
		}
		vectors[item.Index] = item.Embedding
	}
	for index, vector := range vectors {
		if len(vector) == 0 {
			return domain.EmbeddingResult{}, fmt.Errorf("嵌入结果缺少第 %d 条向量", index)
		}
		// 维度不符必须显式报错：静默接受会让 pgvector 在写入时抛出难以定位的错误，
		// 而截断/补零则会产生错误的相似度。
		if req.Dimensions > 0 && len(vector) != req.Dimensions {
			return domain.EmbeddingResult{}, fmt.Errorf("嵌入维度不符: 期望 %d，实际 %d（第 %d 条）", req.Dimensions, len(vector), index)
		}
	}
	modelName := strings.TrimSpace(decoded.Model)
	if modelName == "" {
		modelName = model
	}
	return domain.EmbeddingResult{Model: modelName, Vectors: vectors}, nil
}

// defaultEmbeddingModel 未显式指定嵌入模型时的默认值。
const defaultEmbeddingModel = "text-embedding-3-small"
