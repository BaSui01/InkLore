package cache

import (
	"context"
	"time"

	"InkLore/internal/platform/config"

	"github.com/redis/go-redis/v9"
)

// redisProbeTimeout 限制单次就绪探测耗时，保证 /readyz 不会被慢依赖拖垮。
const redisProbeTimeout = 2 * time.Second

// RedisHealthChecker 为就绪探针提供一个**惰性**的 Redis 客户端。
//
// 与 NewRedisCache 的关键区别：构造时不建立连接、不返回错误。
// 就绪探针要回答的是“此刻是否可用”，如果构造即失败，进程根本起不来，
// 探针也就失去了意义；惰性客户端能让依赖恢复后 /readyz 自动转回就绪（INK-036）。
type RedisHealthChecker struct {
	client *redis.Client
}

// NewRedisHealthChecker 按配置构造探测客户端；未启用 Redis 时返回不探测的实例。
func NewRedisHealthChecker(cfg *config.RedisConfig) *RedisHealthChecker {
	if cfg == nil || !cfg.Enabled {
		return &RedisHealthChecker{}
	}
	return &RedisHealthChecker{client: NewRedisClient(cfg)}
}

// Configured 报告是否配置了 Redis；未配置时探针应报告 disabled 而不是 unreachable。
func (h *RedisHealthChecker) Configured() bool {
	return h != nil && h.client != nil
}

// Ping 探测连通性。未配置时直接返回 nil（视为无此依赖）。
func (h *RedisHealthChecker) Ping(ctx context.Context) error {
	if !h.Configured() {
		return nil
	}
	probeCtx, cancel := context.WithTimeout(ctx, redisProbeTimeout)
	defer cancel()
	return h.client.Ping(probeCtx).Err()
}

// Close 释放连接池；未配置时为无操作。
func (h *RedisHealthChecker) Close() error {
	if !h.Configured() {
		return nil
	}
	return h.client.Close()
}
