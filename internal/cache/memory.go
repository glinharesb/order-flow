package cache

import (
	"context"
	"sync"
	"time"
)

type memEntry struct {
	data      []byte
	expiresAt time.Time
}

// MemoryCache is an in-memory cache for testing and development.
type MemoryCache struct {
	mu    sync.RWMutex
	items map[string]memEntry
}

// NewMemoryCache creates a new in-memory cache.
func NewMemoryCache() *MemoryCache {
	return &MemoryCache{items: make(map[string]memEntry)}
}

func (c *MemoryCache) Get(_ context.Context, key string) ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.items[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, ErrCacheMiss
	}
	cp := make([]byte, len(entry.data))
	copy(cp, entry.data)
	return cp, nil
}

func (c *MemoryCache) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	cp := make([]byte, len(value))
	copy(cp, value)
	c.items[key] = memEntry{data: cp, expiresAt: time.Now().Add(ttl)}
	return nil
}

func (c *MemoryCache) Delete(_ context.Context, keys ...string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, key := range keys {
		delete(c.items, key)
	}
	return nil
}
