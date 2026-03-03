package repository

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/glinharesb/order-flow/internal/domain"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Clean up tables for test isolation.
	for _, table := range []string{"outbox", "order_items", "orders"} {
		if _, err := db.ExecContext(ctx, "DELETE FROM "+table); err != nil {
			t.Fatalf("clean table %s: %v", table, err)
		}
	}

	return db
}

func TestPostgresOrder_CreateAndGet(t *testing.T) {
	db := testDB(t)
	repo := NewPostgresOrder(db)
	ctx := context.Background()

	orderID := uuid.New()
	now := time.Now().UTC().Truncate(time.Microsecond)

	order := domain.Order{
		ID:             orderID,
		CustomerID:     "cust-1",
		Status:         domain.StatusCreated,
		TotalAmount:    3000,
		IdempotencyKey: "idem-" + uuid.NewString(),
		Items: []domain.OrderItem{
			{ID: uuid.New(), OrderID: orderID, ProductID: "prod-1", Quantity: 2, UnitPrice: 1000},
			{ID: uuid.New(), OrderID: orderID, ProductID: "prod-2", Quantity: 1, UnitPrice: 1000},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	event := domain.NewOrderEvent(domain.EventOrderCreated, orderID, domain.StatusCreated)

	if err := repo.CreateOrderTx(ctx, order, event); err != nil {
		t.Fatalf("CreateOrderTx: %v", err)
	}

	got, err := repo.Get(ctx, orderID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.CustomerID != "cust-1" {
		t.Errorf("CustomerID = %q, want %q", got.CustomerID, "cust-1")
	}
	if got.Status != domain.StatusCreated {
		t.Errorf("Status = %q, want %q", got.Status, domain.StatusCreated)
	}
	if len(got.Items) != 2 {
		t.Errorf("Items count = %d, want 2", len(got.Items))
	}
}

func TestPostgresOrder_DuplicateIdempotencyKey(t *testing.T) {
	db := testDB(t)
	repo := NewPostgresOrder(db)
	ctx := context.Background()

	idemKey := "idem-dup-" + uuid.NewString()
	now := time.Now().UTC().Truncate(time.Microsecond)

	order := domain.Order{
		ID: uuid.New(), CustomerID: "cust-1", Status: domain.StatusCreated,
		TotalAmount: 1000, IdempotencyKey: idemKey,
		Items:     []domain.OrderItem{{ID: uuid.New(), ProductID: "p1", Quantity: 1, UnitPrice: 1000}},
		CreatedAt: now, UpdatedAt: now,
	}
	event := domain.NewOrderEvent(domain.EventOrderCreated, order.ID, domain.StatusCreated)

	if err := repo.CreateOrderTx(ctx, order, event); err != nil {
		t.Fatalf("first create: %v", err)
	}

	order2 := order
	order2.ID = uuid.New()
	event2 := domain.NewOrderEvent(domain.EventOrderCreated, order2.ID, domain.StatusCreated)

	err := repo.CreateOrderTx(ctx, order2, event2)
	if err != domain.ErrDuplicateOrder {
		t.Errorf("expected ErrDuplicateOrder, got %v", err)
	}
}

func TestPostgresOrder_UpdateStatus(t *testing.T) {
	db := testDB(t)
	repo := NewPostgresOrder(db)
	ctx := context.Background()

	orderID := uuid.New()
	now := time.Now().UTC().Truncate(time.Microsecond)
	order := domain.Order{
		ID: orderID, CustomerID: "cust-1", Status: domain.StatusCreated,
		TotalAmount: 500, IdempotencyKey: "idem-upd-" + uuid.NewString(),
		Items:     []domain.OrderItem{{ID: uuid.New(), ProductID: "p1", Quantity: 1, UnitPrice: 500}},
		CreatedAt: now, UpdatedAt: now,
	}
	event := domain.NewOrderEvent(domain.EventOrderCreated, orderID, domain.StatusCreated)

	if err := repo.CreateOrderTx(ctx, order, event); err != nil {
		t.Fatalf("CreateOrderTx: %v", err)
	}

	if err := repo.UpdateStatus(ctx, orderID, domain.StatusPaymentPending); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	got, err := repo.Get(ctx, orderID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != domain.StatusPaymentPending {
		t.Errorf("Status = %q, want %q", got.Status, domain.StatusPaymentPending)
	}
}

func TestPostgresOrder_List(t *testing.T) {
	db := testDB(t)
	repo := NewPostgresOrder(db)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Microsecond)
	for i := range 3 {
		id := uuid.New()
		order := domain.Order{
			ID: id, CustomerID: "cust-list", Status: domain.StatusCreated,
			TotalAmount: int64((i + 1) * 100), IdempotencyKey: "idem-list-" + uuid.NewString(),
			Items:     []domain.OrderItem{{ID: uuid.New(), ProductID: "p1", Quantity: 1, UnitPrice: int64((i + 1) * 100)}},
			CreatedAt: now, UpdatedAt: now,
		}
		event := domain.NewOrderEvent(domain.EventOrderCreated, id, domain.StatusCreated)
		if err := repo.CreateOrderTx(ctx, order, event); err != nil {
			t.Fatalf("create order %d: %v", i, err)
		}
	}

	orders, total, err := repo.List(ctx, domain.ListOrdersFilter{CustomerID: "cust-list", PageSize: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if len(orders) != 3 {
		t.Errorf("orders count = %d, want 3", len(orders))
	}
}
