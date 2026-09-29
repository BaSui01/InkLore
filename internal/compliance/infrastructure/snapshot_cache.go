package infrastructure

import (
	d "InkLore/internal/compliance/domain"
	"sync"
)

type MemorySnapshotCache struct {
	mu     sync.RWMutex
	cache  map[string]map[string]*d.Matcher // tenantID -> checksum -> Matcher
	order  map[string][]string              // tenantID -> list of checksums for LRU
	maxPer int
}

func NewMemorySnapshotCache(maxPerTenant int) *MemorySnapshotCache {
	if maxPerTenant <= 0 {
		maxPerTenant = 4
	}
	return &MemorySnapshotCache{
		cache:  make(map[string]map[string]*d.Matcher),
		order:  make(map[string][]string),
		maxPer: maxPerTenant,
	}
}

func (c *MemorySnapshotCache) Get(tenantID, checksum string) (*d.Matcher, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	tenantCache, ok := c.cache[tenantID]
	if !ok {
		return nil, false
	}
	matcher, ok := tenantCache[checksum]
	return matcher, ok
}

func (c *MemorySnapshotCache) Put(tenantID, checksum string, matcher *d.Matcher) {
	c.mu.Lock()
	defer c.mu.Unlock()

	tenantCache, ok := c.cache[tenantID]
	if !ok {
		tenantCache = make(map[string]*d.Matcher)
		c.cache[tenantID] = tenantCache
		c.order[tenantID] = make([]string, 0)
	}

	if _, exists := tenantCache[checksum]; !exists {
		if len(c.order[tenantID]) >= c.maxPer {
			oldest := c.order[tenantID][0]
			c.order[tenantID] = c.order[tenantID][1:]
			delete(tenantCache, oldest)
		}
		c.order[tenantID] = append(c.order[tenantID], checksum)
	}

	tenantCache[checksum] = matcher
}

func (c *MemorySnapshotCache) Invalidate(tenantID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.cache, tenantID)
	delete(c.order, tenantID)
}
