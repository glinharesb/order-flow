package domain

import (
	"errors"
	"testing"
)

func TestCreateOrderRequest_Validate(t *testing.T) {
	validItems := []CreateOrderItemRequest{
		{ProductID: "prod-1", Quantity: 2, UnitPrice: 1500},
	}

	tests := []struct {
		name    string
		req     CreateOrderRequest
		wantErr bool
		errIs   error
	}{
		{
			name: "valid request",
			req: CreateOrderRequest{
				CustomerID:     "cust-1",
				IdempotencyKey: "idem-1",
				Items:          validItems,
			},
			wantErr: false,
		},
		{
			name: "missing customer_id",
			req: CreateOrderRequest{
				IdempotencyKey: "idem-1",
				Items:          validItems,
			},
			wantErr: true,
			errIs:   ErrInvalidInput,
		},
		{
			name: "missing idempotency_key",
			req: CreateOrderRequest{
				CustomerID: "cust-1",
				Items:      validItems,
			},
			wantErr: true,
			errIs:   ErrInvalidInput,
		},
		{
			name: "empty items",
			req: CreateOrderRequest{
				CustomerID:     "cust-1",
				IdempotencyKey: "idem-1",
				Items:          nil,
			},
			wantErr: true,
			errIs:   ErrInvalidInput,
		},
		{
			name: "item missing product_id",
			req: CreateOrderRequest{
				CustomerID:     "cust-1",
				IdempotencyKey: "idem-1",
				Items:          []CreateOrderItemRequest{{Quantity: 1, UnitPrice: 100}},
			},
			wantErr: true,
			errIs:   ErrInvalidInput,
		},
		{
			name: "item zero quantity",
			req: CreateOrderRequest{
				CustomerID:     "cust-1",
				IdempotencyKey: "idem-1",
				Items:          []CreateOrderItemRequest{{ProductID: "p1", Quantity: 0, UnitPrice: 100}},
			},
			wantErr: true,
			errIs:   ErrInvalidInput,
		},
		{
			name: "item negative unit_price",
			req: CreateOrderRequest{
				CustomerID:     "cust-1",
				IdempotencyKey: "idem-1",
				Items:          []CreateOrderItemRequest{{ProductID: "p1", Quantity: 1, UnitPrice: -50}},
			},
			wantErr: true,
			errIs:   ErrInvalidInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.errIs != nil && !errors.Is(err, tt.errIs) {
				t.Errorf("expected error wrapping %v, got %v", tt.errIs, err)
			}
		})
	}
}

func TestCalculateTotal(t *testing.T) {
	tests := []struct {
		name  string
		items []CreateOrderItemRequest
		want  int64
	}{
		{
			name:  "empty",
			items: nil,
			want:  0,
		},
		{
			name: "single item",
			items: []CreateOrderItemRequest{
				{Quantity: 2, UnitPrice: 1000},
			},
			want: 2000,
		},
		{
			name: "multiple items",
			items: []CreateOrderItemRequest{
				{Quantity: 3, UnitPrice: 500},
				{Quantity: 1, UnitPrice: 2000},
			},
			want: 3500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateTotal(tt.items)
			if got != tt.want {
				t.Errorf("CalculateTotal() = %d, want %d", got, tt.want)
			}
		})
	}
}
