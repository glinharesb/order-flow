package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/glinharesb/order-flow/internal/domain"
)

// OrderRepository defines the persistence interface for orders.
type OrderRepository interface {
	// CreateOrderTx inserts an order with its items and an outbox event in a single transaction.
	CreateOrderTx(ctx context.Context, order domain.Order, event domain.OrderEvent) error
	// Get retrieves a single order by ID including its items.
	Get(ctx context.Context, id uuid.UUID) (domain.Order, error)
	// List returns orders matching the given filter.
	List(ctx context.Context, filter domain.ListOrdersFilter) ([]domain.Order, int, error)
	// UpdateStatus transitions an order to a new status.
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.OrderStatus) error
	// UpdateStatusTx transitions an order to a new status and writes an outbox event in a single transaction.
	UpdateStatusTx(ctx context.Context, id uuid.UUID, status domain.OrderStatus, event domain.OrderEvent) error
}

// OutboxRepository defines the persistence interface for the transactional outbox.
type OutboxRepository interface {
	// FetchUnpublished returns up to limit outbox entries that have not been published.
	FetchUnpublished(ctx context.Context, limit int) ([]OutboxEntry, error)
	// MarkPublished marks an outbox entry as published.
	MarkPublished(ctx context.Context, id uuid.UUID) error
}

// OutboxEntry is the raw outbox row.
type OutboxEntry struct {
	ID        uuid.UUID
	EventType string
	Payload   []byte
}
