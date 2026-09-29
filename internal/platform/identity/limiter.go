package identity

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type memoryLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
}

func newMemoryLimiter() *memoryLimiter { return &memoryLimiter{attempts: make(map[string][]time.Time)} }

// Mirroring every attempt prevents a Redis outage from granting a fresh local budget.
// At capacity, new keys fail closed instead of evicting live limits.
func (m *memoryLimiter) Allow(_ context.Context, key string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-time.Minute)
	for k, attempts := range m.attempts {
		if len(attempts) == 0 || !attempts[len(attempts)-1].After(cutoff) {
			delete(m.attempts, k)
		}
	}
	attempts := m.attempts[key]
	i := 0
	for i < len(attempts) && !attempts[i].After(cutoff) {
		i++
	}
	attempts = attempts[i:]
	if len(attempts) >= 5 {
		m.attempts[key] = attempts
		return false, nil
	}
	if len(attempts) == 0 && len(m.attempts) >= 10000 {
		return false, nil
	}
	m.attempts[key] = append(attempts, now)
	return true, nil
}

type redisLimiter struct{ client redis.UniversalClient }

func NewRedisLoginLimiter(client redis.UniversalClient) LoginLimiter {
	if client == nil {
		return nil
	}
	return &redisLimiter{client: client}
}

var loginWindow = redis.NewScript(`
local ts=redis.call('TIME')
local now=tonumber(ts[1])*1000+math.floor(tonumber(ts[2])/1000)
redis.call('ZREMRANGEBYSCORE',KEYS[1],'-inf',now-60000)
if redis.call('ZCARD',KEYS[1]) >= 5 then return 0 end
redis.call('ZADD',KEYS[1],now,ARGV[1])
redis.call('PEXPIRE',KEYS[1],60000)
return 1
`)

func (r *redisLimiter) Allow(ctx context.Context, key string) (bool, error) {
	value, err := loginWindow.Run(ctx, r.client, []string{"inklore:r1:login:" + key}, uuid.NewString()).Int64()
	return value == 1, err
}
func (s *Service) allowLogin(ctx context.Context, key string) bool {
	local, _ := s.memory.Allow(ctx, key)
	if !local {
		return false
	}
	if s.limiter == nil {
		return true
	}
	allowed, err := s.limiter.Allow(ctx, key)
	if err != nil {
		return true
	}
	return allowed
}
