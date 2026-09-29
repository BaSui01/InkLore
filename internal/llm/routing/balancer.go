package routing

import (
	"math"
	"sync"
	"time"

	"InkLore/internal/platform/logger"
)

// Balancer 负载均衡器接口
type Balancer interface {
	Select(keys []*KeyWrapper) (*KeyWrapper, error)
}

// WeightedRoundRobinBalancer 加权轮询负载均衡器
type WeightedRoundRobinBalancer struct {
	mu            sync.Mutex
	currentIdx    int
	currentWeight int
	maxWeight     int
	gcdWeight     int
}

// NewWeightedRoundRobinBalancer 创建加权轮询均衡器
func NewWeightedRoundRobinBalancer() *WeightedRoundRobinBalancer {
	return &WeightedRoundRobinBalancer{}
}

// Select 选择一个可用的密钥
func (b *WeightedRoundRobinBalancer) Select(keys []*KeyWrapper) (*KeyWrapper, error) {
	if len(keys) == 0 {
		return nil, ErrNoAvailableKey
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// 过滤可用的key
	available := b.filterAvailableKeys(keys)
	if len(available) == 0 {
		return nil, ErrNoAvailableKey
	}

	// 如果只有一个可用，直接返回
	if len(available) == 1 {
		return available[0], nil
	}

	// 计算权重
	b.calculateWeights(available)

	// 加权轮询选择（添加循环上限防止死循环）
	maxIterations := len(available) * b.maxWeight
	if maxIterations <= 0 {
		maxIterations = len(available) * 10 // 降级保护
	}

	for i := 0; i < maxIterations; i++ {
		b.currentIdx = (b.currentIdx + 1) % len(available)

		if b.currentIdx == 0 {
			b.currentWeight = b.currentWeight - b.gcdWeight
			if b.currentWeight <= 0 {
				b.currentWeight = b.maxWeight
			}
		}

		if available[b.currentIdx].Key.Weight >= b.currentWeight {
			return available[b.currentIdx], nil
		}
	}

	// 兜底：返回第一个可用 Key
	return available[0], nil
}

// filterAvailableKeys 过滤可用的密钥
func (b *WeightedRoundRobinBalancer) filterAvailableKeys(keys []*KeyWrapper) []*KeyWrapper {
	available := make([]*KeyWrapper, 0)
	now := time.Now()
	inactiveCount := 0
	coolingCount := 0
	limitReachedCount := 0
	inUseCount := 0

	for _, kw := range keys {
		if kw == nil || kw.Key == nil {
			inactiveCount++
			continue
		}
		// 检查状态（使用新版 ChannelStatusActive 常量）
		if kw.Key.Status != ChannelStatusActive {
			inactiveCount++
			continue
		}

		// 检查是否在冷却中
		if now.Before(kw.Cooldown) {
			coolingCount++
			continue
		}

		// 检查每日限额（DailyLimit=0 表示不限额）
		if kw.Key.DailyLimit > 0 && kw.Key.DailyUsed >= kw.Key.DailyLimit {
			limitReachedCount++
			continue
		}

		// 检查是否正在使用中（避免并发冲突）
		if kw.InUse {
			inUseCount++
			continue
		}

		available = append(available, kw)
	}

	// 设计考量：当所有候选 Key 全被过滤导致无可用渠道时，记录详细维度的淘汰原因统计，便于快速排查是配额超限、状态非活动还是熔断冷却
	if len(available) == 0 && len(keys) > 0 {
		logger.Warn("模型渠道负载均衡候选池全员不可用",
			"category", "modelruntime",
			"total_candidates", len(keys),
			"inactive_count", inactiveCount,
			"cooling_count", coolingCount,
			"limit_reached_count", limitReachedCount,
			"in_use_count", inUseCount,
		)
	}

	return available
}

// calculateWeights 计算权重参数
func (b *WeightedRoundRobinBalancer) calculateWeights(keys []*KeyWrapper) {
	b.maxWeight = 0
	weights := make([]int, len(keys))

	for i, kw := range keys {
		weights[i] = kw.Key.Weight
		if kw.Key.Weight > b.maxWeight {
			b.maxWeight = kw.Key.Weight
		}
	}

	if len(weights) > 0 {
		b.gcdWeight = weights[0]
		for i := 1; i < len(weights); i++ {
			b.gcdWeight = gcd(b.gcdWeight, weights[i])
		}
	}

	// 兜底保护：若权重全部为 0（按语义应禁用），避免后续轮询陷入无意义循环
	if b.maxWeight <= 0 {
		b.maxWeight = 1
	}
	if b.gcdWeight <= 0 {
		b.gcdWeight = 1
	}
}

// gcd 计算最大公约数
func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// LeastUsedBalancer 最少使用负载均衡器
type LeastUsedBalancer struct {
	mu sync.Mutex
}

// NewLeastUsedBalancer 创建最少使用均衡器
func NewLeastUsedBalancer() *LeastUsedBalancer {
	return &LeastUsedBalancer{}
}

// Select 选择使用次数最少的密钥
func (b *LeastUsedBalancer) Select(keys []*KeyWrapper) (*KeyWrapper, error) {
	if len(keys) == 0 {
		return nil, ErrNoAvailableKey
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	var selected *KeyWrapper
	minUsed := int(^uint(0) >> 1) // MaxInt
	now := time.Now()

	for _, kw := range keys {
		// 检查可用性（使用新版 ChannelStatusActive 常量）
		if kw.Key.Status != ChannelStatusActive {
			continue
		}
		if now.Before(kw.Cooldown) {
			continue
		}
		if kw.Key.DailyLimit > 0 && kw.Key.DailyUsed >= kw.Key.DailyLimit {
			continue
		}
		if kw.InUse {
			continue
		}

		// 选择使用次数最少的
		if kw.Key.DailyUsed < minUsed {
			minUsed = kw.Key.DailyUsed
			selected = kw
		}
	}

	if selected == nil {
		return nil, ErrNoAvailableKey
	}

	return selected, nil
}

// SmoothWeightedSelector 线程安全的平滑加权轮询选择器（Nginx SWRR 算法）。
// 保证在多渠道加权调度时，请求交替平滑分布，彻底杜绝连续命中同一渠道导致的瞬间排队与 429。
type SmoothWeightedSelector struct {
	mu     sync.Mutex
	states map[int64]int // map[candidateID]currentWeight
}

// NewSmoothWeightedSelector 创建平滑加权轮询选择器
func NewSmoothWeightedSelector() *SmoothWeightedSelector {
	return &SmoothWeightedSelector{
		states: make(map[int64]int),
	}
}

// Select 从候选列表中按平滑加权轮询算法选出一个索引
func (s *SmoothWeightedSelector) Select(candidateIDs []int64, candidateWeights []int) int {
	n := len(candidateIDs)
	if n == 0 || len(candidateWeights) == 0 || n != len(candidateWeights) {
		return -1
	}
	if n == 1 {
		return 0
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.states == nil {
		s.states = make(map[int64]int)
	}

	totalWeight := 0
	bestIdx := -1
	maxCurrent := -1 << 62 // 极小初值

	for i, id := range candidateIDs {
		weight := candidateWeights[i]
		if weight <= 0 {
			continue
		}
		totalWeight += weight
		current := s.states[id] + weight
		s.states[id] = current

		if current > maxCurrent {
			maxCurrent = current
			bestIdx = i
		}
	}

	if bestIdx >= 0 && totalWeight > 0 {
		bestID := candidateIDs[bestIdx]
		s.states[bestID] -= totalWeight
		return bestIdx
	}

	// 兜底返回第一个候选
	return 0
}

// SchedulingWeight 保留低权重的折减精度；整数配置仅作为历史调用方的兼容入口。
func SchedulingWeight(configured int, runtime float64) float64 {
	if runtime > 0 && !math.IsNaN(runtime) && !math.IsInf(runtime, 0) {
		return runtime
	}
	return math.Max(float64(configured), 0)
}

// FixedSchedulingWeight 只在选择器边界量化，避免 W:1 在每次折减后都被取整回 1。
func FixedSchedulingWeight(weight float64) int {
	if weight <= 0 || math.IsNaN(weight) || math.IsInf(weight, 0) {
		return 0
	}
	return int(math.Max(1, math.Round(math.Min(weight, 1e6)*1000)))
}

// LoadAdjustedWeight 优先比较容量占用率；历史限流器只有在途计数时，
// 保留按排队任务数保守折减的兼容行为，但不能将其展示为容量占用率。
func LoadAdjustedWeight(weight float64, inFlight, capacity int) float64 {
	if inFlight <= 0 {
		return weight
	}
	if capacity <= 0 {
		return weight / float64(1+inFlight)
	}
	return weight / (1 + float64(inFlight)/float64(capacity))
}
