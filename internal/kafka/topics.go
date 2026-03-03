package kafka

// Topic constants for the order processing pipeline.
const (
	TopicOrderCreated     = "order.created"
	TopicOrderCancelled   = "order.cancelled"
	TopicPaymentProcessed = "payment.processed"
	TopicOrderDLQ         = "order.dlq"
)

// DLQTopic returns the dead-letter queue topic for a given source topic.
func DLQTopic(source string) string {
	return TopicOrderDLQ
}
