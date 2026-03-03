package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"

	"github.com/glinharesb/order-flow/internal/cache"
	"github.com/glinharesb/order-flow/internal/domain"
	"log/slog"
)

func TestEventProcessor_HandleOrderCreated(t *testing.T) {
	orderID := uuid.New()
	order := domain.Order{ID: orderID, Status: domain.StatusCreated}

	var updatedStatus domain.OrderStatus
	repo := &mockOrderRepo{
		getFn: func(_ context.Context, _ uuid.UUID) (domain.Order, error) {
			return order, nil
		},
		updateStatusFn: func(_ context.Context, _ uuid.UUID, status domain.OrderStatus) error {
			updatedStatus = status
			return nil
		},
	}

	proc := NewEventProcessor(repo, cache.NewMemoryCache(), slog.Default())

	event := domain.NewOrderEvent(domain.EventOrderCreated, orderID, domain.StatusCreated)
	payload, _ := json.Marshal(event)

	err := proc.HandleOrderCreated(context.Background(), kafka.Message{Value: payload})
	if err != nil {
		t.Fatalf("HandleOrderCreated: %v", err)
	}
	if updatedStatus != domain.StatusPaymentPending {
		t.Errorf("status = %q, want %q", updatedStatus, domain.StatusPaymentPending)
	}
}

func TestEventProcessor_HandlePaymentProcessed(t *testing.T) {
	orderID := uuid.New()
	order := domain.Order{ID: orderID, Status: domain.StatusPaymentPending}

	var updatedStatus domain.OrderStatus
	repo := &mockOrderRepo{
		getFn: func(_ context.Context, _ uuid.UUID) (domain.Order, error) {
			return order, nil
		},
		updateStatusFn: func(_ context.Context, _ uuid.UUID, status domain.OrderStatus) error {
			updatedStatus = status
			return nil
		},
	}

	proc := NewEventProcessor(repo, cache.NewMemoryCache(), slog.Default())

	event := domain.NewOrderEvent(domain.EventPaymentProcessed, orderID, domain.StatusPaymentPending)
	payload, _ := json.Marshal(event)

	err := proc.HandlePaymentProcessed(context.Background(), kafka.Message{Value: payload})
	if err != nil {
		t.Fatalf("HandlePaymentProcessed: %v", err)
	}
	if updatedStatus != domain.StatusConfirmed {
		t.Errorf("status = %q, want %q", updatedStatus, domain.StatusConfirmed)
	}
}

func TestEventProcessor_HandleOrderCreated_AlreadyTransitioned(t *testing.T) {
	orderID := uuid.New()
	order := domain.Order{ID: orderID, Status: domain.StatusPaymentPending}

	repo := &mockOrderRepo{
		getFn: func(_ context.Context, _ uuid.UUID) (domain.Order, error) {
			return order, nil
		},
	}

	proc := NewEventProcessor(repo, cache.NewMemoryCache(), slog.Default())

	event := domain.NewOrderEvent(domain.EventOrderCreated, orderID, domain.StatusCreated)
	payload, _ := json.Marshal(event)

	// Should not error — just skip.
	err := proc.HandleOrderCreated(context.Background(), kafka.Message{Value: payload})
	if err != nil {
		t.Fatalf("expected no error for already-transitioned order, got %v", err)
	}
}
