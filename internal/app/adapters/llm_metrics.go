package adapters

import (
	"strings"

	llmdomain "InkLore/internal/llm/domain"
	"InkLore/internal/llm/routing"
	"InkLore/internal/platform/metrics"
)

// LLM 调用延迟直方图边界（毫秒）：覆盖本地桩到上游超时的量级。
var llmLatencyBucketsMS = []float64{50, 100, 250, 500, 1000, 2500, 5000, 10000, 30000}

// 指标名：对外契约，改名等于破坏监控面板，因此集中在这里声明。
const (
	metricLLMRequestsTotal = "inklore_llm_requests_total"
	metricLLMTokensTotal   = "inklore_llm_tokens_total"
	metricLLMLatency       = "inklore_llm_request_duration_ms"
	metricBreakerKeys      = "inklore_llm_breaker_keys"
	metricBreakerOpenKeys  = "inklore_llm_breaker_open_keys"
	metricBreakerTrips     = "inklore_llm_breaker_trips"
	metricBuildInfo        = "inklore_build_info"
	metricUptimeSeconds    = "inklore_uptime_seconds"
	metricDBConfigured     = "inklore_database_configured"
	metricRedisConfigured  = "inklore_redis_configured"
)

// LLMMetrics 把模型基座域的调用样本与选路熔断状态桥接到 platform/metrics（INK-056）。
//
// 两条通路：
//   - **推**：实现 llm/domain.MetricsRecorder，在每次上游调用结束时累加计数与延迟直方图；
//   - **拉**：注册 collector，抓取时向渠道池读取密钥级熔断快照（状态随时间变化，必须现读）。
//
// 标签只含模型名与渠道名（低基数、非敏感）：请求 ID、Prompt 正文与密钥一律不入标签（ADR-0004）。
type LLMMetrics struct {
	registry *metrics.Registry
	breakers func() []routing.ChannelBreakerSnapshot
}

// NewLLMMetrics 创建桥接器并注册拉取式采集。
func NewLLMMetrics(registry *metrics.Registry) *LLMMetrics {
	bridge := &LLMMetrics{registry: registry}
	if registry != nil {
		registry.RegisterCollector(bridge.collectBreakers)
	}
	return bridge
}

// SetBreakerSource 注入熔断快照来源（组合根在 ProviderRouter 构造完成后调用）。
func (m *LLMMetrics) SetBreakerSource(source func() []routing.ChannelBreakerSnapshot) {
	if m == nil {
		return
	}
	m.breakers = source
}

// RecordCall 实现 llm/domain.MetricsRecorder。
func (m *LLMMetrics) RecordCall(call llmdomain.CallMetric) {
	if m == nil || m.registry == nil {
		return
	}
	model := labelValue(call.Model, "unknown")
	channel := labelValue(call.Channel, "direct")
	m.registry.AddCounter(metricLLMRequestsTotal, map[string]string{
		"model": model, "channel": channel, "status": labelValue(call.Status, "unknown"),
	}, 1)
	if call.InputTokens > 0 {
		m.registry.AddCounter(metricLLMTokensTotal, map[string]string{
			"model": model, "channel": channel, "type": "input",
		}, float64(call.InputTokens))
	}
	if call.OutputTokens > 0 {
		m.registry.AddCounter(metricLLMTokensTotal, map[string]string{
			"model": model, "channel": channel, "type": "output",
		}, float64(call.OutputTokens))
	}
	m.registry.ObserveHistogram(metricLLMLatency, llmLatencyBucketsMS, map[string]string{
		"model": model, "channel": channel,
	}, float64(call.LatencyMS))
}

// collectBreakers 是拉取式采集：渠道池为空时不导出任何熔断序列。
func (m *LLMMetrics) collectBreakers(registry *metrics.Registry) {
	if m == nil || m.breakers == nil {
		return
	}
	for _, snapshot := range m.breakers() {
		labels := map[string]string{"channel": labelValue(snapshot.Channel, "unknown")}
		registry.SetGauge(metricBreakerKeys, labels, float64(snapshot.Keys))
		registry.SetGauge(metricBreakerOpenKeys, labels, float64(snapshot.OpenKeys))
		registry.SetGauge(metricBreakerTrips, labels, float64(snapshot.TripsTotal))
	}
}

// RegisterRuntimeGauges 注册进程级基础指标：构建信息、存活时长与依赖配置状态。
//
// 依赖状态只反映"是否配置"（启动期确定），不冒充实时健康：实时探测由 /readyz 承担。
func RegisterRuntimeGauges(registry *metrics.Registry, version, gitCommit string, databaseConfigured, redisConfigured bool) {
	if registry == nil {
		return
	}
	registry.RegisterCollector(func(reg *metrics.Registry) {
		reg.SetGauge(metricBuildInfo, map[string]string{
			"version": labelValue(version, "dev"), "git_commit": labelValue(gitCommit, "unknown"),
		}, 1)
		reg.SetGauge(metricUptimeSeconds, nil, metrics.Uptime().Seconds())
		reg.SetGauge(metricDBConfigured, nil, boolValue(databaseConfigured))
		reg.SetGauge(metricRedisConfigured, nil, boolValue(redisConfigured))
	})
}

func labelValue(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func boolValue(flag bool) float64 {
	if flag {
		return 1
	}
	return 0
}
