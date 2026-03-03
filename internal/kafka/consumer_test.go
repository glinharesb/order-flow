package kafka

import (
	"context"
	"testing"
)

func TestDLQTopic(t *testing.T) {
	tests := []struct {
		source string
		want   string
	}{
		{"order.created", TopicOrderDLQ},
		{"order.cancelled", TopicOrderDLQ},
		{"payment.processed", TopicOrderDLQ},
	}

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			got := DLQTopic(tt.source)
			if got != tt.want {
				t.Errorf("DLQTopic(%q) = %q, want %q", tt.source, got, tt.want)
			}
		})
	}
}

func TestNoopPublisher(t *testing.T) {
	pub := &NoopPublisher{}

	if err := pub.Publish(context.TODO(), "test-topic", []byte("key"), []byte("value"), nil); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	if len(pub.Messages) != 1 {
		t.Fatalf("Messages count = %d, want 1", len(pub.Messages))
	}
	if pub.Messages[0].Topic != "test-topic" {
		t.Errorf("Topic = %q, want %q", pub.Messages[0].Topic, "test-topic")
	}
	if string(pub.Messages[0].Key) != "key" {
		t.Errorf("Key = %q, want %q", pub.Messages[0].Key, "key")
	}
}
