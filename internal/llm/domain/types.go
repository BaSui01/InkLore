package domain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
)

var ErrProviderNotConfigured = errors.New("llm provider is not configured")

type Capability string

const (
	CapabilityText             Capability = "text"
	CapabilityVision           Capability = "vision"
	CapabilityToolCalling      Capability = "tool_calling"
	CapabilityStructuredOutput Capability = "structured_output"
)

type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

type ToolCall struct {
	ID               string            `json:"id"`
	Type             string            `json:"type"`
	Function         ToolCallFunction  `json:"function"`
	ProviderMetadata map[string]string `json:"provider_metadata,omitempty"`
}

func (tc ToolCall) Clone() ToolCall {
	res := tc
	if tc.ProviderMetadata != nil {
		res.ProviderMetadata = make(map[string]string, len(tc.ProviderMetadata))
		for k, v := range tc.ProviderMetadata {
			res.ProviderMetadata[k] = v
		}
	}
	return res
}

type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type TokenUsage struct {
	PromptTokens          int  `json:"prompt_tokens"`
	CompletionTokens      int  `json:"completion_tokens"`
	TotalTokens           int  `json:"total_tokens"`
	ReasoningTokens       int  `json:"reasoning_tokens,omitempty"`
	CachedTokens          int  `json:"cached_tokens,omitempty"`
	PromptCacheHitTokens  int  `json:"prompt_cache_hit_tokens,omitempty"`
	PromptCacheMissTokens int  `json:"prompt_cache_miss_tokens,omitempty"`
	InputTokens           int  `json:"input_tokens,omitempty"`
	OutputTokens          int  `json:"output_tokens,omitempty"`
	Estimated             bool `json:"estimated,omitempty"`
}

type ChatMessage struct {
	Role             string     `json:"role"`
	Content          string     `json:"content"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string     `json:"tool_call_id,omitempty"`
	Name             string     `json:"name,omitempty"`
}

const (
	ChatMessageRoleSystem    = "system"
	ChatMessageRoleUser      = "user"
	ChatMessageRoleAssistant = "assistant"
	ChatMessageRoleTool      = "tool"
)

func EstimateTextTokens(text, model string) int {
	return int(float64(len([]rune(text))) * 0.75)
}

type TextGenerationRequest struct {
	Model             string                   `json:"model"`
	SystemPrompt      string                   `json:"system_prompt,omitempty"`
	Messages          []ChatMessage            `json:"messages"`
	Tools             []Tool                   `json:"tools,omitempty"`
	ToolChoice        any                      `json:"tool_choice,omitempty"`
	ResponseFormat    *ResponseFormat          `json:"response_format,omitempty"`
	ReasoningEffort   string                   `json:"reasoning_effort,omitempty"`
	ThinkingEnabled   *bool                    `json:"thinking_enabled,omitempty"`
	ServiceTier       string                   `json:"service_tier,omitempty"`
	FastMode          bool                     `json:"fast_mode,omitempty"`
	Stop              []string                 `json:"stop,omitempty"`
	Temperature       float64                  `json:"temperature,omitempty"`
	TopP              float64                  `json:"top_p,omitempty"`
	PresencePenalty   float64                  `json:"presence_penalty,omitempty"`
	FrequencyPenalty  float64                  `json:"frequency_penalty,omitempty"`
	Seed              *int                     `json:"seed,omitempty"`
	ParallelToolCalls *bool                    `json:"parallel_tool_calls,omitempty"`
	MaxTokens         int                      `json:"max_tokens,omitempty"`
	Options           map[string]interface{}   `json:"options,omitempty"`
	TaskKey           string                   `json:"task_key,omitempty"`
	TenantID          string                   `json:"-"`
	PolicyVersionID   string                   `json:"-"`
	ModelID           string                   `json:"-"`
	ExecutionKey      string                   `json:"-"`
	Dispatch          *GatewayDispatch         `json:"-"`
	RequestID         string                   `json:"request_id,omitempty"`
	OnReasoningChunk  func(chunk string) error `json:"-"`
	OnTextChunk       func(chunk string) error `json:"-"`
	OnToolCalls       func(calls []ToolCall)   `json:"-"`
	OnUsage           func(usage *TokenUsage)  `json:"-"`
	OnReceipt         func(receipt *Usage)     `json:"-"`
}

type TextGenerationResponse struct {
	Content      string      `json:"content"`
	Thinking     string      `json:"thinking,omitempty"`
	ToolCalls    []ToolCall  `json:"tool_calls,omitempty"`
	Model        string      `json:"model"`
	FinishReason string      `json:"finish_reason"`
	Usage        *TokenUsage `json:"usage,omitempty"`
	Receipt      *Usage      `json:"-"`
}

func CloneTokenUsage(u *TokenUsage) *TokenUsage {
	if u == nil {
		return nil
	}
	res := *u
	return &res
}

func AddTokenUsage(a, b *TokenUsage) *TokenUsage {
	if a == nil && b == nil {
		return nil
	}
	if a == nil {
		return CloneTokenUsage(b)
	}
	if b == nil {
		return CloneTokenUsage(a)
	}
	return &TokenUsage{
		PromptTokens:          a.PromptTokens + b.PromptTokens,
		CompletionTokens:      a.CompletionTokens + b.CompletionTokens,
		TotalTokens:           a.TotalTokens + b.TotalTokens,
		ReasoningTokens:       a.ReasoningTokens + b.ReasoningTokens,
		CachedTokens:          a.CachedTokens + b.CachedTokens,
		PromptCacheHitTokens:  a.PromptCacheHitTokens + b.PromptCacheHitTokens,
		PromptCacheMissTokens: a.PromptCacheMissTokens + b.PromptCacheMissTokens,
		InputTokens:           a.InputTokens + b.InputTokens,
		OutputTokens:          a.OutputTokens + b.OutputTokens,
		Estimated:             a.Estimated || b.Estimated,
	}
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GenerateRequest struct {
	Model        string    `json:"model"`
	SystemPrompt string    `json:"system_prompt"`
	Messages     []Message `json:"messages"`
	Temperature  float64   `json:"temperature"`
	RequestID    string    `json:"request_id"`
	TaskKey      string    `json:"task_key,omitempty"`
	MaxTokens    int       `json:"max_tokens,omitempty"`
}

type GenerationUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
	CachedTokens int `json:"cached_tokens,omitempty"`
}

type GenerateResult struct {
	Content           string          `json:"content"`
	Model             string          `json:"model"`
	UpstreamRequestID string          `json:"upstream_request_id,omitempty"`
	Usage             GenerationUsage `json:"usage"`
}

type StreamChunkHandler func(chunk string) error

type StreamEventType string

const (
	StreamEventThink      StreamEventType = "think"
	StreamEventMessage    StreamEventType = "message"
	StreamEventToolCall   StreamEventType = "tool_call"
	StreamEventToolResult StreamEventType = "tool_result"
	StreamEventDone       StreamEventType = "done"
	StreamEventUsage      StreamEventType = "usage"
	StreamEventError      StreamEventType = "error"
)

type StreamEvent struct {
	Type         StreamEventType  `json:"type"`
	Delta        string           `json:"delta,omitempty"`
	Reasoning    string           `json:"reasoning,omitempty"`
	ToolCallID   string           `json:"tool_call_id,omitempty"`
	ToolName     string           `json:"tool_name,omitempty"`
	ToolArgs     string           `json:"tool_args,omitempty"`
	ToolResult   string           `json:"tool_result,omitempty"`
	Status       string           `json:"status,omitempty"`
	Usage        *GenerationUsage `json:"usage,omitempty"`
	ErrorMessage string           `json:"error,omitempty"`
}

type StreamEventHandler func(event StreamEvent) error

type RuntimeConfig struct {
	ActiveModel        string  `json:"active_model"`
	DefaultTemperature float64 `json:"default_temperature"`
	BaseURL            string  `json:"base_url"`
	ProviderType       string  `json:"provider_type"`
	StreamEnabled      bool    `json:"stream_enabled"`
}

type Gateway interface {
	Generate(ctx context.Context, req GenerateRequest) (GenerateResult, error)
	GenerateStream(ctx context.Context, req GenerateRequest, onChunk StreamChunkHandler) error
	GenerateEventsStream(ctx context.Context, req GenerateRequest, onEvent StreamEventHandler) error
	GenerateChat(ctx context.Context, req TextGenerationRequest) (TextGenerationResponse, error)
	GenerateChatStream(ctx context.Context, req TextGenerationRequest) error
	GetProviderInfo(ctx context.Context) ProviderInfo
	GetRuntimeConfig(ctx context.Context) RuntimeConfig
	UpdateRuntimeConfig(ctx context.Context, cfg RuntimeConfig) RuntimeConfig
}

type ProviderInfo struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	IsLiveAPI    bool     `json:"is_live_api"`
	DefaultModel string   `json:"default_model"`
	Capabilities []string `json:"capabilities"`
}

// EmbeddingRequest 是一次批量向量化请求。
type EmbeddingRequest struct {
	Model      string
	Inputs     []string
	Dimensions int
}

// EmbeddingResult 是批量向量化结果，维度与输入一一对应。
type EmbeddingResult struct {
	Model   string
	Vectors [][]float32
}

// EmbeddingGateway 是可选的向量化能力。
type EmbeddingGateway interface {
	Embeddings(ctx context.Context, req EmbeddingRequest) (EmbeddingResult, error)
}

// ErrEmbeddingsUnavailable 当前网关不提供向量化能力。
var ErrEmbeddingsUnavailable = errors.New("embeddings are not available on the current gateway")

// ImageGenerationRequest 定义文生图与图生图通用请求契约。
type ImageGenerationRequest struct {
	Model             string   `json:"model,omitempty"`
	Prompt            string   `json:"prompt"`
	NegativePrompt    string   `json:"negative_prompt,omitempty"`
	ReferenceImages   []string `json:"reference_images,omitempty"`
	ReferenceStrength *float64 `json:"reference_strength,omitempty"`
	Mode              string   `json:"mode,omitempty"`
	Size              string   `json:"size,omitempty"`
	AspectRatio       string   `json:"aspect_ratio,omitempty"`
	Quality           string   `json:"quality,omitempty"`
	Style             string   `json:"style,omitempty"`
	N                 int      `json:"n,omitempty"`
	OutputFormat      string   `json:"output_format,omitempty"`
	RequestID         string   `json:"request_id,omitempty"`
	TaskKey           string   `json:"task_key,omitempty"`
}

// GeneratedImage 单张生成图片的产物。
type GeneratedImage struct {
	B64JSON       string `json:"b64_json,omitempty"`
	URL           string `json:"url,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
	MimeType      string `json:"mime_type,omitempty"`
}

// ImageGenerationResponse 图片生成响应实体。
type ImageGenerationResponse struct {
	Images []GeneratedImage `json:"images"`
	Usage  *TokenUsage      `json:"usage,omitempty"`
}

// ImageGateway 是生图（文生图与图生图）的能力接口。
type ImageGateway interface {
	GenerateImage(ctx context.Context, req ImageGenerationRequest) (ImageGenerationResponse, error)
}

// CallMetric 是一次上游调用的指标样本（模型 / 渠道维度）。
type CallMetric struct {
	Model        string
	Channel      string
	ChannelID    int64
	Status       string
	LatencyMS    int64
	InputTokens  int
	OutputTokens int
}

// MetricsRecorder 消费模型调用指标。
type MetricsRecorder interface {
	RecordCall(metric CallMetric)
}

// 用量审计的状态取值。
const (
	UsageStatusSucceeded = "succeeded"
	UsageStatusFailed    = "failed"
)

// UsageRecord 是一次模型调用的审计记录。
type UsageRecord struct {
	RequestID    string
	TaskKey      string
	Model        string
	ErrorText    string
	Status       string
	InputTokens  int
	OutputTokens int
	ChannelID    int64
	LatencyMS    int64
}

// UsageRecorder 记录模型调用用量。
type UsageRecorder interface {
	Record(ctx context.Context, record UsageRecord)
}

// ThinkStreamFilter 思考链流式分流状态机
// 统一处理 DeepSeek 显式 reasoning_content 与开源大模型正文内嵌 <think>...</think> 标签
type ThinkStreamFilter struct {
	mu         sync.Mutex
	inThinkTag bool
	buffer     string
}

func NewThinkStreamFilter() *ThinkStreamFilter {
	return &ThinkStreamFilter{}
}

// ProcessChunk 处理单个 chunk，返回提取出的思考增量和正文增量
func (f *ThinkStreamFilter) ProcessChunk(reasoningDelta, contentDelta string) (thinkDelta, finalContentDelta string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// 1. 如果上游直接返回了结构化 reasoning_content（如 DeepSeek-R1 API、Claude thinking block）
	if reasoningDelta != "" {
		return reasoningDelta, contentDelta
	}

	if contentDelta == "" {
		return "", ""
	}

	// 2. 处理可能跨 chunk 的 <think> 标签内嵌情况
	text := f.buffer + contentDelta
	f.buffer = ""

	// 如果当前正在 <think> 内部
	if f.inThinkTag {
		if idx := strings.Index(text, "</think>"); idx != -1 {
			// 闭合标签
			thinkPart := text[:idx]
			contentPart := text[idx+len("</think>"):]
			f.inThinkTag = false
			return thinkPart, contentPart
		}
		// 仍然在 think 内部，但防止截断 "</think>" 前缀
		if strings.HasSuffix(text, "<") || strings.HasSuffix(text, "</") || strings.HasSuffix(text, "</t") || strings.HasSuffix(text, "</th") || strings.HasSuffix(text, "</thi") || strings.HasSuffix(text, "</thin") || strings.HasSuffix(text, "</think") {
			lastLt := strings.LastIndex(text, "<")
			f.buffer = text[lastLt:]
			return text[:lastLt], ""
		}
		return text, ""
	}

	// 当前不在 think 内部，检测是否有开始标签 <think>
	if idx := strings.Index(text, "<think>"); idx != -1 {
		beforeThink := text[:idx]
		afterThink := text[idx+len("<think>"):]
		f.inThinkTag = true

		// 检查是否在同一个 chunk 内闭合
		if endIdx := strings.Index(afterThink, "</think>"); endIdx != -1 {
			thinkPart := afterThink[:endIdx]
			contentPart := afterThink[endIdx+len("</think>"):]
			f.inThinkTag = false
			return thinkPart, beforeThink + contentPart
		}

		return afterThink, beforeThink
	}

	// 防止截断 "<think>" 前缀
	if strings.HasSuffix(text, "<") || strings.HasSuffix(text, "<t") || strings.HasSuffix(text, "<th") || strings.HasSuffix(text, "<thi") || strings.HasSuffix(text, "<thin") || strings.HasSuffix(text, "<think") {
		lastLt := strings.LastIndex(text, "<")
		f.buffer = text[lastLt:]
		return "", text[:lastLt]
	}

	return "", text
}

// Flush 清空缓冲区残留
func (f *ThinkStreamFilter) Flush() (thinkDelta, contentDelta string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	remaining := f.buffer
	f.buffer = ""
	if f.inThinkTag {
		return remaining, ""
	}
	return "", remaining
}

// HTTPStatusError 携带上游 HTTP 状态码的最小契约。
type HTTPStatusError interface {
	error
	ProviderHTTPStatus() int
}

// StreamHandler 流式回调函数签名。
type StreamHandler func(chunk string) error

// 结构化输出（INK-065）。
//
// 背景：大纲生成、事实抽取、评分这些环节要求模型稳定吐出一个**结构**，此前的做法是
// 「让它输出 JSON，然后祈祷」——失败发生在昂贵的生成之后，且失败原因无法区分。
// 这里提供两条路径，并在响应里如实标明用了哪一条。

// 响应格式类型（与 OpenAI 兼容上游的 response_format.type 对齐）。
const (
	ResponseFormatText       = "text"
	ResponseFormatJSONObject = "json_object"
	ResponseFormatJSONSchema = "json_schema"
)

// 结构化生成路径标识：写在响应里，调用方与运维都能核对
// 「这次到底受没受 Schema 约束」，而不是靠猜。
const (
	StructuredPathJSONSchema        = "json_schema"
	StructuredPathPromptConstrained = "prompt_constrained"
)

// 结构化模式，控制是否允许回退。
const (
	// StructuredModeAuto 先试 json_schema，上游不支持时自动回退到 prompt 约束。
	StructuredModeAuto = "auto"
	// StructuredModeJSONSchema 只允许 json_schema，失败即报错（用于验证上游能力）。
	StructuredModeJSONSchema = "json_schema"
	// StructuredModePrompt 直接走 prompt 约束（上游明确不支持 response_format 时使用）。
	StructuredModePrompt = "prompt_constrained"
)

// fence 是 Markdown 代码围栏（三个反引号）。
//
// 用码点构造而不是字面量：本仓库的源码在工具链里经过多次转写，字面反引号既容易被
// 吞掉，也会让嵌在模板里的编辑出错——domain/diff.go 与 domain/summary.go 都因转义踩过坑。
var fence = strings.Repeat(string(rune(96)), 3)

// JSONSchemaSpec 是 json_schema 约束的载荷。
type JSONSchemaSpec struct {
	Name   string `json:"name"`
	Strict bool   `json:"strict,omitempty"`
	Schema any    `json:"schema"`
}

// ResponseFormat 是出站 response_format 字段。
type ResponseFormat struct {
	Type       string          `json:"type"`
	JSONSchema *JSONSchemaSpec `json:"json_schema,omitempty"`
}

// NewJSONSchemaResponseFormat 构造 json_schema 约束。
func NewJSONSchemaResponseFormat(name string, strict bool, schema any) *ResponseFormat {
	return &ResponseFormat{
		Type:       ResponseFormatJSONSchema,
		JSONSchema: &JSONSchemaSpec{Name: name, Strict: strict, Schema: schema},
	}
}

// NormalizeStructuredMode 归一化模式；空值按 auto 处理。
func NormalizeStructuredMode(mode string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", StructuredModeAuto:
		return StructuredModeAuto, nil
	case StructuredModeJSONSchema:
		return StructuredModeJSONSchema, nil
	case StructuredModePrompt, "prompt":
		return StructuredModePrompt, nil
	default:
		return "", fmt.Errorf("未知的结构化模式: %s", mode)
	}
}

// ExtractJSON 从模型输出里抽出一个 JSON 文本。
//
// 三级容错，按「越少改动越可信」的顺序：
//  1. 整体本身就是合法 JSON —— 直接用；
//  2. 被 Markdown 代码围栏包着 —— 剥掉围栏；
//  3. 混在解释性文字里 —— 扫出第一个**括号平衡**的 {...} 或 [...]（跳过字符串内的括号与转义）。
//
// 刻意不接受标量（数字/布尔/裸字符串）：调用方要的是结构，标量输出说明模型没按要求做，
// 静默接受比报错更危险。
func ExtractJSON(content string) (string, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return "", fmt.Errorf("模型输出为空")
	}
	if isJSONContainer(trimmed) && json.Valid([]byte(trimmed)) {
		return trimmed, nil
	}
	if unfenced := stripCodeFence(trimmed); unfenced != "" {
		if isJSONContainer(unfenced) && json.Valid([]byte(unfenced)) {
			return unfenced, nil
		}
		trimmed = unfenced
	}
	if candidate := firstBalancedValue(trimmed); candidate != "" && json.Valid([]byte(candidate)) {
		return candidate, nil
	}
	return "", fmt.Errorf("输出中未找到合法的 JSON 对象或数组")
}

func isJSONContainer(text string) bool {
	return strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[")
}

// stripCodeFence 剥掉 Markdown 代码围栏；没有围栏时返回空串。
func stripCodeFence(content string) string {
	index := strings.Index(content, fence)
	if index < 0 {
		return ""
	}
	rest := content[index+len(fence):]
	if newline := strings.IndexAny(rest, "\r\n"); newline >= 0 {
		language := strings.ToLower(strings.TrimSpace(rest[:newline]))
		if language == "" || language == "json" {
			rest = rest[newline+1:]
		}
	}
	if end := strings.Index(rest, fence); end >= 0 {
		return strings.TrimSpace(rest[:end])
	}
	return ""
}

// firstBalancedValue 扫描第一个括号平衡的 JSON 对象或数组。
func firstBalancedValue(content string) string {
	start := -1
	var open, closing byte
	for index := 0; index < len(content); index++ {
		if content[index] == '{' || content[index] == '[' {
			start = index
			open = content[index]
			if open == '{' {
				closing = '}'
			} else {
				closing = ']'
			}
			break
		}
	}
	if start < 0 {
		return ""
	}
	depth := 0
	inString := false
	escaped := false
	for index := start; index < len(content); index++ {
		current := content[index]
		switch {
		case escaped:
			escaped = false
		case current == '\\' && inString:
			escaped = true
		case current == '"':
			inString = !inString
		case inString:
			// 字符串内的括号不参与配对。
		case current == open:
			depth++
		case current == closing:
			depth--
			if depth == 0 {
				return content[start : index+1]
			}
		}
	}
	return ""
}

// ValidateAgainstSchemaShape 做**形状**校验，而不是完整 JSON Schema 校验。
//
// 覆盖「object 的 required 字段是否存在」这一最常出错的环节。不引入 JSON Schema 库
// （新增依赖需要 ADR），因此这里刻意只做能做到且能说清的那部分：只检查 required，
// 不检查类型、枚举与嵌套约束——调用方需要完整校验时应自行处理，而不是以为这里做了。
func ValidateAgainstSchemaShape(schema any, jsonText string) error {
	spec, ok := schema.(map[string]any)
	if !ok {
		return nil
	}
	required, ok := spec["required"].([]any)
	if !ok || len(required) == 0 {
		return nil
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(jsonText), &decoded); err != nil {
		return fmt.Errorf("输出不是 JSON 对象: %w", err)
	}
	missing := make([]string, 0, len(required))
	for _, item := range required {
		name, ok := item.(string)
		if !ok {
			continue
		}
		if _, exists := decoded[name]; !exists {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("输出缺少 Schema 要求的字段: %s", strings.Join(missing, ", "))
	}
	return nil
}
