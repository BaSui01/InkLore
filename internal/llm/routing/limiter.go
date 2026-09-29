package routing

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"InkLore/internal/platform/cache"
	"InkLore/internal/platform/safego"
)

// KeyConcurrencyLimiter bounds in-flight LLM calls per channel key.
// ConcurrencyLimit<=0 means unlimited (no slot tracking).
type KeyConcurrencyLimiter interface {
	// TryAcquire returns false when the key is already at capacity.
	TryAcquire(ctx context.Context, keyID int64, limit int) (bool, error)
	// Release decrements the in-flight counter for a previously acquired key.
	Release(ctx context.Context, keyID int64)
	// GetInFlight returns the current number of in-flight requests for the key.
	GetInFlight(ctx context.Context, keyID int64) int
}

// keyConcurrencyTTL 是 in-flight 计数的兜底 TTL：
//   - 进程崩溃/超时中断导致 Release 缺失时，计数在 2 小时后自动清除（防永久泄漏）；
//   - 代价：活跃 key 的 TTL 只在计数从 0→1（key 创建）时设置，不随每次 INCR 刷新，
//     因此持续使用中的 key 每 2 小时计数会重置一次——生图/视频任务远短于 2 小时，
//     重置后并发限制从零重建，短暂超限由上游限流兜底，属有意的可靠性权衡。
const keyConcurrencyTTL = 2 * time.Hour

// MemoryKeyConcurrencyLimiter is process-local (tests / Redis disabled).
type MemoryKeyConcurrencyLimiter struct {
	mu       sync.Mutex
	inFlight map[int64]int
}

func NewMemoryKeyConcurrencyLimiter() *MemoryKeyConcurrencyLimiter {
	return &MemoryKeyConcurrencyLimiter{inFlight: make(map[int64]int)}
}

func (l *MemoryKeyConcurrencyLimiter) TryAcquire(_ context.Context, keyID int64, limit int) (bool, error) {
	if l == nil || keyID <= 0 || limit <= 0 {
		return true, nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.inFlight == nil {
		l.inFlight = make(map[int64]int)
	}
	if l.inFlight[keyID] >= limit {
		return false, nil
	}
	l.inFlight[keyID]++
	return true, nil
}

func (l *MemoryKeyConcurrencyLimiter) Release(_ context.Context, keyID int64) {
	if l == nil || keyID <= 0 {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	n := l.inFlight[keyID]
	if n <= 1 {
		delete(l.inFlight, keyID)
		return
	}
	l.inFlight[keyID] = n - 1
}

func (l *MemoryKeyConcurrencyLimiter) GetInFlight(_ context.Context, keyID int64) int {
	if l == nil || keyID <= 0 {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.inFlight[keyID]
}

// RedisKeyConcurrencyLimiter uses Redis INCR/DECR for multi-process safety.
type RedisKeyConcurrencyLimiter struct {
	redis *cache.RedisCache
}

func NewRedisKeyConcurrencyLimiter(redisCache *cache.RedisCache) *RedisKeyConcurrencyLimiter {
	if redisCache == nil || !redisCache.IsEnabled() {
		return nil
	}
	return &RedisKeyConcurrencyLimiter{redis: redisCache}
}

const redisKeyConcurrencyAcquireScript = `
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local ttl = tonumber(ARGV[2])
local current = tonumber(redis.call('GET', key) or '0')
if current >= limit then
  return 0
end
local n = redis.call('INCR', key)
if n == 1 then
  redis.call('PEXPIRE', key, ttl)
end
if n > limit then
  redis.call('DECR', key)
  return 0
end
return 1
`

const redisKeyConcurrencyReleaseScript = `
local key = KEYS[1]
local n = tonumber(redis.call('GET', key) or '0')
if n <= 0 then
  redis.call('DEL', key)
  return 0
end
n = redis.call('DECR', key)
if n <= 0 then
  redis.call('DEL', key)
  return 0
end
return n
`

func (l *RedisKeyConcurrencyLimiter) TryAcquire(ctx context.Context, keyID int64, limit int) (bool, error) {
	if l == nil || l.redis == nil || !l.redis.IsEnabled() || keyID <= 0 || limit <= 0 {
		return true, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	key := l.redis.WrapKey(fmt.Sprintf("channel_key:inflight:%d", keyID))
	result, err := l.redis.GetClient().Eval(
		ctx,
		redisKeyConcurrencyAcquireScript,
		[]string{key},
		limit,
		keyConcurrencyTTL.Milliseconds(),
	).Result()
	if err != nil {
		return false, err
	}
	switch v := result.(type) {
	case int64:
		return v == 1, nil
	case int:
		return v == 1, nil
	default:
		return false, fmt.Errorf("unexpected redis acquire result type %T", result)
	}
}

func (l *RedisKeyConcurrencyLimiter) Release(ctx context.Context, keyID int64) {
	if l == nil || l.redis == nil || !l.redis.IsEnabled() || keyID <= 0 {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	key := l.redis.WrapKey(fmt.Sprintf("channel_key:inflight:%d", keyID))
	_, _ = l.redis.GetClient().Eval(
		ctx,
		redisKeyConcurrencyReleaseScript,
		[]string{key},
	).Result()
}

func (l *RedisKeyConcurrencyLimiter) GetInFlight(ctx context.Context, keyID int64) int {
	if l == nil || l.redis == nil || !l.redis.IsEnabled() || keyID <= 0 {
		return 0
	}
	if ctx == nil {
		ctx = context.Background()
	}
	key := l.redis.WrapKey(fmt.Sprintf("channel_key:inflight:%d", keyID))
	val, err := l.redis.GetClient().Get(ctx, key).Int()
	if err != nil || val < 0 {
		return 0
	}
	return val
}

// NewKeyConcurrencyLimiter prefers Redis when available, else memory.
func NewKeyConcurrencyLimiter(redisCache *cache.RedisCache) KeyConcurrencyLimiter {
	if limiter := NewRedisKeyConcurrencyLimiter(redisCache); limiter != nil {
		return limiter
	}
	return NewMemoryKeyConcurrencyLimiter()
}

const (
	keyRateTokenScale     = int64(1_000_000)
	keyRateBucketTTL      = 2 * time.Minute
	keyRateRefillPeriod   = time.Minute
	keyRateBucketCapacity = 1.0
	globalImageRateBucket = "upstream:image:global_rpm_bucket"
)

// KeyRateLimiter strictly smooths requests per channel key. rateLimit is RPM;
// rateLimit<=0 means unlimited. Calls wait for a token or context cancellation.
type KeyRateLimiter interface {
	TryConsume(ctx context.Context, keyID int64, rateLimit int) (bool, error)
}

// GlobalUpstreamRateLimiter smooths all image upstream HTTP requests in one
// process and relies on Redis-backed storage for cross-process coordination.
type GlobalUpstreamRateLimiter struct {
	consumer  tokenBucketConsumer
	rateLimit int
}

type tokenBucketConsumer interface {
	tryConsumeBucket(ctx context.Context, bucketKey string, rateLimit int) (bool, error)
}

type memoryKeyRateBucket struct {
	lastRefill time.Time
	tokens     float64
	mu         sync.Mutex
}

// keyRateLimiterCleanupInterval/IdleTTL：与 middleware/rate_limiter.go 一致，
// 定期清理不活跃的本地令牌桶/锁条目，避免进程内 Map 随 key 数量无限增长。
const (
	keyRateLimiterCleanupInterval = 10 * time.Minute
	keyRateLimiterIdleTTL         = 30 * time.Minute
)

// MemoryKeyRateLimiter is process-local (tests / Redis disabled).
type MemoryKeyRateLimiter struct {
	now         func() time.Time
	wait        func(context.Context, time.Duration) error
	buckets     sync.Map
	cleanupOnce sync.Once
	stopCleanup chan struct{}
	closeOnce   sync.Once
}

func NewMemoryKeyRateLimiter() *MemoryKeyRateLimiter {
	l := newMemoryKeyRateLimiter(time.Now, waitForKeyRateToken)
	l.startCleanup()
	return l
}

func newMemoryKeyRateLimiter(now func() time.Time, wait func(context.Context, time.Duration) error) *MemoryKeyRateLimiter {
	if now == nil {
		now = time.Now
	}
	if wait == nil {
		wait = waitForKeyRateToken
	}
	return &MemoryKeyRateLimiter{now: now, wait: wait}
}

// startCleanup 定期清理不活跃的令牌桶（幂等）。
func (l *MemoryKeyRateLimiter) startCleanup() {
	l.cleanupOnce.Do(func() {
		l.stopCleanup = make(chan struct{})
		// 设计考量 (Task 1.6 并发安全治理)：
		// 后台定期清理不活跃令牌桶使用 safego 受控启动并注入 Panic 恢复兜底，
		// 防止字典并发迭代或锁操作异常导致主服务崩溃。
		safego.GoWithRecover(context.Background(), "routing.memory_key_rate_cleanup", func(wCtx context.Context) {
			ticker := time.NewTicker(keyRateLimiterCleanupInterval)
			defer ticker.Stop()
			for {
				select {
				case <-l.stopCleanup:
					return
				case <-ticker.C:
					now := l.now()
					l.buckets.Range(func(key, value interface{}) bool {
						bucket, ok := value.(*memoryKeyRateBucket)
						if !ok {
							return true
						}
						bucket.mu.Lock()
						// lastRefill 为零值表示刚创建尚未使用，同样视为不活跃。
						if bucket.lastRefill.IsZero() || now.Sub(bucket.lastRefill) > keyRateLimiterIdleTTL {
							l.buckets.Delete(key)
						}
						bucket.mu.Unlock()
						return true
					})
				}
			}
		}, nil)
	})
}

// Close 停止清理协程（幂等）。
func (l *MemoryKeyRateLimiter) Close() {
	if l == nil {
		return
	}
	l.closeOnce.Do(func() {
		if l.stopCleanup != nil {
			close(l.stopCleanup)
		}
	})
}

// keyRateMaxWait 是允许为等待令牌而排队的最长时间。
//
// 超过该时长立即返回 allowed=false，让调用方（routing.Pool）产出明确的
// RateLimitError，而不是把请求无限期挂在令牌桶上静默排队。
const keyRateMaxWait = 500 * time.Millisecond

func (l *MemoryKeyRateLimiter) TryConsume(ctx context.Context, keyID int64, rateLimit int) (bool, error) {
	if l == nil || keyID <= 0 || rateLimit <= 0 {
		return true, nil
	}
	return l.tryConsumeBucket(ctx, fmt.Sprintf("channel_key:%d", keyID), rateLimit)
}

func (l *MemoryKeyRateLimiter) tryConsumeBucket(ctx context.Context, bucketKey string, rateLimit int) (bool, error) {
	if l == nil || bucketKey == "" || rateLimit <= 0 {
		return true, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	bucketValue, _ := l.buckets.LoadOrStore(bucketKey, &memoryKeyRateBucket{tokens: keyRateBucketCapacity})
	bucket := bucketValue.(*memoryKeyRateBucket)
	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	for {
		now := l.now()
		if bucket.lastRefill.IsZero() {
			bucket.lastRefill = now
		}
		elapsed := now.Sub(bucket.lastRefill)
		if elapsed > 0 {
			refill := elapsed.Seconds() * float64(rateLimit) / keyRateRefillPeriod.Seconds()
			bucket.tokens = math.Min(keyRateBucketCapacity, bucket.tokens+refill)
			bucket.lastRefill = now
		}
		if bucket.tokens >= 1 {
			bucket.tokens--
			return true, nil
		}
		missing := 1 - bucket.tokens
		wait := time.Duration(math.Ceil(missing * keyRateRefillPeriod.Seconds() / float64(rateLimit) * float64(time.Second)))
		if wait < time.Microsecond {
			wait = time.Microsecond
		}
		if wait > keyRateMaxWait {
			return false, nil
		}
		if err := l.wait(ctx, wait); err != nil {
			return false, err
		}
	}
}

// keyLockEntry 本地每 key 互斥锁条目，记录最后使用时间供定期清理。
type keyLockEntry struct {
	mu       sync.Mutex
	lastUsed time.Time
}

// RedisKeyRateLimiter uses a capacity-1 token bucket shared by all processes.
// A per-process/key mutex prevents thousands of local workers from polling Redis
// for the same token; cross-process arbitration remains atomic in Lua.
type RedisKeyRateLimiter struct {
	redis       *cache.RedisCache
	keyLocks    sync.Map
	waitToken   func(context.Context, time.Duration) error
	cleanupOnce sync.Once
	stopCleanup chan struct{}
	closeOnce   sync.Once
}

func NewRedisKeyRateLimiter(redisCache *cache.RedisCache) *RedisKeyRateLimiter {
	if redisCache == nil || !redisCache.IsEnabled() {
		return nil
	}
	l := &RedisKeyRateLimiter{redis: redisCache, waitToken: waitForKeyRateToken}
	l.startCleanup()
	return l
}

// startCleanup 定期清理不活跃的本地锁条目（幂等）。
// 删除时若某 goroutine 正持有旧 entry 的锁，删除仅移除引用；新请求会创建
// 新 entry，旧 entry 的锁最终释放，不会死锁。
func (l *RedisKeyRateLimiter) startCleanup() {
	l.cleanupOnce.Do(func() {
		l.stopCleanup = make(chan struct{})
		// 设计考量 (Task 1.6 并发安全治理)：
		// 后台定期清理不活跃本地锁使用 safego 受控启动并注入 Panic 恢复兜底，
		// 防止字典并发迭代或锁操作异常导致主服务崩溃。
		safego.GoWithRecover(context.Background(), "routing.redis_key_rate_cleanup", func(wCtx context.Context) {
			ticker := time.NewTicker(keyRateLimiterCleanupInterval)
			defer ticker.Stop()
			for {
				select {
				case <-l.stopCleanup:
					return
				case <-ticker.C:
					now := time.Now()
					l.keyLocks.Range(func(key, value interface{}) bool {
						entry, ok := value.(*keyLockEntry)
						if !ok {
							return true
						}
						entry.mu.Lock()
						if now.Sub(entry.lastUsed) > keyRateLimiterIdleTTL {
							l.keyLocks.Delete(key)
						}
						entry.mu.Unlock()
						return true
					})
				}
			}
		}, nil)
	})
}

// Close 停止清理协程（幂等）。
func (l *RedisKeyRateLimiter) Close() {
	if l == nil {
		return
	}
	l.closeOnce.Do(func() {
		if l.stopCleanup != nil {
			close(l.stopCleanup)
		}
	})
}

const redisKeyRPMConsumeScript = `
local key = KEYS[1]
local rpm = tonumber(ARGV[1])
local scale = tonumber(ARGV[2])
local ttl = tonumber(ARGV[3])
local redis_time = redis.call('TIME')
local now_us = tonumber(redis_time[1]) * 1000000 + tonumber(redis_time[2])
local state = redis.call('HMGET', key, 'tokens', 'last_us')
local tokens = tonumber(state[1])
local last_us = tonumber(state[2])
if tokens == nil or last_us == nil then
  tokens = scale
  last_us = now_us
end
local elapsed_us = math.max(0, now_us - last_us)
tokens = math.min(scale, tokens + (elapsed_us * rpm * scale / 60000000))
local allowed = 0
local retry_us = 0
if tokens >= scale then
  tokens = tokens - scale
  allowed = 1
else
  retry_us = math.ceil((scale - tokens) * 60000000 / (rpm * scale))
end
redis.call('HSET', key, 'tokens', tokens, 'last_us', now_us)
redis.call('PEXPIRE', key, ttl)
return {allowed, retry_us, now_us}
`

// TryConsume 与内存实现保持同一语义：排队等待超过 keyRateMaxWait 立即返回 allowed=false，
// 让 routing.Pool 产出明确的 RateLimitError。
//
// 历史缺陷（INK-058 实测发现）：本方法此前直接走无上限的等待循环，RPM 打满时请求会**静默排队**
// 到下一个令牌（rate_limit=1 时最长约 60 秒）再返回成功，限流语义在启用 Redis 的生产配置下失效。
func (l *RedisKeyRateLimiter) TryConsume(ctx context.Context, keyID int64, rateLimit int) (bool, error) {
	if l == nil || l.redis == nil || !l.redis.IsEnabled() || keyID <= 0 || rateLimit <= 0 {
		return true, nil
	}
	return l.tryConsumeBucketWithin(ctx, fmt.Sprintf("channel_key:rpm_bucket:%d", keyID), rateLimit, keyRateMaxWait)
}

// tryConsumeBucket 是无上限等待版本，供全局图片限流器的 Wait 语义使用（阻塞到拿到令牌为止）。
func (l *RedisKeyRateLimiter) tryConsumeBucket(ctx context.Context, bucketKey string, rateLimit int) (bool, error) {
	allowed, _, err := l.tryConsumeBucketAt(ctx, bucketKey, rateLimit)
	return allowed, err
}

func (l *RedisKeyRateLimiter) tryConsumeBucketWithin(ctx context.Context, bucketKey string, rateLimit int, maxWait time.Duration) (bool, error) {
	allowed, _, err := l.tryConsumeBucketWithinAt(ctx, bucketKey, rateLimit, maxWait)
	return allowed, err
}

func (l *RedisKeyRateLimiter) tryConsumeBucketAt(ctx context.Context, bucketKey string, rateLimit int) (bool, int64, error) {
	return l.tryConsumeBucketWithinAt(ctx, bucketKey, rateLimit, 0)
}

// tryConsumeBucketWithinAt 是令牌桶消费的核心实现；maxWait<=0 表示允许无限等待。
func (l *RedisKeyRateLimiter) tryConsumeBucketWithinAt(ctx context.Context, bucketKey string, rateLimit int, maxWait time.Duration) (bool, int64, error) {
	if l == nil || l.redis == nil || !l.redis.IsEnabled() || bucketKey == "" || rateLimit <= 0 {
		return true, 0, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	lockValue, _ := l.keyLocks.LoadOrStore(bucketKey, &keyLockEntry{lastUsed: time.Now()})
	entry := lockValue.(*keyLockEntry)
	entry.mu.Lock()
	entry.lastUsed = time.Now()
	defer entry.mu.Unlock()

	key := l.redis.WrapKey(bucketKey)
	waited := time.Duration(0)
	for {
		result, err := l.redis.GetClient().Eval(
			ctx,
			redisKeyRPMConsumeScript,
			[]string{key},
			rateLimit,
			keyRateTokenScale,
			keyRateBucketTTL.Milliseconds(),
		).Result()
		if err != nil {
			return false, 0, err
		}
		values, ok := result.([]interface{})
		if !ok || len(values) < 3 {
			return false, 0, fmt.Errorf("unexpected redis rpm token bucket result type %T", result)
		}
		if redisRateInt64(values[0]) == 1 {
			return true, redisRateInt64(values[2]), nil
		}
		retry := time.Duration(redisRateInt64(values[1])) * time.Microsecond
		if retry < time.Microsecond {
			retry = time.Microsecond
		}
		// 有上限的调用方（Pool 的密钥级 RPM 闸门）不允许把请求挂在桶上静默排队：
		// 预计等待超过上限就立即返回 false，交由 Pool 产出明确的限流错误。
		if maxWait > 0 && waited+retry > maxWait {
			return false, 0, nil
		}
		wait := l.waitToken
		if wait == nil {
			wait = waitForKeyRateToken
		}
		if err := wait(ctx, retry); err != nil {
			return false, 0, err
		}
		waited += retry
	}
}

func waitForKeyRateToken(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func redisRateInt64(value interface{}) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case string:
		var parsed int64
		_, _ = fmt.Sscan(v, &parsed)
		return parsed
	default:
		return 0
	}
}

// NewKeyRateLimiter prefers Redis when available, else memory.
func NewKeyRateLimiter(redisCache *cache.RedisCache) KeyRateLimiter {
	if limiter := NewRedisKeyRateLimiter(redisCache); limiter != nil {
		return limiter
	}
	return NewMemoryKeyRateLimiter()
}

// NewGlobalUpstreamRateLimiter builds the application-wide image request
// limiter. A positive rateLimit is required; production defaults it to 3600.
func NewGlobalUpstreamRateLimiter(redisCache *cache.RedisCache, rateLimit int) *GlobalUpstreamRateLimiter {
	if rateLimit <= 0 {
		return nil
	}
	var consumer tokenBucketConsumer = NewMemoryKeyRateLimiter()
	if limiter := NewRedisKeyRateLimiter(redisCache); limiter != nil {
		consumer = limiter
	}
	return &GlobalUpstreamRateLimiter{consumer: consumer, rateLimit: rateLimit}
}

// Wait blocks until the global image request bucket grants one token.
func (l *GlobalUpstreamRateLimiter) Wait(ctx context.Context) error {
	if l == nil || l.consumer == nil || l.rateLimit <= 0 {
		return nil
	}
	_, err := l.consumer.tryConsumeBucket(ctx, globalImageRateBucket, l.rateLimit)
	return err
}
