package cache

import (
	"context"
	"testing"
	"time"
)

func TestMemoryIdempotencyStore_CheckAndSet(t *testing.T) {
	store := NewMemoryIdempotencyStore(time.Minute)
	ctx := context.Background()

	// First call should succeed.
	ok, err := store.CheckAndSet(ctx, "key-1")
	if err != nil {
		t.Fatalf("CheckAndSet: %v", err)
	}
	if !ok {
		t.Error("expected true for first set")
	}

	// Second call with same key should return false (duplicate).
	ok, err = store.CheckAndSet(ctx, "key-1")
	if err != nil {
		t.Fatalf("CheckAndSet (dup): %v", err)
	}
	if ok {
		t.Error("expected false for duplicate set")
	}

	// Different key should succeed.
	ok, err = store.CheckAndSet(ctx, "key-2")
	if err != nil {
		t.Fatalf("CheckAndSet (new): %v", err)
	}
	if !ok {
		t.Error("expected true for new key")
	}
}

func TestMemoryIdempotencyStore_Expiry(t *testing.T) {
	store := NewMemoryIdempotencyStore(1 * time.Millisecond)
	ctx := context.Background()

	ok, err := store.CheckAndSet(ctx, "expire-key")
	if err != nil {
		t.Fatalf("CheckAndSet: %v", err)
	}
	if !ok {
		t.Fatal("expected true")
	}

	time.Sleep(5 * time.Millisecond)

	// After TTL, should be able to set again.
	ok, err = store.CheckAndSet(ctx, "expire-key")
	if err != nil {
		t.Fatalf("CheckAndSet (expired): %v", err)
	}
	if !ok {
		t.Error("expected true after expiry")
	}
}
