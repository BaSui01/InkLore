package routing

import (
	"context"
	"fmt"
	"math"
	"math/bits"
	"os"
	"sync"
	"time"
)

// LatencyObservation 是一次生成请求的非重叠阶段时延观测。
// 异步视频如果没有可信 upstream completed_at，只填 CombinedProviderWaitMS，
// 不把未知时间伪拆成 provider_processing/result_detection。
type LatencyObservation struct {
	ProfileHash            string
	ProtocolFingerprint    string
	ObservedAt             time.Time
	EndToEndMS             int64
	InputPrepareMS         int64
	QueueWaitMS            int64
	ProviderSubmitMS       int64
	ProviderProcessingMS   int64
	ResultDetectionMS      int64
	PostprocessMS          int64
	CombinedProviderWaitMS int64
	EndToEndCompleteKnown  bool
	SuccessKnown           bool
	Success                bool
}

func (o LatencyObservation) durationMS() int64 {
	if o.EndToEndCompleteKnown && o.EndToEndMS > 0 {
		return o.EndToEndMS
	}
	providerWait := o.ProviderProcessingMS + o.ResultDetectionMS
	if providerWait <= 0 {
		providerWait = o.CombinedProviderWaitMS
	}
	return maxLatencyInt64(0, o.InputPrepareMS) +
		maxLatencyInt64(0, o.QueueWaitMS) +
		maxLatencyInt64(0, o.ProviderSubmitMS) +
		maxLatencyInt64(0, providerWait) +
		maxLatencyInt64(0, o.PostprocessMS)
}

// DurationMilliseconds 暴露统一的非重叠阶段合计，供持久化指标设施计算桶位。
func (o LatencyObservation) DurationMilliseconds() int64 {
	return o.durationMS()
}

// LatencyMetricSnapshot 是固定直方图查询结果，避免规划热路径排序全部历史样本。
type LatencyMetricSnapshot struct {
	P50MS               int64
	P90MS               int64
	ConservativeUpperMS int64
	SampleCount         uint64
	Confidence          float64
	Coverage            float64
}

// LatencyHistogram 是线程安全的固定长度对数直方图；Observe O(1)，分位查询 O(B)。
type LatencyHistogram struct {
	mu       sync.RWMutex
	buckets  [64]uint64
	count    uint64
	lastSeen time.Time
}

func (h *LatencyHistogram) Observe(observation LatencyObservation) {
	if h == nil {
		return
	}
	duration := observation.durationMS()
	if duration <= 0 {
		return
	}
	index := latencyBucketIndex(duration)
	now := observation.ObservedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	h.mu.Lock()
	h.buckets[index]++
	h.count++
	h.lastSeen = now
	h.mu.Unlock()
}

// MergeBucket 从持久化固定桶恢复统计，不需要加载历史明细样本。
func (h *LatencyHistogram) MergeBucket(index int, count uint64, lastSeen time.Time) {
	if h == nil || count == 0 || index < 0 || index >= len(h.buckets) {
		return
	}
	h.mu.Lock()
	h.buckets[index] += count
	h.count += count
	if lastSeen.After(h.lastSeen) {
		h.lastSeen = lastSeen
	}
	h.mu.Unlock()
}

// LatencyBucketIndex 返回固定对数直方图桶位，持久化层只保存桶计数。
func LatencyBucketIndex(observation LatencyObservation) int {
	duration := observation.durationMS()
	if duration <= 0 {
		return -1
	}
	return latencyBucketIndex(duration)
}

func (h *LatencyHistogram) Snapshot(now time.Time) LatencyMetricSnapshot {
	if h == nil {
		return LatencyMetricSnapshot{}
	}
	h.mu.RLock()
	buckets := h.buckets
	count := h.count
	lastSeen := h.lastSeen
	h.mu.RUnlock()
	if count == 0 {
		return LatencyMetricSnapshot{}
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	p50 := histogramQuantile(buckets, count, 0.50)
	p90 := histogramQuantile(buckets, count, 0.90)
	upper := p90
	if count < 20 {
		upper = maxLatencyInt64(int64(math.Round(float64(p90)*1.50)), p90+5_000)
	} else {
		upper = maxLatencyInt64(int64(math.Round(float64(p90)*1.20)), p90+1_000)
	}
	confidence := math.Min(1, float64(count)/100)
	if !lastSeen.IsZero() {
		age := now.Sub(lastSeen)
		if age > 24*time.Hour {
			confidence *= 0.25
		} else if age > 6*time.Hour {
			confidence *= 0.60
		}
	}
	return LatencyMetricSnapshot{
		P50MS: p50, P90MS: p90, ConservativeUpperMS: upper,
		SampleCount: count, Confidence: confidence, Coverage: 1,
	}
}

func latencyBucketIndex(durationMS int64) int {
	if durationMS <= 1 {
		return 0
	}
	index := bits.Len64(uint64(durationMS))
	if index >= 64 {
		return 63
	}
	return index
}

func histogramQuantile(buckets [64]uint64, count uint64, quantile float64) int64 {
	if count == 0 {
		return 0
	}
	target := uint64(math.Ceil(float64(count) * quantile))
	if target == 0 {
		target = 1
	}
	var cumulative uint64
	for index, bucketCount := range buckets {
		cumulative += bucketCount
		if cumulative >= target {
			if index == 0 {
				return 1
			}
			return int64(uint64(1) << uint(index))
		}
	}
	return int64(^uint64(0) >> 1)
}

func maxLatencyInt64(value, fallback int64) int64 {
	if value > fallback {
		return value
	}
	return fallback
}

const runtimeEventLimit = 512

// RuntimeDecision 只允许记录脱敏后的数值与标识，不保留输入、参数、凭据和档案正文。
type RuntimeDecision struct {
	At               time.Time          `json:"at"`
	Stage            string             `json:"stage"`
	Model            string             `json:"model"`
	MappingID        int64              `json:"mapping_id"`
	ChannelID        int64              `json:"channel_id"`
	Reason           string             `json:"reason"`
	Strategy         string             `json:"strategy,omitempty"`
	AlgorithmVersion string             `json:"algorithm_version,omitempty"`
	Candidates       []RuntimeCandidate `json:"candidates,omitempty"`
	CandidateCount   int                `json:"candidate_count,omitempty"`
}

type RuntimeCandidate struct {
	MappingID        int64   `json:"mapping_id"`
	ChannelID        int64   `json:"channel_id"`
	Priority         int     `json:"priority"`
	ConfiguredWeight int     `json:"configured_weight"`
	RuntimeWeight    float64 `json:"runtime_weight"`
	HealthWeight     float64 `json:"health_weight"`
	LoadWeight       float64 `json:"load_weight"`
	StrategyScore    float64 `json:"strategy_score"`
	TargetShare      float64 `json:"target_share"`
	InFlight         int     `json:"in_flight"`
	Capacity         int     `json:"capacity"`
	LoadKnown        bool    `json:"load_known"`
	ETAP90MS         int64   `json:"eta_p90_ms"`
	Confidence       float64 `json:"confidence"`
	Reason           string  `json:"reason"`
}

type RuntimeSnapshot struct {
	Scope         string            `json:"scope"`
	Instance      string            `json:"instance"`
	StartedAt     time.Time         `json:"started_at"`
	ObservedAt    time.Time         `json:"observed_at"`
	WindowSeconds int               `json:"window_seconds"`
	EventLimit    int               `json:"event_limit"`
	BufferFull    bool              `json:"buffer_full"`
	Events        []RuntimeDecision `json:"events"`
}

// RuntimeMonitor 是有界的进程级诊断窗口，不冒充持久审计或集群流量统计。
type RuntimeMonitor struct {
	mu       sync.Mutex
	started  time.Time
	instance string
	events   []RuntimeDecision
	next     int
}

func NewRuntimeMonitor() *RuntimeMonitor {
	host, _ := os.Hostname()
	return &RuntimeMonitor{started: time.Now().UTC(), instance: fmt.Sprintf("%s:%d", host, os.Getpid())}
}

func (m *RuntimeMonitor) record(event RuntimeDecision) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.events) < runtimeEventLimit {
		m.events = append(m.events, event)
		return
	}
	m.events[m.next] = event
	m.next = (m.next + 1) % runtimeEventLimit
}

func (m *RuntimeMonitor) RecordPlan(model string, plan *GenerationRoutePlan) {
	if m == nil || plan == nil {
		return
	}
	event := RuntimeDecision{At: time.Now().UTC(), Stage: "planning", Model: model,
		MappingID: plan.SelectedMappingID, ChannelID: plan.SelectedChannelID,
		Reason: plan.SelectionReason, Strategy: string(plan.StrategyCode), AlgorithmVersion: plan.AlgorithmVersion,
		CandidateCount: len(plan.Candidates)}
	for _, c := range plan.Candidates {
		if len(event.Candidates) >= 128 {
			break
		}
		reason := "eligible"
		switch {
		case !c.Candidate.Available:
			reason = "unavailable"
		case c.Candidate.Weight <= 0:
			reason = "weight_disabled"
		case !c.Eligibility.Compatible || !c.Eligibility.Known:
			reason = "capability_or_constraint"
		case c.Candidate.Priority != plan.SelectedPriority:
			reason = "lower_priority"
		case c.TargetShare <= 0:
			reason = "strategy_filtered"
		}
		event.Candidates = append(event.Candidates, RuntimeCandidate{
			MappingID: c.Candidate.MappingID, ChannelID: c.Candidate.ChannelID,
			Priority: c.Candidate.Priority, ConfiguredWeight: c.Candidate.Weight,
			RuntimeWeight: c.SchedulingWeight, TargetShare: c.TargetShare,
			HealthWeight: c.Candidate.HealthWeight, LoadWeight: c.Candidate.RuntimeWeight, StrategyScore: c.Score,
			InFlight: c.Candidate.InFlight, Capacity: c.Candidate.Capacity, LoadKnown: c.Candidate.LoadKnown,
			ETAP90MS: c.Estimate.ETAP90Milliseconds, Confidence: c.Estimate.Confidence, Reason: reason,
		})
	}
	m.record(event)
}

func (m *RuntimeMonitor) RecordAssignment(model string, mappingID, channelID int64, reason string) {
	m.record(RuntimeDecision{At: time.Now().UTC(), Stage: "assignment", Model: model,
		MappingID: mappingID, ChannelID: channelID, Reason: reason})
}

func (m *RuntimeMonitor) Snapshot(model string) RuntimeSnapshot {
	now := time.Now().UTC()
	result := RuntimeSnapshot{Scope: "process_local", ObservedAt: now, WindowSeconds: 1800,
		EventLimit: runtimeEventLimit, Events: []RuntimeDecision{}}
	if m == nil {
		return result
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	result.Instance, result.StartedAt = m.instance, m.started
	result.BufferFull = len(m.events) == runtimeEventLimit
	includedPlan := false
	for i := 0; i < len(m.events); i++ {
		index := (m.next + len(m.events) - 1 - i) % len(m.events)
		if len(m.events) < runtimeEventLimit {
			index = len(m.events) - 1 - i
		}
		event := m.events[index]
		if event.Model != model || now.Sub(event.At) > 30*time.Minute {
			continue
		}
		// 轮询仅携带最近一次规划的明细，避免重复传输窗口内数百份候选矩阵。
		// 历史规划的完整快照仍由 submission 持久化元数据承担。
		if event.Stage == "planning" && !includedPlan {
			event.Candidates = append([]RuntimeCandidate(nil), event.Candidates...)
			includedPlan = true
		} else {
			event.Candidates = nil
		}
		result.Events = append(result.Events, event)
	}
	return result
}

func (p *Pool) RuntimeMonitor() *RuntimeMonitor { return p.monitor }

// KeyLoad 只读取已有并发计数；未配置容量的 key 不伪装成已观测到空闲。
func (p *Pool) KeyLoad(ctx context.Context, keyID int64, capacity int) (int, bool) {
	limiter := p.concurrencyLimiter()
	if limiter == nil || capacity <= 0 {
		return 0, false
	}
	return limiter.GetInFlight(ctx, keyID), true
}
