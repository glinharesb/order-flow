package cache

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"
)

// Cache defines the interface for caching operations.
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
}

// ErrCacheMiss is returned when a key is not found in cache.
var ErrCacheMiss = fmt.Errorf("cache miss")

// Key builders for consistent cache key generation.
const keyPrefix = "order"

func OrderKey(id string) string {
	return fmt.Sprintf("%s:order:%s", keyPrefix, id)
}

func OrderListKey(filterHash string) string {
	return fmt.Sprintf("%s:orders:list:%s", keyPrefix, filterHash)
}

// HashFilter produces a short hash for a filter string to use as cache key suffix.
func HashFilter(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h[:8])
}
