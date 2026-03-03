package repository

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/glinharesb/order-flow/internal/domain"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate runs all SQL migration files in order.
func Migrate(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for i, entry := range entries {
		version := i + 1

		var applied int
		err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE version = $1", version).Scan(&applied)
		if err != nil {
			return fmt.Errorf("check migration %d: %w", version, err)
		}
		if applied > 0 {
			continue
		}

		content, err := migrationsFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin tx for migration %d: %w", version, err)
		}

		if _, err := tx.ExecContext(ctx, string(content)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("exec migration %s: %w", entry.Name(), err)
		}

		if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %d: %w", version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", version, err)
		}

		slog.Info("applied migration", "file", entry.Name(), "version", version)
	}

	return nil
}

// PostgresOrder implements OrderRepository using PostgreSQL.
type PostgresOrder struct {
	db *sql.DB
}

// NewPostgresOrder creates a new PostgreSQL-backed order repository.
func NewPostgresOrder(db *sql.DB) *PostgresOrder {
	return &PostgresOrder{db: db}
}

// CreateOrderTx inserts an order, its items, and an outbox event in a single transaction.
func (r *PostgresOrder) CreateOrderTx(ctx context.Context, order domain.Order, event domain.OrderEvent) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO orders (id, customer_id, status, total_amount, idempotency_key, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, order.ID, order.CustomerID, order.Status, order.TotalAmount, order.IdempotencyKey, order.CreatedAt, order.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return domain.ErrDuplicateOrder
		}
		return fmt.Errorf("insert order: %w", err)
	}

	for _, item := range order.Items {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO order_items (id, order_id, product_id, quantity, unit_price)
			VALUES ($1, $2, $3, $4, $5)
		`, item.ID, order.ID, item.ProductID, item.Quantity, item.UnitPrice)
		if err != nil {
			return fmt.Errorf("insert order_item: %w", err)
		}
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO outbox (id, event_type, payload, created_at)
		VALUES ($1, $2, $3, $4)
	`, event.ID, event.Type, payload, event.Timestamp)
	if err != nil {
		return fmt.Errorf("insert outbox: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}

// Get retrieves a single order by ID including its items.
func (r *PostgresOrder) Get(ctx context.Context, id uuid.UUID) (domain.Order, error) {
	var o domain.Order
	err := r.db.QueryRowContext(ctx, `
		SELECT id, customer_id, status, total_amount, idempotency_key, created_at, updated_at
		FROM orders WHERE id = $1
	`, id).Scan(&o.ID, &o.CustomerID, &o.Status, &o.TotalAmount, &o.IdempotencyKey, &o.CreatedAt, &o.UpdatedAt)
	if err == sql.ErrNoRows {
		return domain.Order{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Order{}, fmt.Errorf("query order: %w", err)
	}

	items, err := r.getItems(ctx, id)
	if err != nil {
		return domain.Order{}, err
	}
	o.Items = items

	return o, nil
}

func (r *PostgresOrder) getItems(ctx context.Context, orderID uuid.UUID) ([]domain.OrderItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, order_id, product_id, quantity, unit_price
		FROM order_items WHERE order_id = $1
	`, orderID)
	if err != nil {
		return nil, fmt.Errorf("query order_items: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var items []domain.OrderItem
	for rows.Next() {
		var item domain.OrderItem
		if err := rows.Scan(&item.ID, &item.OrderID, &item.ProductID, &item.Quantity, &item.UnitPrice); err != nil {
			return nil, fmt.Errorf("scan order_item: %w", err)
		}
		items = append(items, item)
	}
	if items == nil {
		items = []domain.OrderItem{}
	}
	return items, rows.Err()
}

// List returns orders matching the filter with cursor-based pagination.
func (r *PostgresOrder) List(ctx context.Context, filter domain.ListOrdersFilter) ([]domain.Order, int, error) {
	filter.Normalize()

	var conditions []string
	var args []any
	argIdx := 1

	if filter.CustomerID != "" {
		conditions = append(conditions, fmt.Sprintf("customer_id = $%d", argIdx))
		args = append(args, filter.CustomerID)
		argIdx++
	}
	if filter.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, filter.Status)
		argIdx++
	}
	if filter.PageToken != "" {
		conditions = append(conditions, fmt.Sprintf("id < $%d", argIdx))
		args = append(args, filter.PageToken)
		argIdx++
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total matching (excluding cursor condition).
	var total int
	countArgs := make([]any, 0, len(args))
	countConditions := conditions
	if filter.PageToken != "" {
		countConditions = countConditions[:len(countConditions)-1]
	}
	countWhere := ""
	if len(countConditions) > 0 {
		countWhere = "WHERE " + strings.Join(countConditions, " AND ")
	}
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM orders %s", countWhere)
	for i, a := range args {
		if filter.PageToken != "" && i == len(args)-1 {
			break
		}
		countArgs = append(countArgs, a)
	}
	if err := r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count orders: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT id, customer_id, status, total_amount, idempotency_key, created_at, updated_at
		FROM orders %s
		ORDER BY id DESC
		LIMIT $%d
	`, where, argIdx)
	args = append(args, filter.PageSize)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query orders: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var orders []domain.Order
	for rows.Next() {
		var o domain.Order
		if err := rows.Scan(&o.ID, &o.CustomerID, &o.Status, &o.TotalAmount, &o.IdempotencyKey, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan order: %w", err)
		}
		items, err := r.getItems(ctx, o.ID)
		if err != nil {
			return nil, 0, err
		}
		o.Items = items
		orders = append(orders, o)
	}
	if orders == nil {
		orders = []domain.Order{}
	}

	return orders, total, rows.Err()
}

// UpdateStatus sets the order status and updated_at timestamp.
func (r *PostgresOrder) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.OrderStatus) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE orders SET status = $1, updated_at = $2 WHERE id = $3
	`, status, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("update order status: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// UpdateStatusTx transitions an order's status and writes an outbox event atomically.
func (r *PostgresOrder) UpdateStatusTx(ctx context.Context, id uuid.UUID, status domain.OrderStatus, event domain.OrderEvent) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, `
		UPDATE orders SET status = $1, updated_at = $2 WHERE id = $3
	`, status, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("update order status: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return domain.ErrNotFound
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO outbox (id, event_type, payload, created_at)
		VALUES ($1, $2, $3, $4)
	`, event.ID, event.Type, payload, event.Timestamp)
	if err != nil {
		return fmt.Errorf("insert outbox: %w", err)
	}

	return tx.Commit()
}
