// Package cache 提供Redis缓存功能
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"InkLore/internal/platform/config"
	"InkLore/internal/platform/logger"

	"github.com/redis/go-redis/v9"
)

var (
	// ErrCacheMiss 缓存未命中
	ErrCacheMiss = errors.New("cache miss")
	// ErrCacheDisabled 缓存已禁用
	ErrCacheDisabled = errors.New("cache disabled")
)

const redisDeleteIfValueScript = `
if redis.call('GET', KEYS[1]) == ARGV[1] then
  return redis.call('DEL', KEYS[1])
end
return 0
`

// RedisCache Redis缓存客户端
type RedisCache struct {
	client  *redis.Client
	prefix  string
	enabled bool
}

// NewRedisCache 创建Redis缓存客户端
func NewRedisCache(cfg *config.RedisConfig) (*RedisCache, error) {
	if cfg == nil || !cfg.Enabled {
		logger.Info("Redis缓存已禁用")
		return &RedisCache{enabled: false}, nil
	}

	client := NewRedisClient(cfg)

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis连接失败: %w", err)
	}

	logger.Info("Redis连接成功", "get_redis_addr", cfg.GetRedisAddr())
	return &RedisCache{
		client:  client,
		enabled: true,
		prefix:  "acp:", // AI Content Platform 前缀
	}, nil
}

// NewRedisClient 按统一配置创建 Redis 客户端。调用方负责 Ping 和 Close。
func NewRedisClient(cfg *config.RedisConfig) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:         cfg.GetRedisAddr(),
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		MaxRetries:   cfg.MaxRetries,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})
}

// IsEnabled 检查缓存是否启用
func (c *RedisCache) IsEnabled() bool {
	return c.enabled && c.client != nil
}

// Get 获取字符串值
func (c *RedisCache) Get(ctx context.Context, key string) (string, error) {
	if !c.IsEnabled() {
		return "", ErrCacheDisabled
	}

	val, err := c.client.Get(ctx, c.prefix+key).Result()
	if err == redis.Nil {
		return "", ErrCacheMiss
	}
	return val, err
}

// GetDel 获取字符串值并原子删除键。
func (c *RedisCache) GetDel(ctx context.Context, key string) (string, error) {
	if !c.IsEnabled() {
		return "", ErrCacheDisabled
	}

	val, err := c.client.GetDel(ctx, c.prefix+key).Result()
	if err == redis.Nil {
		return "", ErrCacheMiss
	}
	return val, err
}

// Set 设置字符串值
func (c *RedisCache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	if !c.IsEnabled() {
		return ErrCacheDisabled
	}

	return c.client.Set(ctx, c.prefix+key, value, expiration).Err()
}

// Delete 删除键
func (c *RedisCache) Delete(ctx context.Context, keys ...string) error {
	if !c.IsEnabled() {
		return ErrCacheDisabled
	}

	prefixedKeys := make([]string, len(keys))
	for i, k := range keys {
		prefixedKeys[i] = c.prefix + k
	}
	return c.client.Del(ctx, prefixedKeys...).Err()
}

// DeleteIfValue 仅在当前值匹配时删除键，适合释放短租约锁。
func (c *RedisCache) DeleteIfValue(ctx context.Context, key string, value string) error {
	if !c.IsEnabled() {
		return ErrCacheDisabled
	}

	return c.client.Eval(ctx, redisDeleteIfValueScript, []string{c.prefix + key}, value).Err()
}

// Exists 检查键是否存在
func (c *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	if !c.IsEnabled() {
		return false, ErrCacheDisabled
	}

	result, err := c.client.Exists(ctx, c.prefix+key).Result()
	return result > 0, err
}

// GetJSON 获取JSON值并反序列化
func (c *RedisCache) GetJSON(ctx context.Context, key string, dest interface{}) error {
	if !c.IsEnabled() {
		return ErrCacheDisabled
	}

	val, err := c.Get(ctx, key)
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(val), dest)
}

// SetJSON 序列化并设置JSON值
func (c *RedisCache) SetJSON(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	if !c.IsEnabled() {
		return ErrCacheDisabled
	}

	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return c.Set(ctx, key, data, expiration)
}

// MGet 批量获取
func (c *RedisCache) MGet(ctx context.Context, keys ...string) ([]interface{}, error) {
	if !c.IsEnabled() {
		return nil, ErrCacheDisabled
	}

	prefixedKeys := make([]string, len(keys))
	for i, k := range keys {
		prefixedKeys[i] = c.prefix + k
	}
	return c.client.MGet(ctx, prefixedKeys...).Result()
}

// MSet 批量设置（注意：不支持过期时间，需要额外调用Expire）
func (c *RedisCache) MSet(ctx context.Context, values map[string]interface{}) error {
	if !c.IsEnabled() {
		return ErrCacheDisabled
	}

	prefixedValues := make(map[string]interface{})
	for k, v := range values {
		prefixedValues[c.prefix+k] = v
	}
	return c.client.MSet(ctx, prefixedValues).Err()
}

// Incr 自增
func (c *RedisCache) Incr(ctx context.Context, key string) (int64, error) {
	if !c.IsEnabled() {
		return 0, ErrCacheDisabled
	}

	return c.client.Incr(ctx, c.prefix+key).Result()
}

// IncrBy 自增指定值
func (c *RedisCache) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	if !c.IsEnabled() {
		return 0, ErrCacheDisabled
	}

	return c.client.IncrBy(ctx, c.prefix+key, value).Result()
}

// Decr 自减
func (c *RedisCache) Decr(ctx context.Context, key string) (int64, error) {
	if !c.IsEnabled() {
		return 0, ErrCacheDisabled
	}

	return c.client.Decr(ctx, c.prefix+key).Result()
}

// Expire 设置过期时间
func (c *RedisCache) Expire(ctx context.Context, key string, expiration time.Duration) error {
	if !c.IsEnabled() {
		return ErrCacheDisabled
	}

	return c.client.Expire(ctx, c.prefix+key, expiration).Err()
}

// TTL 获取剩余过期时间
func (c *RedisCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	if !c.IsEnabled() {
		return 0, ErrCacheDisabled
	}

	return c.client.TTL(ctx, c.prefix+key).Result()
}

// SetNX 如果不存在则设置（用于分布式锁）
func (c *RedisCache) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	if !c.IsEnabled() {
		return false, ErrCacheDisabled
	}

	return c.client.SetNX(ctx, c.prefix+key, value, expiration).Result()
}

// HSet Hash设置
func (c *RedisCache) HSet(ctx context.Context, key string, values ...interface{}) error {
	if !c.IsEnabled() {
		return ErrCacheDisabled
	}

	return c.client.HSet(ctx, c.prefix+key, values...).Err()
}

// HGet Hash获取
func (c *RedisCache) HGet(ctx context.Context, key, field string) (string, error) {
	if !c.IsEnabled() {
		return "", ErrCacheDisabled
	}

	val, err := c.client.HGet(ctx, c.prefix+key, field).Result()
	if err == redis.Nil {
		return "", ErrCacheMiss
	}
	return val, err
}

// HGetAll 获取Hash所有字段
func (c *RedisCache) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	if !c.IsEnabled() {
		return nil, ErrCacheDisabled
	}

	return c.client.HGetAll(ctx, c.prefix+key).Result()
}

// HDel 删除Hash字段
func (c *RedisCache) HDel(ctx context.Context, key string, fields ...string) error {
	if !c.IsEnabled() {
		return ErrCacheDisabled
	}

	return c.client.HDel(ctx, c.prefix+key, fields...).Err()
}

// SAdd 集合添加
func (c *RedisCache) SAdd(ctx context.Context, key string, members ...interface{}) error {
	if !c.IsEnabled() {
		return ErrCacheDisabled
	}

	return c.client.SAdd(ctx, c.prefix+key, members...).Err()
}

// SMembers 获取集合成员
func (c *RedisCache) SMembers(ctx context.Context, key string) ([]string, error) {
	if !c.IsEnabled() {
		return nil, ErrCacheDisabled
	}

	return c.client.SMembers(ctx, c.prefix+key).Result()
}

// SRem 删除集合成员
func (c *RedisCache) SRem(ctx context.Context, key string, members ...interface{}) error {
	if !c.IsEnabled() {
		return ErrCacheDisabled
	}

	return c.client.SRem(ctx, c.prefix+key, members...).Err()
}

// DeleteByPattern 按模式删除键
func (c *RedisCache) DeleteByPattern(ctx context.Context, pattern string) error {
	if !c.IsEnabled() {
		return ErrCacheDisabled
	}

	iter := c.client.Scan(ctx, 0, c.prefix+pattern, 100).Iterator()
	for iter.Next(ctx) {
		if err := c.client.Del(ctx, iter.Val()).Err(); err != nil {
			return err
		}
	}
	return iter.Err()
}

// Ping 健康检查
func (c *RedisCache) Ping(ctx context.Context) error {
	if !c.IsEnabled() {
		return ErrCacheDisabled
	}

	return c.client.Ping(ctx).Err()
}

// Close 关闭连接
func (c *RedisCache) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

// Info 获取Redis信息
func (c *RedisCache) Info(ctx context.Context) (string, error) {
	if !c.IsEnabled() {
		return "", ErrCacheDisabled
	}

	return c.client.Info(ctx).Result()
}

// DBSize 获取数据库大小
func (c *RedisCache) DBSize(ctx context.Context) (int64, error) {
	if !c.IsEnabled() {
		return 0, ErrCacheDisabled
	}

	return c.client.DBSize(ctx).Result()
}

// FlushDB 清空当前数据库（慎用）
func (c *RedisCache) FlushDB(ctx context.Context) error {
	if !c.IsEnabled() {
		return ErrCacheDisabled
	}

	return c.client.FlushDB(ctx).Err()
}

// GetClient 获取底层Redis客户端（高级用途）
func (c *RedisCache) GetClient() *redis.Client {
	return c.client
}

// PoolStatsSnapshot 是连接池计数的中立快照：调用方（如 middleware）不应
// 依赖 go-redis 具体类型，跨类型差值统一用 int64 防止无符号回绕。
type PoolStatsSnapshot struct {
	TotalConns     int64
	IdleConns      int64
	StaleConns     int64
	Hits           int64
	Misses         int64
	Timeouts       int64
	WaitCount      int64
	WaitDurationNs int64
}

// PoolStats 返回底层 Redis 连接池快照，供慢请求观测使用。
func (c *RedisCache) PoolStats() *PoolStatsSnapshot {
	if c == nil || c.client == nil {
		return nil
	}
	stats := c.client.PoolStats()
	if stats == nil {
		return nil
	}
	return &PoolStatsSnapshot{
		TotalConns:     int64(stats.TotalConns),
		IdleConns:      int64(stats.IdleConns),
		StaleConns:     int64(stats.StaleConns),
		Hits:           int64(stats.Hits),
		Misses:         int64(stats.Misses),
		Timeouts:       int64(stats.Timeouts),
		WaitCount:      int64(stats.WaitCount),
		WaitDurationNs: stats.WaitDurationNs,
	}
}

// GetWithFallback 优雅降级获取
func (c *RedisCache) GetWithFallback(ctx context.Context, key string, fallback func() (interface{}, error), expiration time.Duration) (interface{}, error) {
	// 尝试从缓存获取
	if c.IsEnabled() {
		var result interface{}
		if err := c.GetJSON(ctx, key, &result); err == nil {
			return result, nil
		}
	}

	// 缓存未命中或禁用，执行fallback
	value, err := fallback()
	if err != nil {
		return nil, err
	}

	// 尝试写入缓存（忽略错误）
	if c.IsEnabled() {
		_ = c.SetJSON(ctx, key, value, expiration)
	}

	return value, nil
}

// GetStringWithFallback 字符串类型的优雅降级获取
func (c *RedisCache) GetStringWithFallback(ctx context.Context, key string, fallback func() (string, error), expiration time.Duration) (string, error) {
	// 尝试从缓存获取
	if c.IsEnabled() {
		if val, err := c.Get(ctx, key); err == nil {
			return val, nil
		}
	}

	// 缓存未命中或禁用，执行fallback
	value, err := fallback()
	if err != nil {
		return "", err
	}

	// 尝试写入缓存（忽略错误）
	if c.IsEnabled() {
		_ = c.Set(ctx, key, value, expiration)
	}

	return value, nil
}

// Pipeline 创建Pipeline
func (c *RedisCache) Pipeline() redis.Pipeliner {
	if !c.IsEnabled() {
		return nil
	}
	return c.client.Pipeline()
}

// Prefix 获取前缀
func (c *RedisCache) Prefix() string {
	return c.prefix
}

// BuildKey 构建完整键名
func (c *RedisCache) BuildKey(parts ...string) string {
	key := ""
	for i, part := range parts {
		if i > 0 {
			key += ":"
		}
		key += part
	}
	return key
}

// Stats 获取缓存统计信息
func (c *RedisCache) Stats(ctx context.Context) (*CacheStats, error) {
	if !c.IsEnabled() {
		return &CacheStats{Enabled: false}, nil
	}

	info, err := c.client.Info(ctx, "stats", "memory").Result()
	if err != nil {
		return nil, err
	}

	dbSize, _ := c.client.DBSize(ctx).Result()

	return &CacheStats{
		Enabled: true,
		Keys:    dbSize,
		Info:    info,
	}, nil
}

// CacheStats 缓存统计
type CacheStats struct {
	Info    string `json:"info,omitempty"`
	Keys    int64  `json:"keys"`
	Enabled bool   `json:"enabled"`
}

// WrapKey 包装键名（添加前缀）
func (c *RedisCache) WrapKey(key string) string {
	return c.prefix + key
}

// UnwrapKey 解包键名（移除前缀）
func (c *RedisCache) UnwrapKey(key string) string {
	if len(key) > len(c.prefix) && key[:len(c.prefix)] == c.prefix {
		return key[len(c.prefix):]
	}
	return key
}

// String 实现fmt.Stringer接口
func (c *RedisCache) String() string {
	if c.IsEnabled() {
		return fmt.Sprintf("RedisCache[enabled, prefix=%s]", c.prefix)
	}
	return "RedisCache[disabled]"
}
