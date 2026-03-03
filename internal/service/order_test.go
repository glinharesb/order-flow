package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/glinharesb/order-flow/internal/cache"
	"github.com/glinharesb/order-flow/internal/domain"
	"log/slog"
)

// mockOrderRepo implements repository.OrderRepository for testing.
type mockOrderRepo struct {
	createFn       func(ctx context.Context, order domain.Order, event domain.OrderEvent) error
	getFn          func(ctx context.Context, id uuid.UUID) (domain.Order, error)
	listFn         func(ctx context.Context, filter domain.ListOrdersFilter) ([]domain.Order, int, error)
	updateStatusFn func(ctx context.Context, id uuid.UUID, status domain.OrderStatus) error
	updateTxFn     func(ctx context.Context, id uuid.UUID, status domain.OrderStatus, event domain.OrderEvent) error
}

func (m *mockOrderRepo) CreateOrderTx(ctx context.Context, order domain.Order, event domain.OrderEvent) error {
	return m.createFn(ctx, order, event)
}

func (m *mockOrderRepo) Get(ctx context.Context, id uuid.UUID) (domain.Order, error) {
	return m.getFn(ctx, id)
}

func (m *mockOrderRepo) List(ctx context.Context, filter domain.ListOrdersFilter) ([]domain.Order, int, error) {
	return m.listFn(ctx, filter)
}

func (m *mockOrderRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.OrderStatus) error {
	return m.updateStatusFn(ctx, id, status)
}

func (m *mockOrderRepo) UpdateStatusTx(ctx context.Context, id uuid.UUID, status domain.OrderStatus, event domain.OrderEvent) error {
	return m.updateTxFn(ctx, id, status, event)
}

func TestOrderService_CreateOrder(t *testing.T) {
	repo := &mockOrderRepo{
		createFn: func(_ context.Context, _ domain.Order, _ domain.OrderEvent) error {
			return nil
		},
	}
	c := cache.NewMemoryCache()
	idem := cache.NewMemoryIdempotencyStore(time.Minute)
	svc := NewOrderService(repo, c, idem, time.Minute, slog.Default())

	req := domain.CreateOrderRequest{
		CustomerID:     "cust-1",
		IdempotencyKey: "idem-create-1",
		Items: []domain.CreateOrderItemRequest{
			{ProductID: "prod-1", Quantity: 2, UnitPrice: 1500},
		},
	}

	order, err := svc.CreateOrder(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}
	if order.CustomerID != "cust-1" {
		t.Errorf("CustomerID = %q, want %q", order.CustomerID, "cust-1")
	}
	if order.TotalAmount != 3000 {
		t.Errorf("TotalAmount = %d, want 3000", order.TotalAmount)
	}
	if order.Status != domain.StatusCreated {
		t.Errorf("Status = %q, want %q", order.Status, domain.StatusCreated)
	}
}

func TestOrderService_CreateOrder_DuplicateIdempotency(t *testing.T) {
	repo := &mockOrderRepo{
		createFn: func(_ context.Context, _ domain.Order, _ domain.OrderEvent) error {
			return nil
		},
	}
	c := cache.NewMemoryCache()
	idem := cache.NewMemoryIdempotencyStore(time.Minute)
	svc := NewOrderService(repo, c, idem, time.Minute, slog.Default())

	req := domain.CreateOrderRequest{
		CustomerID:     "cust-1",
		IdempotencyKey: "idem-dup-1",
		Items:          []domain.CreateOrderItemRequest{{ProductID: "p1", Quantity: 1, UnitPrice: 100}},
	}

	_, err := svc.CreateOrder(context.Background(), req)
	if err != nil {
		t.Fatalf("first create: %v", err)
	}

	_, err = svc.CreateOrder(context.Background(), req)
	if !errors.Is(err, domain.ErrDuplicateOrder) {
		t.Errorf("expected ErrDuplicateOrder, got %v", err)
	}
}

func TestOrderService_CreateOrder_ValidationError(t *testing.T) {
	svc := NewOrderService(nil, cache.NewMemoryCache(), cache.NewMemoryIdempotencyStore(time.Minute), time.Minute, slog.Default())

	_, err := svc.CreateOrder(context.Background(), domain.CreateOrderRequest{})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}

func TestOrderService_GetOrder_CacheHit(t *testing.T) {
	id := uuid.New()
	order := domain.Order{ID: id, CustomerID: "cust-1", Status: domain.StatusCreated, TotalAmount: 1000}

	callCount := 0
	repo := &mockOrderRepo{
		getFn: func(_ context.Context, _ uuid.UUID) (domain.Order, error) {
			callCount++
			return order, nil
		},
	}
	c := cache.NewMemoryCache()
	svc := NewOrderService(repo, c, cache.NewMemoryIdempotencyStore(time.Minute), time.Minute, slog.Default())

	ctx := context.Background()

	// First call — cache miss.
	got1, err := svc.GetOrder(ctx, id)
	if err != nil {
		t.Fatalf("GetOrder (miss): %v", err)
	}
	if got1.CustomerID != "cust-1" {
		t.Errorf("CustomerID = %q", got1.CustomerID)
	}

	// Second call — cache hit.
	got2, err := svc.GetOrder(ctx, id)
	if err != nil {
		t.Fatalf("GetOrder (hit): %v", err)
	}
	if got2.CustomerID != "cust-1" {
		t.Errorf("CustomerID = %q", got2.CustomerID)
	}

	if callCount != 1 {
		t.Errorf("repo.Get called %d times, want 1", callCount)
	}
}

func TestOrderService_CancelOrder(t *testing.T) {
	id := uuid.New()
	order := domain.Order{ID: id, CustomerID: "cust-1", Status: domain.StatusCreated, TotalAmount: 1000}

	repo := &mockOrderRepo{
		getFn: func(_ context.Context, _ uuid.UUID) (domain.Order, error) {
			return order, nil
		},
		updateTxFn: func(_ context.Context, _ uuid.UUID, _ domain.OrderStatus, _ domain.OrderEvent) error {
			return nil
		},
	}
	c := cache.NewMemoryCache()
	svc := NewOrderService(repo, c, cache.NewMemoryIdempotencyStore(time.Minute), time.Minute, slog.Default())

	got, err := svc.CancelOrder(context.Background(), id)
	if err != nil {
		t.Fatalf("CancelOrder: %v", err)
	}
	if got.Status != domain.StatusCancelled {
		t.Errorf("Status = %q, want %q", got.Status, domain.StatusCancelled)
	}
}

func TestOrderService_CancelOrder_InvalidTransition(t *testing.T) {
	id := uuid.New()
	order := domain.Order{ID: id, Status: domain.StatusDelivered}

	repo := &mockOrderRepo{
		getFn: func(_ context.Context, _ uuid.UUID) (domain.Order, error) {
			return order, nil
		},
	}
	svc := NewOrderService(repo, cache.NewMemoryCache(), cache.NewMemoryIdempotencyStore(time.Minute), time.Minute, slog.Default())

	_, err := svc.CancelOrder(context.Background(), id)
	if !errors.Is(err, domain.ErrInvalidStatus) {
		t.Errorf("expected ErrInvalidStatus, got %v", err)
	}
}
