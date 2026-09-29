package routing

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"InkLore/internal/platform/logger"
)

// CircuitState 熔断器状态
type CircuitState int

const (
	StateClosed CircuitState = iota
	StateHalfOpen
	StateOpen
)

func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateHalfOpen:
		return "half-open"
	case StateOpen:
		return "open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig 熔断器配置
type CircuitBreakerConfig struct {
	FailureThreshold int           // 连续失败触发熔断的次数阈值（默认 5 次）
	CoolingDuration  time.Duration // 熔断冷却持续时间（默认 30 秒）
	Clock            func() time.Time
}

// DefaultCircuitBreakerConfig 默认熔断配置
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		CoolingDuration:  30 * time.Second,
		Clock:            time.Now,
	}
}

// KeyCircuitBreaker 渠道密钥级熔断器接口
type KeyCircuitBreaker interface {
	// IsAvailable 快速判定该 Key 是否可用（支持在选路候选集过滤中无副作用检查）
	IsAvailable(ctx context.Context, keyID int64) bool

	// Allow 在选中该 Key 时进行准入（如果是半开试探，将占用试探配额）
	Allow(ctx context.Context, keyID int64) bool

	// RecordOutcome 记录调用结果，更新熔断状态
	RecordOutcome(ctx context.Context, keyID int64, outcome KeyCallOutcome)

	// GetState 获取当前指定 Key 的熔断状态
	GetState(keyID int64) CircuitState

	// Reset 重置指定 Key 熔断器状态为 Closed
	Reset(keyID int64)
}

type keyBreakerState struct {
	mu                 sync.Mutex
	state              CircuitState
	consecutiveFails   int
	openUntil          time.Time
	halfOpenProbeTaken bool
	// trips 是累计触发熔断次数（进程级计数器，供 /metrics 抓取），不随状态恢复清零。
	trips uint64
}

// MemoryKeyCircuitBreaker 内存实现的 Key 级自适应熔断器
type MemoryKeyCircuitBreaker struct {
	config CircuitBreakerConfig
	mu     sync.RWMutex
	keys   map[int64]*keyBreakerState
}

// NewMemoryKeyCircuitBreaker 创建内存实现的密钥熔断器
func NewMemoryKeyCircuitBreaker(cfg CircuitBreakerConfig) *MemoryKeyCircuitBreaker {
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = 5
	}
	if cfg.CoolingDuration <= 0 {
		cfg.CoolingDuration = 30 * time.Second
	}
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	return &MemoryKeyCircuitBreaker{
		config: cfg,
		keys:   make(map[int64]*keyBreakerState),
	}
}

func (cb *MemoryKeyCircuitBreaker) getOrCreate(keyID int64) *keyBreakerState {
	cb.mu.RLock()
	st, ok := cb.keys[keyID]
	cb.mu.RUnlock()
	if ok {
		return st
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()
	st, ok = cb.keys[keyID]
	if !ok {
		st = &keyBreakerState{
			state: StateClosed,
		}
		cb.keys[keyID] = st
	}
	return st
}

// IsAvailable 判断该 Key 是否可用（不占用试探配额）
func (cb *MemoryKeyCircuitBreaker) IsAvailable(_ context.Context, keyID int64) bool {
	if cb == nil {
		return true
	}
	st := cb.getOrCreate(keyID)
	st.mu.Lock()
	defer st.mu.Unlock()

	now := cb.config.Clock()
	switch st.state {
	case StateClosed:
		return true
	case StateOpen:
		// 如果冷却时间已过，可转为半开试探
		return now.After(st.openUntil)
	case StateHalfOpen:
		return !st.halfOpenProbeTaken
	default:
		return true
	}
}

// Allow 准入请求（半开状态下会占用唯一试探名额）
func (cb *MemoryKeyCircuitBreaker) Allow(ctx context.Context, keyID int64) bool {
	if cb == nil {
		return true
	}
	st := cb.getOrCreate(keyID)
	st.mu.Lock()
	defer st.mu.Unlock()

	if ctx == nil {
		ctx = context.Background()
	}
	now := cb.config.Clock()
	switch st.state {
	case StateClosed:
		return true
	case StateOpen:
		if now.After(st.openUntil) {
			// 冷却期满，转为半开，并占用唯一探活槽位
			st.state = StateHalfOpen
			st.halfOpenProbeTaken = true
			logger.InfoContext(ctx, "渠道密钥熔断冷却期满开启半开试探",
				"category", "modelruntime",
				"key_id", keyID,
				"new_state", StateHalfOpen.String(),
			)
			return true
		}
		return false
	case StateHalfOpen:
		if !st.halfOpenProbeTaken {
			st.halfOpenProbeTaken = true
			return true
		}
		return false
	default:
		return true
	}
}

// RecordOutcome 记录调用结果，推进状态机
func (cb *MemoryKeyCircuitBreaker) RecordOutcome(ctx context.Context, keyID int64, outcome KeyCallOutcome) {
	if cb == nil {
		return
	}
	st := cb.getOrCreate(keyID)
	st.mu.Lock()
	defer st.mu.Unlock()

	if ctx == nil {
		ctx = context.Background()
	}
	now := cb.config.Clock()
	if outcome.Success {
		// 设计考量：若当前处于半开试探状态且本次请求成功，说明上游服务已恢复，记录自愈事件并重置为 Closed
		if st.state == StateHalfOpen {
			logger.InfoContext(ctx, "渠道密钥半开探测成功，熔断器自愈恢复正常",
				"category", "modelruntime",
				"key_id", keyID,
				"new_state", StateClosed.String(),
			)
		}
		// 成功：无论当前是 Closed 还是 HalfOpen，重置为 Closed 状态
		st.state = StateClosed
		st.consecutiveFails = 0
		st.halfOpenProbeTaken = false
		return
	}

	// 仅在上游不可用 (5xx) 或网络传输超时失败时累加熔断计数
	// 业务层 400 Bad Request 或非网络问题不触发服务熔断
	isBreakerFailure := outcome.FailureKind == KeyCallFailureProviderDown || outcome.FailureKind == KeyCallFailureTransport
	if !isBreakerFailure {
		if st.state == StateHalfOpen {
			st.halfOpenProbeTaken = false
		}
		return
	}

	switch st.state {
	case StateClosed:
		st.consecutiveFails++
		if st.consecutiveFails >= cb.config.FailureThreshold {
			st.state = StateOpen
			st.openUntil = now.Add(cb.config.CoolingDuration)
			st.trips++
			logger.WarnContext(ctx, "渠道密钥连续失败触发熔断保护",
				"category", "modelruntime",
				"key_id", keyID,
				"consecutive_fails", st.consecutiveFails,
				"cooling_duration_sec", cb.config.CoolingDuration.Seconds(),
				"failure_kind", outcome.FailureKind,
				"new_state", StateOpen.String(),
			)
		}
	case StateHalfOpen:
		// 半开探测请求失败，立刻重新进入 Open 状态并重新开始冷却
		st.state = StateOpen
		st.trips++
		st.openUntil = now.Add(cb.config.CoolingDuration)
		st.halfOpenProbeTaken = false
		logger.WarnContext(ctx, "渠道密钥半开探测请求失败重新进入熔断冷却",
			"category", "modelruntime",
			"key_id", keyID,
			"cooling_duration_sec", cb.config.CoolingDuration.Seconds(),
			"failure_kind", outcome.FailureKind,
			"new_state", StateOpen.String(),
		)
	case StateOpen:
		// 已经是 Open，保持冷却
	}
}

// GetState 获取指定 Key 的当前状态
func (cb *MemoryKeyCircuitBreaker) GetState(keyID int64) CircuitState {
	if cb == nil {
		return StateClosed
	}
	st := cb.getOrCreate(keyID)
	st.mu.Lock()
	defer st.mu.Unlock()

	now := cb.config.Clock()
	if st.state == StateOpen && now.After(st.openUntil) {
		return StateHalfOpen
	}
	return st.state
}

// BreakerStat 是单个密钥的熔断器只读快照，供 /metrics 抓取（INK-056）。
type BreakerStat struct {
	State CircuitState
	// Trips 是**累计**触发熔断次数（进程级计数器，随进程重启归零），不是当前是否熔断。
	Trips uint64
}

// Stats 返回全部已登记密钥的快照。
//
// 只读取状态与计数，不改变熔断状态机；冷却期已过的 Open 会按 GetState 的语义报告为半开。
func (cb *MemoryKeyCircuitBreaker) Stats() map[int64]BreakerStat {
	if cb == nil {
		return nil
	}
	cb.mu.RLock()
	ids := make([]int64, 0, len(cb.keys))
	for id := range cb.keys {
		ids = append(ids, id)
	}
	cb.mu.RUnlock()

	out := make(map[int64]BreakerStat, len(ids))
	for _, id := range ids {
		st := cb.getOrCreate(id)
		st.mu.Lock()
		state, trips, openUntil := st.state, st.trips, st.openUntil
		st.mu.Unlock()
		if state == StateOpen && cb.config.Clock().After(openUntil) {
			state = StateHalfOpen
		}
		out[id] = BreakerStat{State: state, Trips: trips}
	}
	return out
}

// Reset 重置指定 Key 熔断器状态为 Closed
func (cb *MemoryKeyCircuitBreaker) Reset(keyID int64) {
	if cb == nil {
		return
	}
	st := cb.getOrCreate(keyID)
	st.mu.Lock()
	defer st.mu.Unlock()
	st.state = StateClosed
	st.consecutiveFails = 0
	st.halfOpenProbeTaken = false
}

// RoutingStrategy 是可插拔的候选排序算法。能力过滤、priority 分层和计费边界
// 仍由 planner 统一执行，策略只能对已经通过硬约束的同层候选计算分数和探索倍率。
type RoutingStrategy interface {
	Code() RoutingStrategyCode
	Score(CandidateEstimate, GenerationRouteRequirement) float64
	ExplorationMultiplier(RoutingPolicy) float64
}

type routingStrategyRegistry struct {
	mu         sync.RWMutex
	strategies map[RoutingStrategyCode]RoutingStrategy
}

var defaultRoutingStrategies = newDefaultRoutingStrategyRegistry()

func newDefaultRoutingStrategyRegistry() *routingStrategyRegistry {
	registry := &routingStrategyRegistry{strategies: make(map[RoutingStrategyCode]RoutingStrategy, 4)}
	for _, strategy := range []RoutingStrategy{
		slaFirstStrategy{},
		reliabilityFirstStrategy{},
		costFirstStrategy{},
		balancedStrategy{},
	} {
		registry.strategies[strategy.Code()] = strategy
	}
	return registry
}

// RegisterRoutingStrategy 注册或替换一套排序策略，供新版本按需扩展。
// 替换只影响后续规划，不修改已经持久化的 route plan。
func RegisterRoutingStrategy(strategy RoutingStrategy) error {
	if strategy == nil || strings.TrimSpace(string(strategy.Code())) == "" {
		return fmt.Errorf("routing strategy is nil or code is empty")
	}
	defaultRoutingStrategies.mu.Lock()
	defaultRoutingStrategies.strategies[strategy.Code()] = strategy
	defaultRoutingStrategies.mu.Unlock()
	return nil
}

func resolveRoutingStrategy(code RoutingStrategyCode) RoutingStrategy {
	defaultRoutingStrategies.mu.RLock()
	strategy := defaultRoutingStrategies.strategies[code]
	defaultRoutingStrategies.mu.RUnlock()
	if strategy != nil {
		return strategy
	}
	return slaFirstStrategy{}
}

type strategyScoreBase struct{}

func scoreByCode(estimate CandidateEstimate, strategy RoutingStrategyCode, req GenerationRouteRequirement) float64 {
	estimate = normalizeEstimate(estimate)
	eta := float64(estimate.ETAP90Milliseconds)
	if req.MaxETAMilliseconds > 0 {
		eta /= float64(req.MaxETAMilliseconds)
	} else if eta > 0 {
		eta = math.Log1p(eta)
	}
	cost := float64(estimate.ChargeMicros)
	if req.MaxBudgetMicros > 0 {
		cost /= float64(req.MaxBudgetMicros)
	} else if cost > 0 {
		cost = math.Log1p(cost)
	}
	failure := 1 - estimate.SuccessLowerBound
	load := math.Max(0, math.Min(1, estimate.QueueSaturation))
	uncertainty := 1 - estimate.Confidence
	switch strategy {
	case RoutingStrategyReliabilityFirst:
		return failure*10 + eta*0.1 + cost*0.01 + load*0.05
	case RoutingStrategyCostFirst:
		return cost*10 + eta*0.1 + failure*0.5 + load*0.05
	case RoutingStrategyBalanced:
		return eta*0.50 + failure*0.30 + cost*0.15 + load*0.05 + uncertainty*0.02
	default:
		return eta*0.55 + failure*0.30 + load*0.10 + cost*0.05
	}
}

type slaFirstStrategy struct{}

func (slaFirstStrategy) Code() RoutingStrategyCode { return RoutingStrategySLAFirst }
func (slaFirstStrategy) Score(estimate CandidateEstimate, req GenerationRouteRequirement) float64 {
	return scoreByCode(estimate, RoutingStrategySLAFirst, req)
}
func (slaFirstStrategy) ExplorationMultiplier(RoutingPolicy) float64 { return 0 }

type reliabilityFirstStrategy struct{}

func (reliabilityFirstStrategy) Code() RoutingStrategyCode { return RoutingStrategyReliabilityFirst }
func (reliabilityFirstStrategy) Score(estimate CandidateEstimate, req GenerationRouteRequirement) float64 {
	return scoreByCode(estimate, RoutingStrategyReliabilityFirst, req)
}
func (reliabilityFirstStrategy) ExplorationMultiplier(RoutingPolicy) float64 { return 0 }

type costFirstStrategy struct{}

func (costFirstStrategy) Code() RoutingStrategyCode { return RoutingStrategyCostFirst }
func (costFirstStrategy) Score(estimate CandidateEstimate, req GenerationRouteRequirement) float64 {
	return scoreByCode(estimate, RoutingStrategyCostFirst, req)
}
func (costFirstStrategy) ExplorationMultiplier(RoutingPolicy) float64 { return 0 }

type balancedStrategy struct{}

func (balancedStrategy) Code() RoutingStrategyCode { return RoutingStrategyBalanced }
func (balancedStrategy) Score(estimate CandidateEstimate, req GenerationRouteRequirement) float64 {
	return scoreByCode(estimate, RoutingStrategyBalanced, req)
}
func (balancedStrategy) ExplorationMultiplier(policy RoutingPolicy) float64 {
	if !policy.AllowExploration {
		return 0
	}
	return 1
}
