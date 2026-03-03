package kafka

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/segmentio/kafka-go"
)

const maxRetries = 3

// MessageHandler processes a Kafka message. Return an error to trigger retry/DLQ.
type MessageHandler func(ctx context.Context, msg kafka.Message) error

// EventConsumer reads messages from Kafka topics and dispatches to handlers.
type EventConsumer struct {
	reader    *kafka.Reader
	publisher EventPublisher
	handlers  map[string]MessageHandler
	logger    *slog.Logger
}

// NewEventConsumer creates a consumer for the given topics.
func NewEventConsumer(brokers []string, groupID string, topics []string, publisher EventPublisher, logger *slog.Logger) *EventConsumer {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		GroupID:  groupID,
		Topic:    topics[0],
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	return &EventConsumer{
		reader:    r,
		publisher: publisher,
		handlers:  make(map[string]MessageHandler),
		logger:    logger,
	}
}

// RegisterHandler maps a topic to a handler function.
func (c *EventConsumer) RegisterHandler(topic string, handler MessageHandler) {
	c.handlers[topic] = handler
}

// Run starts consuming messages until the context is cancelled.
func (c *EventConsumer) Run(ctx context.Context) error {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("fetch message: %w", err)
		}

		handler, ok := c.handlers[msg.Topic]
		if !ok {
			c.logger.Warn("no handler for topic", "topic", msg.Topic)
			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				c.logger.Error("commit message", "error", err)
			}
			continue
		}

		if err := c.processWithRetry(ctx, msg, handler); err != nil {
			c.sendToDLQ(ctx, msg, err)
		}

		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			c.logger.Error("commit message", "error", err)
		}
	}
}

func (c *EventConsumer) processWithRetry(ctx context.Context, msg kafka.Message, handler MessageHandler) error {
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err := handler(ctx, msg); err != nil {
			lastErr = err
			c.logger.Warn("handler failed", "topic", msg.Topic, "attempt", attempt, "error", err)
			continue
		}
		return nil
	}
	return lastErr
}

func (c *EventConsumer) sendToDLQ(ctx context.Context, msg kafka.Message, originalErr error) {
	headers := append(msg.Headers,
		kafka.Header{Key: "x-original-topic", Value: []byte(msg.Topic)},
		kafka.Header{Key: "x-error", Value: []byte(originalErr.Error())},
	)

	if err := c.publisher.Publish(ctx, DLQTopic(msg.Topic), msg.Key, msg.Value, headers); err != nil {
		c.logger.Error("send to DLQ failed", "topic", msg.Topic, "error", err)
	} else {
		c.logger.Info("sent to DLQ", "topic", msg.Topic, "dlq", DLQTopic(msg.Topic))
	}
}

// Close stops the consumer.
func (c *EventConsumer) Close() error {
	return c.reader.Close()
}
