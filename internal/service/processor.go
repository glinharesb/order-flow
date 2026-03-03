package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/segmentio/kafka-go"

	"github.com/glinharesb/order-flow/internal/cache"
	"github.com/glinharesb/order-flow/internal/domain"
	"github.com/glinharesb/order-flow/internal/repository"
)

// EventProcessor handles Kafka messages and transitions order state.
type EventProcessor struct {
	repo   repository.OrderRepository
	cache  cache.Cache
	logger *slog.Logger
}

// NewEventProcessor creates a new event processor.
func NewEventProcessor(repo repository.OrderRepository, c cache.Cache, logger *slog.Logger) *EventProcessor {
	return &EventProcessor{repo: repo, cache: c, logger: logger}
}

// HandleOrderCreated processes order.created events (transitions to payment_pending).
func (p *EventProcessor) HandleOrderCreated(ctx context.Context, msg kafka.Message) error {
	var event domain.OrderEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return fmt.Errorf("unmarshal order event: %w", err)
	}

	order, err := p.repo.Get(ctx, event.OrderID)
	if err != nil {
		return fmt.Errorf("get order %s: %w", event.OrderID, err)
	}

	if err := domain.Transition(order.Status, domain.StatusPaymentPending); err != nil {
		p.logger.Warn("skipping transition", "order_id", event.OrderID, "current", order.Status, "target", domain.StatusPaymentPending)
		return nil // not an error — already transitioned
	}

	if err := p.repo.UpdateStatus(ctx, event.OrderID, domain.StatusPaymentPending); err != nil {
		return fmt.Errorf("update order %s: %w", event.OrderID, err)
	}

	_ = p.cache.Delete(ctx, cache.OrderKey(event.OrderID.String()))
	p.logger.Info("order moved to payment_pending", "order_id", event.OrderID)
	return nil
}

// HandlePaymentProcessed processes payment.processed events (transitions to confirmed).
func (p *EventProcessor) HandlePaymentProcessed(ctx context.Context, msg kafka.Message) error {
	var event domain.OrderEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return fmt.Errorf("unmarshal payment event: %w", err)
	}

	order, err := p.repo.Get(ctx, event.OrderID)
	if err != nil {
		return fmt.Errorf("get order %s: %w", event.OrderID, err)
	}

	if err := domain.Transition(order.Status, domain.StatusConfirmed); err != nil {
		p.logger.Warn("skipping transition", "order_id", event.OrderID, "current", order.Status, "target", domain.StatusConfirmed)
		return nil
	}

	if err := p.repo.UpdateStatus(ctx, event.OrderID, domain.StatusConfirmed); err != nil {
		return fmt.Errorf("update order %s: %w", event.OrderID, err)
	}

	_ = p.cache.Delete(ctx, cache.OrderKey(event.OrderID.String()))
	p.logger.Info("order confirmed", "order_id", event.OrderID)
	return nil
}

// HandleOrderCancelled processes order.cancelled events (logs cancellation).
func (p *EventProcessor) HandleOrderCancelled(ctx context.Context, msg kafka.Message) error {
	var event domain.OrderEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return fmt.Errorf("unmarshal cancel event: %w", err)
	}

	_ = p.cache.Delete(ctx, cache.OrderKey(event.OrderID.String()))
	p.logger.Info("order cancellation event processed", "order_id", event.OrderID)
	return nil
}
