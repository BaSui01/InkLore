package domain

import (
	"context"
	"time"

	ai "InkLore/internal/llm/domain"
)

// SpanKind 链路节点类型
type SpanKind string

const (
	SpanKindRun      SpanKind = "run"
	SpanKindTurn     SpanKind = "turn"
	SpanKindThink    SpanKind = "think"
	SpanKindTool     SpanKind = "tool"
	SpanKindSubagent SpanKind = "subagent"
	SpanKindMCP      SpanKind = "mcp"
)

// TraceSpan 分布式追踪 Span 节点
type TraceSpan struct {
	SpanID       string         `json:"span_id"`
	TraceID      string         `json:"trace_id"`
	ParentSpanID string         `json:"parent_span_id,omitempty"`
	Kind         SpanKind       `json:"kind"`
	Name         string         `json:"name"`
	StartedAt    time.Time      `json:"started_at"`
	EndedAt      time.Time      `json:"ended_at"`
	DurationMs   int64          `json:"duration_ms"`
	Status       string         `json:"status"` // "ok" | "error" | "pending_approval"
	Attributes   map[string]any `json:"attributes,omitempty"`
}

// LatencyBreakdown 分段耗时度量
type LatencyBreakdown struct {
	TTFTMs     int64 `json:"ttft_ms"`     // 首字延迟 (Time To First Token)
	ThinkingMs int64 `json:"thinking_ms"` // 思考/推理耗时
	StreamMs   int64 `json:"stream_ms"`   // 流式传输耗时
	TotalMs    int64 `json:"total_ms"`    // 总耗时
}

// ModelPriceInfo 数据库存储的模型实际单价 ($ / 1M tokens) 与计费配置
type ModelPriceInfo struct {
	Currency        string  `json:"currency"` // "USD" | "CNY"
	InputPrice      float64 `json:"input_price"`
	OutputPrice     float64 `json:"output_price"`
	CacheReadPrice  float64 `json:"cache_read_price"`
	CacheWritePrice float64 `json:"cache_write_price"`
	ExchangeRate    float64 `json:"exchange_rate,omitempty"` // 可配置的汇率（如 USD->CNY）
}

// ModelPricingResolver 从数据库或模型服务动态解析单价的接口
type ModelPricingResolver interface {
	GetModelPricing(ctx context.Context, modelName string) (*ModelPriceInfo, error)
}

// CostBreakdown 费用成本度量
type CostBreakdown struct {
	Currency        string  `json:"currency"` // 计价币种（如 "USD" 或 "CNY"）
	InputTokens     int     `json:"input_tokens"`
	OutputTokens    int     `json:"output_tokens"`
	ReasoningTokens int     `json:"reasoning_tokens"`
	CachedTokens    int     `json:"cached_tokens"`
	InputCost       float64 `json:"input_cost"`
	OutputCost      float64 `json:"output_cost"`
	TotalCostUSD    float64 `json:"total_cost_usd"`
	TotalCostCNY    float64 `json:"total_cost_cny"` // 按配置汇率折算
}

// CalculateCostWithPricing 基于数据库读取的模型真实单价与动态汇率计算消耗费用（零硬编码）
func CalculateCostWithPricing(pricing *ModelPriceInfo, usage *ai.TokenUsage) CostBreakdown {
	currency := "USD"
	if pricing != nil && pricing.Currency != "" {
		currency = pricing.Currency
	}

	if usage == nil {
		return CostBreakdown{Currency: currency}
	}

	res := CostBreakdown{
		Currency:        currency,
		InputTokens:     usage.PromptTokens,
		OutputTokens:    usage.CompletionTokens,
		ReasoningTokens: usage.ReasoningTokens,
		CachedTokens:    usage.CachedTokens,
	}

	if pricing == nil {
		return res
	}

	inputPrice := pricing.InputPrice
	outputPrice := pricing.OutputPrice

	res.InputCost = float64(usage.PromptTokens) * (inputPrice / 1_000_000.0)
	res.OutputCost = float64(usage.CompletionTokens) * (outputPrice / 1_000_000.0)
	total := res.InputCost + res.OutputCost

	if currency == "CNY" {
		res.TotalCostCNY = total
		if pricing.ExchangeRate > 0 {
			res.TotalCostUSD = total / pricing.ExchangeRate
		}
	} else {
		res.TotalCostUSD = total
		if pricing.ExchangeRate > 0 {
			res.TotalCostCNY = total * pricing.ExchangeRate
		}
	}

	return res
}
