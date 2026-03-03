package domain

import (
	"time"

	"github.com/google/uuid"
)

// EventType classifies order lifecycle events.
type EventType string

const (
	EventOrderCreated      EventType = "order.created"
	EventOrderCancelled    EventType = "order.cancelled"
	EventPaymentProcessed  EventType = "payment.processed"
	EventOrderConfirmed    EventType = "order.confirmed"
	EventOrderShipped      EventType = "order.shipped"
	EventOrderDelivered    EventType = "order.delivered"
)

// OrderEvent is the envelope for all order-related events published to Kafka.
type OrderEvent struct {
	ID        uuid.UUID   `json:"id"`
	Type      EventType   `json:"type"`
	OrderID   uuid.UUID   `json:"order_id"`
	Status    OrderStatus `json:"status"`
	Payload   any         `json:"payload,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// NewOrderEvent creates a new event envelope.
func NewOrderEvent(eventType EventType, orderID uuid.UUID, status OrderStatus) OrderEvent {
	return OrderEvent{
		ID:        uuid.New(),
		Type:      eventType,
		OrderID:   orderID,
		Status:    status,
		Timestamp: time.Now().UTC(),
	}
}
