package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// PostgresOutbox implements OutboxRepository using PostgreSQL.
type PostgresOutbox struct {
	db *sql.DB
}

// NewPostgresOutbox creates a new PostgreSQL-backed outbox repository.
func NewPostgresOutbox(db *sql.DB) *PostgresOutbox {
	return &PostgresOutbox{db: db}
}

// FetchUnpublished returns up to limit outbox entries that have not been published yet.
func (r *PostgresOutbox) FetchUnpublished(ctx context.Context, limit int) ([]OutboxEntry, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, event_type, payload
		FROM outbox
		WHERE published_at IS NULL
		ORDER BY created_at ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("query outbox: %w", err)
	}
	defer rows.Close()

	var entries []OutboxEntry
	for rows.Next() {
		var e OutboxEntry
		if err := rows.Scan(&e.ID, &e.EventType, &e.Payload); err != nil {
			return nil, fmt.Errorf("scan outbox entry: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// MarkPublished sets the published_at timestamp on an outbox entry.
func (r *PostgresOutbox) MarkPublished(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE outbox SET published_at = $1 WHERE id = $2
	`, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("mark outbox published: %w", err)
	}
	return nil
}
