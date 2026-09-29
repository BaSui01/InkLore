// Package cache 提供中立的缓存基础设施支持
package cache

import (
	"InkLore/internal/platform/logger"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

// ServiceCache 业务层缓存管理器。
// Redis 启用时只以 Redis 为准，避免多实例环境中读到本机陈旧内存缓存。
type ServiceCache struct {
	redis  *RedisCache
	memory *MemoryCache
	mu     sync.RWMutex
	// localFallbackOnRedisError 为 true 时，Redis 已启用但读写失败会回退本机内存并告警（INK-014）。
	// 默认关闭：多数调用方宁可用未命中也不要跨实例的陈旧值。
	localFallbackOnRedisError bool
}

// NewServiceCache 创建业务缓存管理器
func NewServiceCache(redis *RedisCache) *ServiceCache {
	return &ServiceCache{
		redis:  redis,
		memory: NewMemoryCache(),
	}
}

// EnableLocalFallbackOnRedisError 开启「Redis 故障时回退本机内存」策略。
//
// 适用场景：缓存只用于省算力（例如 Prompt 稳定前缀），陈旧值的代价远小于
// 因缓存不可用而拖慢或阻断请求；需要跨实例强一致的调用方不要开启。
func (c *ServiceCache) EnableLocalFallbackOnRedisError() {
	if c != nil {
		c.localFallbackOnRedisError = true
	}
}

// IsRedisEnabled 检查 Redis 是否可用
func (c *ServiceCache) IsRedisEnabled() bool {
	return c.redis != nil && c.redis.IsEnabled()
}

// ========================================
// 通用缓存操作
// ========================================

// Get 获取缓存值。Redis 启用时不再回退到本机内存，避免跨实例缓存失效不一致。
func (c *ServiceCache) Get(ctx context.Context, key string) (string, bool) {
	if c.IsRedisEnabled() {
		val, err := c.redis.Get(ctx, key)
		if err == nil {
			return val, true
		}
		if err != ErrCacheMiss && !errors.Is(err, context.Canceled) {
			logger.ErrorContext(ctx, "Redis读取失败", "key", key, "error", err)
		}
		if c.localFallbackOnRedisError {
			if val, ok := c.memory.Get(key); ok {
				if str, ok := val.(string); ok {
					logger.WarnContext(ctx, "Redis 不可用，已回退本机内存缓存", "key", key)
					return str, true
				}
			}
		}
		return "", false
	}

	// 降级到内存缓存
	if val, ok := c.memory.Get(key); ok {
		if str, ok := val.(string); ok {
			return str, true
		}
	}
	return "", false
}

// Set 设置缓存值。Redis 启用时只写 Redis；Redis 禁用时使用本机内存。
func (c *ServiceCache) Set(ctx context.Context, key string, value string, ttl time.Duration) {
	if c.IsRedisEnabled() {
		if err := c.redis.Set(ctx, key, value, ttl); err != nil && !errors.Is(err, context.Canceled) {
			logger.ErrorContext(ctx, "Redis写入失败", "key", key, "error", err)
			if c.localFallbackOnRedisError {
				c.memory.Set(key, value, ttl)
				logger.WarnContext(ctx, "Redis 写入失败，已改存本机内存缓存", "key", key)
			}
			return
		}
		if c.localFallbackOnRedisError {
			// 同时存本机：Redis 抖动期间读取仍可命中。
			c.memory.Set(key, value, ttl)
		}
		return
	}

	c.memory.Set(key, value, ttl)
}

// Delete 删除缓存
func (c *ServiceCache) Delete(ctx context.Context, keys ...string) {
	for _, key := range keys {
		c.memory.Delete(key)
	}

	if c.IsRedisEnabled() {
		if err := c.redis.Delete(ctx, keys...); err != nil {
			// 配置可能已提交，失效故障不能静默；保持旧调用签名，不伪装数据库回滚。
			logger.ErrorContext(ctx, "Redis缓存失效失败", "key_count", len(keys), "error", logger.SanitizeError(err))
		}
	}
}

// DeleteByPrefix 删除指定前缀下的缓存项。
func (c *ServiceCache) DeleteByPrefix(ctx context.Context, prefix string) {
	if c == nil || prefix == "" {
		return
	}
	if c.memory != nil {
		c.memory.DeleteByPrefix(prefix)
	}

	if c.IsRedisEnabled() {
		if err := c.redis.DeleteByPattern(ctx, prefix+"*"); err != nil {
			logger.ErrorContext(ctx, "Redis前缀缓存失效失败", "error", logger.SanitizeError(err))
		}
	}
}

// GetJSON 获取 JSON 缓存
func (c *ServiceCache) GetJSON(ctx context.Context, key string, dest interface{}) bool {
	val, ok := c.Get(ctx, key)
	if !ok {
		return false
	}
	if err := json.Unmarshal([]byte(val), dest); err != nil {
		return false
	}
	return true
}

// SetJSON 设置 JSON 缓存
func (c *ServiceCache) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	c.Set(ctx, key, string(data), ttl)
	return nil
}

// ========================================
// 统计信息
// ========================================

// ServiceCacheStats 缓存统计
type ServiceCacheStats struct {
	RedisEnabled   bool `json:"redis_enabled"`
	MemoryCacheLen int  `json:"memory_cache_len"`
}

// GetStats 获取缓存统计
func (c *ServiceCache) GetStats() ServiceCacheStats {
	return ServiceCacheStats{
		RedisEnabled:   c.IsRedisEnabled(),
		MemoryCacheLen: c.memory.Count(),
	}
}

// Close 停止业务缓存内部的内存清理协程（幂等）。
// Redis 连接由 RedisCache.Close 统一关闭，此处只回收本机内存缓存协程。
func (c *ServiceCache) Close() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.memory != nil {
		c.memory.Close()
	}
}
