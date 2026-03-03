package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/glinharesb/order-flow/internal/domain"
)

func TestPostgresOutbox_FetchAndMark(t *testing.T) {
	db := testDB(t)
	outboxRepo := NewPostgresOutbox(db)
	orderRepo := NewPostgresOrder(db)
	ctx := context.Background()

	// Create an order so there's an outbox entry.
	orderID := uuid.New()
	now := time.Now().UTC().Truncate(time.Microsecond)
	order := domain.Order{
		ID: orderID, CustomerID: "cust-outbox", Status: domain.StatusCreated,
		TotalAmount: 1000, IdempotencyKey: "idem-outbox-" + uuid.NewString(),
		Items:     []domain.OrderItem{{ID: uuid.New(), ProductID: "p1", Quantity: 1, UnitPrice: 1000}},
		CreatedAt: now, UpdatedAt: now,
	}
	event := domain.NewOrderEvent(domain.EventOrderCreated, orderID, domain.StatusCreated)
	if err := orderRepo.CreateOrderTx(ctx, order, event); err != nil {
		t.Fatalf("CreateOrderTx: %v", err)
	}

	// Fetch unpublished.
	entries, err := outboxRepo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("FetchUnpublished: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one outbox entry")
	}

	found := false
	for _, e := range entries {
		var ev domain.OrderEvent
		if err := json.Unmarshal(e.Payload, &ev); err != nil {
			continue
		}
		if ev.OrderID == orderID {
			found = true
			// Mark as published.
			if err := outboxRepo.MarkPublished(ctx, e.ID); err != nil {
				t.Fatalf("MarkPublished: %v", err)
			}
			break
		}
	}
	if !found {
		t.Error("did not find outbox entry for the created order")
	}

	// After marking, it should no longer appear.
	entries2, err := outboxRepo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("FetchUnpublished (after mark): %v", err)
	}
	for _, e := range entries2 {
		var ev domain.OrderEvent
		if err := json.Unmarshal(e.Payload, &ev); err != nil {
			continue
		}
		if ev.OrderID == orderID {
			t.Error("expected marked entry to not appear in unpublished")
		}
	}
}
