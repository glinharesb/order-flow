package kafka

import (
	"context"
	"fmt"

	"github.com/segmentio/kafka-go"
)

// EventPublisher defines the interface for publishing events to Kafka.
type EventPublisher interface {
	Publish(ctx context.Context, topic string, key []byte, value []byte, headers []kafka.Header) error
	Close() error
}

// KafkaPublisher publishes events using kafka-go Writer.
type KafkaPublisher struct {
	writer *kafka.Writer
}

// NewKafkaPublisher creates a new Kafka publisher.
func NewKafkaPublisher(brokers []string) *KafkaPublisher {
	w := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireAll,
		Async:        false,
	}
	return &KafkaPublisher{writer: w}
}

func (p *KafkaPublisher) Publish(ctx context.Context, topic string, key []byte, value []byte, headers []kafka.Header) error {
	msg := kafka.Message{
		Topic:   topic,
		Key:     key,
		Value:   value,
		Headers: headers,
	}
	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("kafka publish to %s: %w", topic, err)
	}
	return nil
}

func (p *KafkaPublisher) Close() error {
	return p.writer.Close()
}

// NoopPublisher is a no-op publisher for testing.
type NoopPublisher struct {
	Messages []PublishedMessage
}

// PublishedMessage records a message published by NoopPublisher.
type PublishedMessage struct {
	Topic   string
	Key     []byte
	Value   []byte
	Headers []kafka.Header
}

func (p *NoopPublisher) Publish(_ context.Context, topic string, key []byte, value []byte, headers []kafka.Header) error {
	p.Messages = append(p.Messages, PublishedMessage{Topic: topic, Key: key, Value: value, Headers: headers})
	return nil
}

func (p *NoopPublisher) Close() error { return nil }
