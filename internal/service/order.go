package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/glinharesb/order-flow/internal/cache"
	"github.com/glinharesb/order-flow/internal/domain"
	"github.com/glinharesb/order-flow/internal/repository"
)

// IdempotencyChecker checks and sets idempotency keys.
type IdempotencyChecker interface {
	CheckAndSet(ctx context.Context, key string) (bool, error)
}

// OrderService implements the core business logic for order management.
type OrderService struct {
	repo        repository.OrderRepository
	cache       cache.Cache
	idempotency IdempotencyChecker
	cacheTTL    time.Duration
	logger      *slog.Logger
}

// NewOrderService creates a new order service.
func NewOrderService(
	repo repository.OrderRepository,
	c cache.Cache,
	idempotency IdempotencyChecker,
	cacheTTL time.Duration,
	logger *slog.Logger,
) *OrderService {
	return &OrderService{
		repo:        repo,
		cache:       c,
		idempotency: idempotency,
		cacheTTL:    cacheTTL,
		logger:      logger,
	}
}

// CreateOrder validates, checks idempotency, and persists a new order with its items and outbox event.
func (s *OrderService) CreateOrder(ctx context.Context, req domain.CreateOrderRequest) (domain.Order, error) {
	if err := req.Validate(); err != nil {
		return domain.Order{}, err
	}

	// Idempotency check.
	isNew, err := s.idempotency.CheckAndSet(ctx, req.IdempotencyKey)
	if err != nil {
		return domain.Order{}, fmt.Errorf("idempotency check: %w", err)
	}
	if !isNew {
		return domain.Order{}, domain.ErrDuplicateOrder
	}

	orderID := uuid.New()
	now := time.Now().UTC()
	totalAmount := domain.CalculateTotal(req.Items)

	items := make([]domain.OrderItem, len(req.Items))
	for i, ri := range req.Items {
		items[i] = domain.OrderItem{
			ID:        uuid.New(),
			OrderID:   orderID,
			ProductID: ri.ProductID,
			Quantity:  ri.Quantity,
			UnitPrice: ri.UnitPrice,
		}
	}

	order := domain.Order{
		ID:             orderID,
		CustomerID:     req.CustomerID,
		Status:         domain.StatusCreated,
		Items:          items,
		TotalAmount:    totalAmount,
		IdempotencyKey: req.IdempotencyKey,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	event := domain.NewOrderEvent(domain.EventOrderCreated, orderID, domain.StatusCreated)

	if err := s.repo.CreateOrderTx(ctx, order, event); err != nil {
		if errors.Is(err, domain.ErrDuplicateOrder) {
			return domain.Order{}, err
		}
		return domain.Order{}, fmt.Errorf("create order: %w", err)
	}

	s.logger.Info("order created", "order_id", orderID, "customer_id", req.CustomerID, "total", totalAmount)
	return order, nil
}

// GetOrder retrieves an order by ID using cache-aside pattern.
func (s *OrderService) GetOrder(ctx context.Context, id uuid.UUID) (domain.Order, error) {
	// Check cache first.
	key := cache.OrderKey(id.String())
	if data, err := s.cache.Get(ctx, key); err == nil {
		var order domain.Order
		if err := json.Unmarshal(data, &order); err == nil {
			return order, nil
		}
	}

	// Cache miss — query repository.
	order, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.Order{}, err
	}

	// Populate cache.
	if data, err := json.Marshal(order); err == nil {
		_ = s.cache.Set(ctx, key, data, s.cacheTTL)
	}

	return order, nil
}

// ListOrders returns orders matching the filter.
func (s *OrderService) ListOrders(ctx context.Context, filter domain.ListOrdersFilter) ([]domain.Order, int, error) {
	return s.repo.List(ctx, filter)
}

// CancelOrder transitions an order to cancelled status.
func (s *OrderService) CancelOrder(ctx context.Context, id uuid.UUID) (domain.Order, error) {
	order, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.Order{}, err
	}

	if err := domain.Transition(order.Status, domain.StatusCancelled); err != nil {
		return domain.Order{}, err
	}

	event := domain.NewOrderEvent(domain.EventOrderCancelled, id, domain.StatusCancelled)

	if err := s.repo.UpdateStatusTx(ctx, id, domain.StatusCancelled, event); err != nil {
		return domain.Order{}, fmt.Errorf("cancel order: %w", err)
	}

	// Invalidate cache.
	_ = s.cache.Delete(ctx, cache.OrderKey(id.String()))

	order.Status = domain.StatusCancelled
	s.logger.Info("order cancelled", "order_id", id)
	return order, nil
}
