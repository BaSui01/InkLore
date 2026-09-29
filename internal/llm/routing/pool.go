package routing

import (
	"InkLore/internal/platform/logger"
	"context"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
)

type routeParamChannelProvider interface {
	SelectChannelWithRouteParams(modelOrProvider string, region string, params map[string]interface{}) (*Channel, *ChannelKey, error)
}

// Pool API 池，基于渠道管理
// 特性：
// 1. 从 channel_keys 表读取密钥
// 2. 使用 ChannelProvider 接口的智能路由算法
// 3. 使用 ModelProvider 获取模型（从 models 表）
// 4. 可选按 channel_key.concurrency_limit 限制同 Key 在途 LLM 请求数
// 5. 可选按 channel_key.rate_limit 限制同 Key 每分钟请求数 (RPM)
type Pool struct {
	channelProvider ChannelProvider
	modelProvider   ModelProvider

	limiterMu      sync.RWMutex
	keyLimiter     KeyConcurrencyLimiter
	keyRateLimiter KeyRateLimiter
	circuitBreaker KeyCircuitBreaker
	selector       *SmoothWeightedSelector
	keySelector    *SmoothWeightedSelector
	monitor        *RuntimeMonitor

	leaseMu     sync.Mutex
	leases      map[int64]leaseRecord
	nextLeaseID int64
}

type leaseRecord struct {
	keyID   int64
	limiter KeyConcurrencyLimiter
}

// NewPool 创建 API 池
// channelProvider: 渠道服务（实现 ChannelProvider 接口）
// modelProvider: 模型服务（实现 ModelProvider 接口）
func NewPool(channelProvider ChannelProvider, modelProvider ModelProvider) *Pool {
	return &Pool{
		channelProvider: channelProvider,
		modelProvider:   modelProvider,
		keyLimiter:      NewMemoryKeyConcurrencyLimiter(),
		keyRateLimiter:  NewMemoryKeyRateLimiter(),
		circuitBreaker:  NewMemoryKeyCircuitBreaker(DefaultCircuitBreakerConfig()),
		selector:        NewSmoothWeightedSelector(),
		keySelector:     NewSmoothWeightedSelector(),
		monitor:         NewRuntimeMonitor(),
	}
}

// SetKeyConcurrencyLimiter overrides the in-flight limiter (Redis preferred in production).
func (p *Pool) SetKeyConcurrencyLimiter(limiter KeyConcurrencyLimiter) {
	if p == nil {
		return
	}
	if limiter == nil {
		limiter = NewMemoryKeyConcurrencyLimiter()
	}
	p.limiterMu.Lock()
	p.keyLimiter = limiter
	p.limiterMu.Unlock()
}

// SetKeyRateLimiter overrides the per-key RPM limiter (Redis preferred in production).
func (p *Pool) SetKeyRateLimiter(limiter KeyRateLimiter) {
	if p == nil {
		return
	}
	if limiter == nil {
		limiter = NewMemoryKeyRateLimiter()
	}
	p.limiterMu.Lock()
	p.keyRateLimiter = limiter
	p.limiterMu.Unlock()
}

func (p *Pool) rateLimiter() KeyRateLimiter {
	if p == nil {
		return nil
	}
	p.limiterMu.RLock()
	defer p.limiterMu.RUnlock()
	return p.keyRateLimiter
}
func (p *Pool) concurrencyLimiter() KeyConcurrencyLimiter {
	if p == nil {
		return nil
	}
	p.limiterMu.RLock()
	defer p.limiterMu.RUnlock()
	return p.keyLimiter
}

// SetCircuitBreaker overrides the circuit breaker.
func (p *Pool) SetCircuitBreaker(cb KeyCircuitBreaker) {
	if p == nil {
		return
	}
	p.limiterMu.Lock()
	p.circuitBreaker = cb
	p.limiterMu.Unlock()
}

func (p *Pool) getCircuitBreaker() KeyCircuitBreaker {
	if p == nil {
		return nil
	}
	p.limiterMu.RLock()
	defer p.limiterMu.RUnlock()
	return p.circuitBreaker
}

// ReleaseChannelKeyLease releases a selected key without recording call statistics.
func (p *Pool) ReleaseChannelKeyLease(leaseID int64) {
	lease, ok := p.takeLease(leaseID)
	if ok && lease.limiter != nil {
		lease.limiter.Release(context.Background(), lease.keyID)
	}
}

// Initialize 初始化 API 池
func (p *Pool) Initialize(ctx context.Context) error {
	// 获取所有活跃渠道
	channels, err := p.channelProvider.GetActiveChannels()
	if err != nil {
		return fmt.Errorf("获取活跃渠道失败: %w", err)
	}

	// 按提供商分组统计，用于一行汇总日志（避免逐渠道刷屏）
	providerCount := make(map[string]int)
	totalKeys := 0
	loaded := 0

	// 为每个渠道加载密钥（只做健康检查 + 统计，无需在本地缓存；选择走 ChannelProvider.SelectChannel）
	for _, channel := range channels {
		keys, err := p.channelProvider.GetChannelKeys(channel.ID)
		if err != nil {
			logger.WarnContext(ctx, "获取渠道 的密钥失败", "channel_name", channel.Name, "error", err)
			continue
		}

		if len(keys) == 0 {
			logger.WarnContext(ctx, "渠道 没有可用密钥", "channel_name", channel.Name)
			continue
		}

		providerCount[channel.Provider]++
		totalKeys += len(keys)
		loaded++
	}

	// 构建按提供商分组的汇总字符串: "openai:4, gemini:2"
	providers := make([]string, 0, len(providerCount))
	for name := range providerCount {
		providers = append(providers, name)
	}
	sort.Strings(providers)
	parts := make([]string, 0, len(providers))
	for _, name := range providers {
		parts = append(parts, fmt.Sprintf("%s:%d", name, providerCount[name]))
	}
	breakdown := strings.Join(parts, ", ")
	if breakdown == "" {
		breakdown = "无"
	}

	logger.InfoContext(ctx, "初始化完成: 渠道 / 提供商 / 密钥", "loaded", loaded, "provider_count_count", len(providerCount), "total_keys", totalKeys, "breakdown", breakdown)
	return nil
}

// ChannelBreakerSnapshot 是按渠道聚合的密钥级熔断快照（供 /metrics 抓取，INK-056）。
type ChannelBreakerSnapshot struct {
	Channel string
	// Keys 是该渠道已登记熔断状态的密钥数；OpenKeys 是当前处于 Open 的数量。
	Keys       int
	OpenKeys   int
	TripsTotal uint64
}

// BreakerSnapshot 按渠道聚合密钥级熔断状态。
//
// 渠道名来自渠道注册表（keyID → 渠道名）；注册表未提供该索引时返回 nil，而不是猜一个名字。
// 快照只含渠道名与计数，不含任何密钥内容（ADR-0004）。
func (p *Pool) BreakerSnapshot() []ChannelBreakerSnapshot {
	if p == nil {
		return nil
	}
	provider, ok := p.getCircuitBreaker().(interface {
		Stats() map[int64]BreakerStat
	})
	if !ok {
		return nil
	}
	indexer, ok := p.channelProvider.(interface{ KeyChannelIndex() map[int64]string })
	if !ok {
		return nil
	}
	index := indexer.KeyChannelIndex()

	aggregated := make(map[string]*ChannelBreakerSnapshot)
	for keyID, stat := range provider.Stats() {
		name := strings.TrimSpace(index[keyID])
		if name == "" {
			name = fmt.Sprintf("key-%d", keyID)
		}
		item, exists := aggregated[name]
		if !exists {
			item = &ChannelBreakerSnapshot{Channel: name}
			aggregated[name] = item
		}
		item.Keys++
		item.TripsTotal += stat.Trips
		if stat.State == StateOpen {
			item.OpenKeys++
		}
	}
	names := make([]string, 0, len(aggregated))
	for name := range aggregated {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]ChannelBreakerSnapshot, 0, len(names))
	for _, name := range names {
		out = append(out, *aggregated[name])
	}
	return out
}

// GetChannelKeyWithRegion 获取渠道密钥（可选区域偏好）
// 参数 providerName: 提供商名称（如 "openai", "gemini"）
// 参数 region: 可选区域；匹配失败时会回退到全量渠道
// 返回: 渠道信息、解密后的密钥值、密钥ID、错误
func (p *Pool) GetChannelKeyWithRegion(providerName, region string) (channel *Channel, decryptedKey string, keyID int64, err error) {
	return p.getChannelKeyWithRegion(providerName, region, true)
}

func (p *Pool) GetChannelKeyWithRouteParams(providerName string, params map[string]interface{}) (channel *Channel, decryptedKey string, keyID int64, err error) {
	return p.getChannelKeyWithRegionAndParams(context.Background(), providerName, "", params, true)
}

// GetChannelKeyWithRouteParamsContext keeps strict RPM waits bound to the task lifecycle.
func (p *Pool) GetChannelKeyWithRouteParamsContext(ctx context.Context, providerName string, params map[string]interface{}) (channel *Channel, decryptedKey string, keyID int64, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return p.getChannelKeyWithRegionAndParams(ctx, providerName, "", params, true)
}

// GetChannelKeyContext 是 GetChannelKey 的 ctx 版本：选路内并发槽/RPM 排队绑定调用方 ctx，
// 供 assistant pipeline 等带超时上下文的调用方使用（110s 超时可中断选路内排队）。
func (p *Pool) GetChannelKeyContext(ctx context.Context, providerName string) (channel *Channel, decryptedKey string, keyID int64, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return p.getChannelKeyWithRegionAndParams(ctx, providerName, "", nil, true)
}

// GetChannelKeyWithRegionQuiet 获取渠道密钥（可选区域偏好），但不打印“密钥已分配”日志
// 用于内部做多次选择尝试时避免日志刷屏（例如：排除某渠道后寻找备选渠道）
func (p *Pool) GetChannelKeyWithRegionQuiet(providerName, region string) (channel *Channel, decryptedKey string, keyID int64, err error) {
	return p.getChannelKeyWithRegion(providerName, region, false)
}

// getChannelKeyWithRegion 获取渠道密钥的内部实现
func (p *Pool) getChannelKeyWithRegion(providerName, region string, logAssignment bool) (channel *Channel, decryptedKey string, keyID int64, err error) {
	return p.getChannelKeyWithRegionAndParams(context.Background(), providerName, region, nil, logAssignment)
}

func (p *Pool) getChannelKeyWithRegionAndParams(ctx context.Context, providerName, region string, params map[string]interface{}, logAssignment bool) (channel *Channel, decryptedKey string, keyID int64, err error) {
	const maxAttempts = 8
	var lastErr error
	// P3：候选集一次获取（routing rule + channel mapping + key 聚合只查一次库），
	// 8 次重试循环内仅做内存筛选（容量/RPM 门控、解密、lease），不再每次重跑选路 SQL。
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, "", 0, ctxErr
	}
	candidates, candidateErr := p.channelProvider.ListChannelKeyCandidates(ctx, providerName, region, params)
	if candidateErr != nil {
		// 选路查询阶段 ctx 被取消/超时 → 返回 ctx.Err()（与既有取消契约一致）。
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, "", 0, ctxErr
		}
		return nil, "", 0, fmt.Errorf("选择渠道失败: %w", candidateErr)
	}

	// 选路循环：每次尝试在内存候选集上按权重重新选路；调用方 ctx 取消/超时立即返回，
	// 避免排队期间无限重试。并发容量失败的 key 记入 failedKeys 并在后续尝试中排除，
	// 使同渠道兄弟 key 能在内存中确定性重试成功（不重新查库、不做 channel 级排除）。
	var failedKeys map[int64]struct{}
	for attempt := 0; attempt < maxAttempts; attempt++ {
		// 预取消/已超时的 ctx 直接快速返回（不执行选路 SQL、不占用并发槽/限流令牌）。
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, "", 0, ctxErr
		}
		// 内存选路：区域过滤 → 优先级分组 → 组内加权随机 → 密钥加权随机。
		// 语义与 ChannelService 选路一致（routing rule 优先、权重、key 可用性），不重新查库。
		selCandidates := candidates
		if cb := p.getCircuitBreaker(); cb != nil {
			availCandidates := candidatesFilteringBreaker(selCandidates, cb, ctx)
			if len(availCandidates) > 0 {
				selCandidates = availCandidates
			}
		}
		if len(failedKeys) > 0 {
			selCandidates = candidatesExcludingKeys(selCandidates, failedKeys)
			if len(selCandidates) == 0 {
				// 全部候选的 key 都处于容量失败：回退全量候选继续尝试（保留容量重试语义，
				// 让 8 次上限内的多次 acquire 都发生，最终收敛为 ConcurrencyLimitError）。
				selCandidates = candidates
				failedKeys = nil
			}
		}
		selectedCandidate, channelKey := p.selectFromCandidates(selCandidates, region)
		if selectedCandidate == nil || selectedCandidate.Channel == nil || channelKey == nil {
			// 候选集存在但均不可选（如全部映射权重为 0 / 密钥不可用），与既有
			// keys_temporarily_unavailable 语义一致：终止选路，不再重试。
			if lastErr != nil {
				return nil, "", 0, fmt.Errorf("选择渠道失败: no selectable channel key (after concurrency retries: %v)", lastErr)
			}
			return nil, "", 0, fmt.Errorf("选择渠道失败: no selectable channel key")
		}
		if cb := p.getCircuitBreaker(); cb != nil {
			if !cb.Allow(ctx, channelKey.ID) {
				lastErr = fmt.Errorf("key %d circuit breaker open", channelKey.ID)
				if failedKeys == nil {
					failedKeys = make(map[int64]struct{})
				}
				failedKeys[channelKey.ID] = struct{}{}
				continue
			}
		}
		// 浅拷贝渠道并回填 mapping 信息，避免污染候选集共享指针（与 attachSelectedMapping 一致）。
		selectedChannel := *selectedCandidate.Channel
		selectedChannel.SelectedMappingID = selectedCandidate.MappingID
		selectedChannel.SelectedUpstreamModel = strings.TrimSpace(selectedCandidate.UpstreamModel)
		selectedChannel.SelectedParamOverride = selectedCandidate.ParamOverride
		selectedChannel.SelectedProfileHash = selectedCandidate.ProfileHash
		selectedChannel.SelectedPricingSnapshotHash = selectedCandidate.PricingSnapshotHash
		selectedChannel.SelectedProtocolFingerprint = selectedCandidate.ProtocolFingerprint

		// 占用 Key 并发槽：limit<=0 表示不限制且完全跳过 inflight 计数。
		var acquiredWith KeyConcurrencyLimiter
		if channelKey.ConcurrencyLimit > 0 {
			acquiredWith = p.concurrencyLimiter()
			if acquiredWith != nil {
				acquired, acquireErr := acquiredWith.TryAcquire(ctx, channelKey.ID, channelKey.ConcurrencyLimit)
				if acquireErr != nil {
					// 调用方 ctx 已取消/超时时直接返回 ctx.Err()，避免被误分类为容量错误后继续重试。
					if ctxErr := ctx.Err(); ctxErr != nil {
						return nil, "", 0, ctxErr
					}
					lastErr = acquireErr
					logger.Warn("获取密钥并发槽失败", "key_id", channelKey.ID, "acquire_err", acquireErr)
					continue
				}
				if !acquired {
					lastErr = fmt.Errorf("key %d at concurrency limit %d", channelKey.ID, channelKey.ConcurrencyLimit)
					if failedKeys == nil {
						failedKeys = make(map[int64]struct{})
					}
					failedKeys[channelKey.ID] = struct{}{}
					if logAssignment {
						logger.Debug("channel key concurrency limit reached", "channel", selectedChannel.Name, "key_name", channelKey.KeyName, "key_id", channelKey.ID, "limit", channelKey.ConcurrencyLimit)
					}
					continue
				}
			}
		}

		// 可选 RPM 闸门：channel_keys.rate_limit（每分钟请求上限）。
		if channelKey.RateLimit > 0 {
			rl := p.rateLimiter()
			if rl != nil {
				allowed, rateErr := rl.TryConsume(ctx, channelKey.ID, channelKey.RateLimit)
				if rateErr != nil {
					if acquiredWith != nil {
						acquiredWith.Release(context.Background(), channelKey.ID)
					}
					if ctx.Err() != nil {
						return nil, "", 0, ctx.Err()
					}
					lastErr = rateErr
					logger.Warn("获取密钥 RPM 失败", "key_id", channelKey.ID, "rate_err", rateErr)
					continue
				}
				if !allowed {
					if acquiredWith != nil {
						acquiredWith.Release(context.Background(), channelKey.ID)
					}
					lastErr = fmt.Errorf("key %d at rate limit %d rpm", channelKey.ID, channelKey.RateLimit)
					if logAssignment {
						logger.Debug("channel key rate limit reached", "channel", selectedChannel.Name, "key_name", channelKey.KeyName, "key_id", channelKey.ID, "rate_limit", channelKey.RateLimit)
					}
					continue
				}
			}
		}

		// 解密密钥
		decryptedKey, err = p.channelProvider.DecryptChannelKey(channelKey.KeyValue)
		if err != nil {
			if acquiredWith != nil {
				acquiredWith.Release(context.Background(), channelKey.ID)
			}
			return nil, "", 0, fmt.Errorf("解密密钥失败: %w", err)
		}

		if logAssignment {
			logger.Debug("channel key selected", "channel", selectedChannel.Name, "key_name", channelKey.KeyName, "key_id", channelKey.ID, "region", selectedChannel.Region, "concurrency_limit", channelKey.ConcurrencyLimit)
		}

		if p.monitor != nil {
			reason := "adaptive_weighted"
			if selectedCandidate.Preferred {
				reason = "plan_preserved"
			}
			if selectedCandidate.FromRule {
				reason = "routing_rule"
			}
			p.monitor.RecordAssignment(providerName, selectedCandidate.MappingID, selectedChannel.ID, reason)
		}
		return &selectedChannel, decryptedKey, p.registerLease(channelKey.ID, acquiredWith), nil
	}
	if lastErr != nil {
		if strings.Contains(strings.ToLower(lastErr.Error()), "rate limit") {
			return nil, "", 0, fmt.Errorf("选择渠道失败: %w", &RateLimitError{Cause: lastErr})
		}
		return nil, "", 0, fmt.Errorf("选择渠道失败: %w", &ConcurrencyLimitError{Cause: lastErr})
	}
	return nil, "", 0, fmt.Errorf("选择渠道失败: %w", &ConcurrencyLimitError{})
}

// candidatesExcludingKeys 返回剔除指定 key 后的候选集副本（不改动原候选切片）。
// 候选的 Keys 列表被过滤后为空时，整个候选从结果中移除。
func candidatesExcludingKeys(candidates []*ChannelKeyCandidate, excluded map[int64]struct{}) []*ChannelKeyCandidate {
	if len(candidates) == 0 || len(excluded) == 0 {
		return candidates
	}
	out := make([]*ChannelKeyCandidate, 0, len(candidates))
	for _, c := range candidates {
		if c == nil || c.Channel == nil {
			continue
		}
		keys := c.Keys
		filtered := make([]*ChannelKey, 0, len(keys))
		for _, k := range keys {
			if k == nil {
				continue
			}
			if _, skip := excluded[k.ID]; skip {
				continue
			}
			filtered = append(filtered, k)
		}
		if len(filtered) == 0 {
			continue
		}
		clone := *c
		clone.Keys = filtered
		out = append(out, &clone)
	}
	return out
}

func candidatesFilteringBreaker(candidates []*ChannelKeyCandidate, cb KeyCircuitBreaker, ctx context.Context) []*ChannelKeyCandidate {
	if len(candidates) == 0 || cb == nil {
		return candidates
	}
	out := make([]*ChannelKeyCandidate, 0, len(candidates))
	for _, c := range candidates {
		if c == nil || c.Channel == nil {
			continue
		}
		var filtered []*ChannelKey
		for _, k := range c.Keys {
			if k == nil {
				continue
			}
			if cb.IsAvailable(ctx, k.ID) {
				filtered = append(filtered, k)
			}
		}
		if len(filtered) == 0 {
			continue
		}
		clone := *c
		clone.Keys = filtered
		out = append(out, &clone)
	}
	return out
}

// selectFromCandidates 在内存候选集上按选路语义选择候选与密钥（不执行任何数据库查询）。
// 语义与 ChannelService 一致：路由规则候选优先（取第一条命中规则）；否则区域过滤
// （无匹配回退全量）→ 优先级从高到低分组 → 组内在途折减与平滑加权轮询（SWRR）选择（weight<=0 跳过，全禁用降级
// 到下一组）→ 密钥加权平滑轮询。
func (p *Pool) selectFromCandidates(candidates []*ChannelKeyCandidate, region string) (*ChannelKeyCandidate, *ChannelKey) {
	if len(candidates) == 0 {
		return nil, nil
	}
	// 路由规则候选：构建期已按规则命中顺序完成区域/渠道活跃/排除过滤，直接取第一条。
	if candidates[0].FromRule {
		return candidates[0], p.pickCandidateKey(candidates[0])
	}

	filtered := filterCandidatesByRegion(candidates, region)
	if len(filtered) == 0 {
		return nil, nil
	}
	// 可用密钥过滤：无活跃密钥的候选不可选（构建期已预过滤，防御性再查一次）。
	usable := make([]*ChannelKeyCandidate, 0, len(filtered))
	for _, c := range filtered {
		if c != nil && c.Channel != nil && len(c.Keys) > 0 && (!c.WeightDisabled || c.EffectiveWeight > 0) {
			usable = append(usable, c)
		}
	}
	if len(usable) == 0 {
		return nil, nil
	}
	// submission 已固化的 mapping 只在当前最高可用 priority 层内优先，
	// 这样计划不会被普通权重随机改写，也不会让陈旧计划越过管理员硬层级。
	maxPriority := usable[0].EffectivePriority
	for _, candidate := range usable[1:] {
		if candidate.EffectivePriority > maxPriority {
			maxPriority = candidate.EffectivePriority
		}
	}
	for _, candidate := range usable {
		if candidate.Preferred && candidate.EffectivePriority == maxPriority {
			return candidate, p.pickCandidateKey(candidate)
		}
	}

	// 按优先级分组，从高到低尝试；组内平滑加权轮询（结合实时在途并发折减），全禁用（weight<=0）自动降级到下一组。
	priorityGroups := make(map[int][]*ChannelKeyCandidate)
	priorities := make([]int, 0, 8)
	seen := make(map[int]bool)
	for _, c := range usable {
		pr := c.EffectivePriority
		priorityGroups[pr] = append(priorityGroups[pr], c)
		if !seen[pr] {
			seen[pr] = true
			priorities = append(priorities, pr)
		}
	}
	sort.Slice(priorities, func(i, j int) bool { return priorities[i] > priorities[j] })
	for _, pr := range priorities {
		if selected := p.selectGroupCandidate(priorityGroups[pr]); selected != nil {
			return selected, p.pickCandidateKey(selected)
		}
	}
	return nil, nil
}

// selectGroupCandidate 在同优先级候选组内，先通过实时在途并发数（In-Flight）进行动态折减，
// 然后使用平滑加权轮询（SWRR）算法进行交替平滑选路，彻底消除瞬时踩踏与忙闲不均。
func (p *Pool) selectGroupCandidate(candidates []*ChannelKeyCandidate) *ChannelKeyCandidate {
	if len(candidates) == 0 {
		return nil
	}
	if len(candidates) == 1 {
		if candidates[0] == nil || candidates[0].WeightDisabled {
			return nil
		}
		return candidates[0]
	}

	limiter := p.concurrencyLimiter()

	validCandidates := make([]*ChannelKeyCandidate, 0, len(candidates))
	candidateIDs := make([]int64, 0, len(candidates))
	candidateWeights := make([]int, 0, len(candidates))

	for _, c := range candidates {
		if c == nil || c.WeightDisabled {
			continue
		}
		inFlight := 0
		capacity := 0
		if limiter != nil {
			for _, k := range c.Keys {
				if k != nil && k.ID > 0 {
					inFlight += limiter.GetInFlight(context.Background(), k.ID)
					if k.ConcurrencyLimit > 0 {
						capacity += k.ConcurrencyLimit
					}
				}
			}
		}

		// 根据容量占用率折减，保留小数直到选择器边界，低权重仍然能感知负载。
		weight := SchedulingWeight(c.EffectiveWeight, c.RuntimeWeight)
		if weight <= 0 {
			weight = 1
		}
		weight = LoadAdjustedWeight(weight, inFlight, capacity)
		dynWeight := FixedSchedulingWeight(weight)

		// candidateID 标识：优先使用 MappingID（多映射独立轮询），否则使用 Channel.ID
		cid := c.MappingID
		if cid <= 0 && c.Channel != nil {
			cid = c.Channel.ID
		}

		validCandidates = append(validCandidates, c)
		candidateIDs = append(candidateIDs, cid)
		candidateWeights = append(candidateWeights, dynWeight)
	}

	if len(validCandidates) == 0 {
		return nil
	}
	if len(validCandidates) == 1 {
		return validCandidates[0]
	}

	if p != nil && p.selector != nil {
		idx := p.selector.Select(candidateIDs, candidateWeights)
		if idx >= 0 && idx < len(validCandidates) {
			return validCandidates[idx]
		}
	}

	// 降级回退：加权随机
	return weightedRandomCandidate(validCandidates)
}

// weightedRandomCandidate 按 effectiveWeight 加权随机选择候选；weight<=0 视为禁用（跳过）。
// 语义与 admin 层 weightedRandomSelectCandidates 一致：单候选直接返回（不校验权重，
// 权重为 0 仅表示“未设置渠道级覆盖”，不影响唯一可选性）；多候选时权重竞争。
func weightedRandomCandidate(candidates []*ChannelKeyCandidate) *ChannelKeyCandidate {
	if len(candidates) == 0 {
		return nil
	}
	if len(candidates) == 1 {
		return candidates[0]
	}
	totalWeight := 0
	weighted := make([]*ChannelKeyCandidate, 0, len(candidates))
	for _, c := range candidates {
		if c == nil || c.EffectiveWeight <= 0 {
			continue
		}
		weighted = append(weighted, c)
		totalWeight += c.EffectiveWeight
	}
	if len(weighted) == 0 {
		return nil
	}
	if len(weighted) == 1 {
		return weighted[0]
	}
	randomWeight := rand.Intn(totalWeight)
	currentWeight := 0
	for _, c := range weighted {
		currentWeight += c.EffectiveWeight
		if currentWeight > randomWeight {
			return c
		}
	}
	return weighted[0]
}

// pickCandidateKey 按密钥权重与在途负载平滑选择候选密钥
func (p *Pool) pickCandidateKey(candidate *ChannelKeyCandidate) *ChannelKey {
	if candidate == nil {
		return nil
	}
	keys := candidate.Keys
	if len(keys) == 0 {
		return nil
	}
	if len(keys) == 1 {
		return keys[0]
	}

	limiter := p.concurrencyLimiter()

	validKeys := make([]*ChannelKey, 0, len(keys))
	keyIDs := make([]int64, 0, len(keys))
	keyWeights := make([]int, 0, len(keys))

	for _, k := range keys {
		if k == nil || k.Weight <= 0 {
			continue
		}
		inFlight := 0
		if limiter != nil && k.ID > 0 {
			inFlight = limiter.GetInFlight(context.Background(), k.ID)
		}
		dynWeight := FixedSchedulingWeight(float64(k.Weight))
		if inFlight > 0 {
			dynWeight = FixedSchedulingWeight(LoadAdjustedWeight(float64(k.Weight), inFlight, k.ConcurrencyLimit))
		}
		validKeys = append(validKeys, k)
		keyIDs = append(keyIDs, k.ID)
		keyWeights = append(keyWeights, dynWeight)
	}

	if len(validKeys) == 0 {
		return nil
	}
	if len(validKeys) == 1 {
		return validKeys[0]
	}

	if p != nil && p.keySelector != nil {
		idx := p.keySelector.Select(keyIDs, keyWeights)
		if idx >= 0 && idx < len(validKeys) {
			return validKeys[idx]
		}
	}

	return validKeys[0]
}

// filterCandidatesByRegion 与 admin 层 filterCandidatesByRegion 语义一致：优先匹配区域，
// 无匹配时回退全量（不因区域偏好缩小候选集）。
func filterCandidatesByRegion(candidates []*ChannelKeyCandidate, region string) []*ChannelKeyCandidate {
	normalized := normalizeRegion(region)
	if normalized == "" {
		return candidates
	}
	matched := make([]*ChannelKeyCandidate, 0, len(candidates))
	for _, c := range candidates {
		if c != nil && c.Channel != nil && normalizeRegion(c.Channel.Region) == normalized {
			matched = append(matched, c)
		}
	}
	if len(matched) > 0 {
		return matched
	}
	return candidates
}

func normalizeRegion(region string) string {
	return strings.ToLower(strings.TrimSpace(region))
}

func (p *Pool) registerLease(keyID int64, limiter KeyConcurrencyLimiter) int64 {
	if limiter == nil {
		return keyID
	}
	p.leaseMu.Lock()
	defer p.leaseMu.Unlock()
	p.nextLeaseID--
	if p.nextLeaseID >= 0 {
		p.nextLeaseID = -1
	}
	if p.leases == nil {
		p.leases = make(map[int64]leaseRecord)
	}
	p.leases[p.nextLeaseID] = leaseRecord{keyID: keyID, limiter: limiter}
	return p.nextLeaseID
}

func (p *Pool) takeLease(leaseID int64) (leaseRecord, bool) {
	p.leaseMu.Lock()
	defer p.leaseMu.Unlock()
	lease, ok := p.leases[leaseID]
	if ok {
		delete(p.leases, leaseID)
	}
	return lease, ok
}

// GetChannelKey 获取渠道密钥（无区域偏好，兼容旧接口）
func (p *Pool) GetChannelKey(providerName string) (channel *Channel, decryptedKey string, keyID int64, err error) {
	return p.GetChannelKeyWithRegion(providerName, "")
}

func (p *Pool) GetChannelKeyByID(keyID int64) (channel *Channel, decryptedKey string, err error) {
	resolver, ok := p.channelProvider.(interface {
		GetChannelAndKeyByKeyID(int64) (*Channel, *ChannelKey, error)
	})
	if !ok {
		return nil, "", fmt.Errorf("channel provider does not support exact key lookup")
	}
	channel, key, err := resolver.GetChannelAndKeyByKeyID(keyID)
	if err != nil {
		return nil, "", err
	}
	if channel == nil || key == nil {
		return nil, "", fmt.Errorf("channel key %d not found", keyID)
	}
	decryptedKey, err = p.channelProvider.DecryptChannelKey(key.KeyValue)
	if err != nil {
		return nil, "", fmt.Errorf("解密密钥失败: %w", err)
	}
	return channel, decryptedKey, nil
}

// GetChannelKeyQuiet 获取渠道密钥（无区域偏好），但不打印“密钥已分配”日志
func (p *Pool) GetChannelKeyQuiet(providerName string) (channel *Channel, decryptedKey string, keyID int64, err error) {
	return p.GetChannelKeyWithRegionQuiet(providerName, "")
}

// ResolveChannelKeyID maps an internal lease handle to the persisted channel key ID.
func (p *Pool) ResolveChannelKeyID(leaseID int64) int64 {
	if lease, ok := p.peekLease(leaseID); ok {
		return lease.keyID
	}
	return leaseID
}

func (p *Pool) peekLease(leaseID int64) (leaseRecord, bool) {
	p.leaseMu.Lock()
	defer p.leaseMu.Unlock()
	lease, ok := p.leases[leaseID]
	return lease, ok
}

// RecordKeyCall 记录密钥调用结果（新版接口）
func (p *Pool) RecordKeyCall(leaseID int64, success bool, err error) {
	lease, ok := p.takeLease(leaseID)
	keyID := leaseID
	if ok {
		keyID = lease.keyID
		if lease.limiter != nil {
			lease.limiter.Release(context.Background(), keyID)
		}
	} else if leaseID < 0 {
		// Internal lease handles are one-shot. A duplicate callback must not be
		// persisted as if it were a real channel_keys.id.
		return
	}

	// 记录密钥调用
	outcome := classifyKeyCallOutcome(success, err)
	if cb := p.getCircuitBreaker(); cb != nil {
		cb.RecordOutcome(context.Background(), keyID, outcome)
	}
	if recordErr := p.channelProvider.RecordKeyCall(keyID, outcome); recordErr != nil {
		logger.Warn("记录密钥调用失败", "record_err", recordErr)
	}

	// 如果失败，记录告警日志（单次失败由上层重试与降级机制处理，不视为系统级严重错误）
	if !success && err != nil {
		logger.Warn("channel key call failed", "key_id", keyID, "error", err)
	}
}

// GetModelByID 根据 model_id 字符串获取模型（新版 Model）
func (p *Pool) GetModelByID(modelID string) (*Model, error) {
	return p.modelProvider.GetModelByName(modelID)
}

// GetModelByDBID 根据数据库自增 ID 获取模型（新版 Model）
func (p *Pool) GetModelByDBID(dbID int64) (*Model, error) {
	return p.modelProvider.GetModel(dbID)
}

// GetAllModels 获取所有可用模型（新版 Model）
func (p *Pool) GetAllModels() ([]*Model, error) {
	return p.modelProvider.GetActiveModels()
}

// GetMappingByChannelAndModel 获取渠道-模型映射（包含上游模型名）
func (p *Pool) GetMappingByChannelAndModel(channelID, modelID int64) (*ChannelModelMapping, error) {
	return p.modelProvider.GetMappingByChannelAndModel(channelID, modelID)
}

// GetMappingsByChannelAndModels 批量获取渠道-模型映射（包含上游模型名）
func (p *Pool) GetMappingsByChannelAndModels(channelID int64, modelIDs []int64) (map[int64]*ChannelModelMapping, error) {
	return p.modelProvider.GetMappingsByChannelAndModels(channelID, modelIDs)
}

// GetProviders 获取所有提供商名称
func (p *Pool) GetProviders() []string {
	providers, err := p.channelProvider.GetProviders()
	if err != nil {
		logger.Error("获取提供商列表失败", "error", err)
		return []string{}
	}
	return providers
}

// Reload 重新加载配置
func (p *Pool) Reload(ctx context.Context) error {
	return p.Initialize(ctx)
}

// ResetDailyUsage 重置每日使用量（由定时任务调用）
func (p *Pool) ResetDailyUsage() error {
	// 委托给 ChannelService
	return p.channelProvider.ResetDailyUsage(context.Background())
}

// ============================================================
// 渠道管理高级功能
// ============================================================

// GetChannelStats 获取渠道统计信息
func (p *Pool) GetChannelStats() ([]*ChannelWithStats, error) {
	return p.channelProvider.GetChannelStats()
}

// TestChannel 测试渠道连接
func (p *Pool) TestChannel(channelID int64) error {
	return p.channelProvider.TestChannel(channelID)
}
