package kafka

import (
	"context"
	"log/slog"
	"time"

	"github.com/glinharesb/order-flow/internal/repository"
)

// OutboxRelay polls the outbox table and publishes events to Kafka.
type OutboxRelay struct {
	outbox    repository.OutboxRepository
	publisher EventPublisher
	pollFreq  time.Duration
	batchSize int
	logger    *slog.Logger
}

// NewOutboxRelay creates a new outbox relay.
func NewOutboxRelay(outbox repository.OutboxRepository, publisher EventPublisher, pollFreq time.Duration, logger *slog.Logger) *OutboxRelay {
	return &OutboxRelay{
		outbox:    outbox,
		publisher: publisher,
		pollFreq:  pollFreq,
		batchSize: 50,
		logger:    logger,
	}
}

// Run starts the relay loop until the context is cancelled.
func (r *OutboxRelay) Run(ctx context.Context) error {
	ticker := time.NewTicker(r.pollFreq)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := r.pollAndPublish(ctx); err != nil {
				r.logger.Error("outbox relay poll", "error", err)
			}
		}
	}
}

func (r *OutboxRelay) pollAndPublish(ctx context.Context) error {
	entries, err := r.outbox.FetchUnpublished(ctx, r.batchSize)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if err := r.publisher.Publish(ctx, entry.EventType, entry.ID[:], entry.Payload, nil); err != nil {
			r.logger.Error("publish outbox entry", "id", entry.ID, "error", err)
			continue
		}

		if err := r.outbox.MarkPublished(ctx, entry.ID); err != nil {
			r.logger.Error("mark outbox published", "id", entry.ID, "error", err)
			continue
		}

		r.logger.Debug("published outbox entry", "id", entry.ID, "type", entry.EventType)
	}

	return nil
}
