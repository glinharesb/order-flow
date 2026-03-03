package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Sentinel errors for the order domain.
var (
	ErrNotFound       = errors.New("not found")
	ErrInvalidInput   = errors.New("invalid input")
	ErrAlreadyExists  = errors.New("already exists")
	ErrInvalidStatus  = errors.New("invalid status transition")
	ErrDuplicateOrder = errors.New("duplicate order (idempotency key)")
)

// OrderStatus represents the lifecycle state of an order.
type OrderStatus string

const (
	StatusCreated        OrderStatus = "created"
	StatusPaymentPending OrderStatus = "payment_pending"
	StatusConfirmed      OrderStatus = "confirmed"
	StatusShipped        OrderStatus = "shipped"
	StatusDelivered      OrderStatus = "delivered"
	StatusCancelled      OrderStatus = "cancelled"
)

// ValidStatuses is the set of all valid order statuses.
var ValidStatuses = map[OrderStatus]bool{
	StatusCreated:        true,
	StatusPaymentPending: true,
	StatusConfirmed:      true,
	StatusShipped:        true,
	StatusDelivered:      true,
	StatusCancelled:      true,
}

// Order represents a customer order.
type Order struct {
	ID             uuid.UUID   `json:"id"`
	CustomerID     string      `json:"customer_id"`
	Status         OrderStatus `json:"status"`
	Items          []OrderItem `json:"items"`
	TotalAmount    int64       `json:"total_amount"`
	IdempotencyKey string      `json:"idempotency_key"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

// OrderItem represents a single line item within an order.
type OrderItem struct {
	ID        uuid.UUID `json:"id"`
	OrderID   uuid.UUID `json:"order_id"`
	ProductID string    `json:"product_id"`
	Quantity  int32     `json:"quantity"`
	UnitPrice int64     `json:"unit_price"`
}

// CreateOrderRequest holds the data needed to create a new order.
type CreateOrderRequest struct {
	CustomerID     string
	Items          []CreateOrderItemRequest
	IdempotencyKey string
}

// CreateOrderItemRequest holds line item data for order creation.
type CreateOrderItemRequest struct {
	ProductID string
	Quantity  int32
	UnitPrice int64
}

// Validate checks that the create request has all required fields.
func (r CreateOrderRequest) Validate() error {
	if r.CustomerID == "" {
		return fmt.Errorf("%w: customer_id is required", ErrInvalidInput)
	}
	if r.IdempotencyKey == "" {
		return fmt.Errorf("%w: idempotency_key is required", ErrInvalidInput)
	}
	if len(r.Items) == 0 {
		return fmt.Errorf("%w: at least one item is required", ErrInvalidInput)
	}
	for i, item := range r.Items {
		if item.ProductID == "" {
			return fmt.Errorf("%w: item[%d].product_id is required", ErrInvalidInput, i)
		}
		if item.Quantity <= 0 {
			return fmt.Errorf("%w: item[%d].quantity must be positive", ErrInvalidInput, i)
		}
		if item.UnitPrice <= 0 {
			return fmt.Errorf("%w: item[%d].unit_price must be positive", ErrInvalidInput, i)
		}
	}
	return nil
}

// CalculateTotal computes the total amount from line items.
func CalculateTotal(items []CreateOrderItemRequest) int64 {
	var total int64
	for _, item := range items {
		total += int64(item.Quantity) * item.UnitPrice
	}
	return total
}

// ListOrdersFilter holds optional filters for listing orders.
type ListOrdersFilter struct {
	CustomerID string
	Status     OrderStatus
	PageSize   int
	PageToken  string // UUID of last item for cursor pagination
}

// Normalize ensures sane defaults for pagination.
func (f *ListOrdersFilter) Normalize() {
	if f.PageSize <= 0 || f.PageSize > 100 {
		f.PageSize = 20
	}
}
