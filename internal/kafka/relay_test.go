package kafka

import (
	"context"
	"sync"
	"testing"
	"time"

	"log/slog"

	"github.com/google/uuid"

	"github.com/glinharesb/order-flow/internal/repository"
)

// mockOutbox implements repository.OutboxRepository for testing.
type mockOutbox struct {
	mu        sync.Mutex
	entries   []repository.OutboxEntry
	published map[uuid.UUID]bool
}

func newMockOutbox(entries []repository.OutboxEntry) *mockOutbox {
	return &mockOutbox{entries: entries, published: make(map[uuid.UUID]bool)}
}

func (m *mockOutbox) FetchUnpublished(_ context.Context, limit int) ([]repository.OutboxEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var result []repository.OutboxEntry
	for _, e := range m.entries {
		if !m.published[e.ID] {
			result = append(result, e)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (m *mockOutbox) MarkPublished(_ context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.published[id] = true
	return nil
}

func TestOutboxRelay_PollAndPublish(t *testing.T) {
	id1, id2 := uuid.New(), uuid.New()
	entries := []repository.OutboxEntry{
		{ID: id1, EventType: "order.created", Payload: []byte(`{"id":"1"}`)},
		{ID: id2, EventType: "order.cancelled", Payload: []byte(`{"id":"2"}`)},
	}

	outbox := newMockOutbox(entries)
	pub := &NoopPublisher{}
	logger := slog.Default()

	relay := NewOutboxRelay(outbox, pub, 10*time.Millisecond, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_ = relay.Run(ctx)

	if len(pub.Messages) < 2 {
		t.Errorf("published %d messages, want at least 2", len(pub.Messages))
	}

	outbox.mu.Lock()
	defer outbox.mu.Unlock()
	if !outbox.published[id1] {
		t.Error("entry 1 not marked as published")
	}
	if !outbox.published[id2] {
		t.Error("entry 2 not marked as published")
	}
}
