package cache

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMemoryCache_SetGetDelete(t *testing.T) {
	c := NewMemoryCache()
	ctx := context.Background()

	// Miss on empty cache.
	_, err := c.Get(ctx, "key1")
	if !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("expected ErrCacheMiss, got %v", err)
	}

	// Set and get.
	if err := c.Set(ctx, "key1", []byte("value1"), time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := c.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "value1" {
		t.Errorf("got %q, want %q", got, "value1")
	}

	// Delete.
	if err := c.Delete(ctx, "key1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = c.Get(ctx, "key1")
	if !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("expected ErrCacheMiss after delete, got %v", err)
	}
}

func TestMemoryCache_Expiry(t *testing.T) {
	c := NewMemoryCache()
	ctx := context.Background()

	if err := c.Set(ctx, "exp", []byte("data"), 1*time.Millisecond); err != nil {
		t.Fatalf("Set: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	_, err := c.Get(ctx, "exp")
	if !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("expected ErrCacheMiss after expiry, got %v", err)
	}
}

func TestKeyBuilders(t *testing.T) {
	if got := OrderKey("abc"); got != "order:order:abc" {
		t.Errorf("OrderKey = %q", got)
	}
	h := HashFilter("test")
	if len(h) != 16 {
		t.Errorf("HashFilter length = %d, want 16", len(h))
	}
}
