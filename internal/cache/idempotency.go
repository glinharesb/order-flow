package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// IdempotencyStore provides idempotency checks using Redis SET NX.
type IdempotencyStore struct {
	client *redis.Client
	ttl    time.Duration
}

// NewIdempotencyStore creates a new idempotency store backed by Redis.
func NewIdempotencyStore(client *redis.Client, ttl time.Duration) *IdempotencyStore {
	return &IdempotencyStore{client: client, ttl: ttl}
}

func idempotencyKey(key string) string {
	return fmt.Sprintf("order:idempotency:%s", key)
}

// CheckAndSet attempts to set the idempotency key. Returns true if the key was set
// (first time), false if it already existed (duplicate). An error is returned for
// infrastructure failures.
func (s *IdempotencyStore) CheckAndSet(ctx context.Context, key string) (bool, error) {
	err := s.client.SetArgs(ctx, idempotencyKey(key), "1", redis.SetArgs{
		Mode: "NX",
		TTL:  s.ttl,
	}).Err()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("idempotency set nx: %w", err)
	}
	return true, nil
}

// MemoryIdempotencyStore is an in-memory implementation for testing.
type MemoryIdempotencyStore struct {
	cache *MemoryCache
	ttl   time.Duration
}

// NewMemoryIdempotencyStore creates a new in-memory idempotency store.
func NewMemoryIdempotencyStore(ttl time.Duration) *MemoryIdempotencyStore {
	return &MemoryIdempotencyStore{cache: NewMemoryCache(), ttl: ttl}
}

// CheckAndSet attempts to set the key. Returns true if first time, false if duplicate.
func (s *MemoryIdempotencyStore) CheckAndSet(ctx context.Context, key string) (bool, error) {
	_, err := s.cache.Get(ctx, idempotencyKey(key))
	if err == nil {
		return false, nil // already exists
	}
	if !errors.Is(err, ErrCacheMiss) {
		return false, err
	}
	return true, s.cache.Set(ctx, idempotencyKey(key), []byte("1"), s.ttl)
}
