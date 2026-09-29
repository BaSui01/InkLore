package cache

import (
	"sync"
	"time"
)

// CacheItem 缓存项
type CacheItem struct {
	Value      interface{}
	Expiration int64
}

// IsExpired 检查是否过期
func (item *CacheItem) IsExpired() bool {
	if item.Expiration == 0 {
		return false
	}
	return time.Now().UnixNano() > item.Expiration
}

// MemoryCache 内存缓存
type MemoryCache struct {
	items     map[string]*CacheItem
	mu        sync.RWMutex
	stop      chan struct{}
	closeOnce sync.Once
}

// NewMemoryCache 创建内存缓存实例
func NewMemoryCache() *MemoryCache {
	cache := &MemoryCache{
		items: make(map[string]*CacheItem),
		stop:  make(chan struct{}),
	}

	// 启动清理协程
	go cache.startCleanup()

	return cache
}

// Set 设置缓存项
func (c *MemoryCache) Set(key string, value interface{}, duration time.Duration) {
	var expiration int64
	if duration > 0 {
		expiration = time.Now().Add(duration).UnixNano()
	}

	c.mu.Lock()
	c.items[key] = &CacheItem{
		Value:      value,
		Expiration: expiration,
	}
	c.mu.Unlock()
}

// Get 获取缓存项
func (c *MemoryCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	item, found := c.items[key]
	c.mu.RUnlock()

	if !found {
		return nil, false
	}

	if item.IsExpired() {
		c.Delete(key)
		return nil, false
	}

	return item.Value, true
}

// Delete 删除缓存项
func (c *MemoryCache) Delete(key string) {
	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()
}

func (c *MemoryCache) DeleteByPrefix(prefix string) {
	c.mu.Lock()
	for key := range c.items {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			delete(c.items, key)
		}
	}
	c.mu.Unlock()
}

// Clear 清空所有缓存
func (c *MemoryCache) Clear() {
	c.mu.Lock()
	c.items = make(map[string]*CacheItem)
	c.mu.Unlock()
}

// Count 返回缓存项数量
func (c *MemoryCache) Count() int {
	c.mu.RLock()
	count := len(c.items)
	c.mu.RUnlock()
	return count
}

// startCleanup 定期清理过期项
func (c *MemoryCache) startCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-c.stop:
			return
		case <-ticker.C:
			c.cleanup()
		}
	}
}

// Close 停止清理协程（幂等）。进程退出/应用关闭时调用。
func (c *MemoryCache) Close() {
	if c == nil {
		return
	}
	c.closeOnce.Do(func() {
		if c.stop != nil {
			close(c.stop)
		}
	})
}

// cleanup 清理过期项
func (c *MemoryCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key, item := range c.items {
		if item.IsExpired() {
			delete(c.items, key)
		}
	}
}
