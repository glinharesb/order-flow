package domain

import (
	"errors"
	"testing"
)

func TestTransition(t *testing.T) {
	tests := []struct {
		name    string
		from    OrderStatus
		to      OrderStatus
		wantErr bool
	}{
		// Valid transitions
		{"created → payment_pending", StatusCreated, StatusPaymentPending, false},
		{"created → cancelled", StatusCreated, StatusCancelled, false},
		{"payment_pending → confirmed", StatusPaymentPending, StatusConfirmed, false},
		{"payment_pending → cancelled", StatusPaymentPending, StatusCancelled, false},
		{"confirmed → shipped", StatusConfirmed, StatusShipped, false},
		{"confirmed → cancelled", StatusConfirmed, StatusCancelled, false},
		{"shipped → delivered", StatusShipped, StatusDelivered, false},

		// Invalid transitions
		{"created → confirmed", StatusCreated, StatusConfirmed, true},
		{"created → shipped", StatusCreated, StatusShipped, true},
		{"created → delivered", StatusCreated, StatusDelivered, true},
		{"payment_pending → shipped", StatusPaymentPending, StatusShipped, true},
		{"confirmed → payment_pending", StatusConfirmed, StatusPaymentPending, true},
		{"shipped → cancelled", StatusShipped, StatusCancelled, true},
		{"delivered → anything", StatusDelivered, StatusCancelled, true},
		{"cancelled → anything", StatusCancelled, StatusCreated, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Transition(tt.from, tt.to)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.wantErr && !errors.Is(err, ErrInvalidStatus) {
				t.Errorf("expected ErrInvalidStatus, got %v", err)
			}
		})
	}
}

func TestTransition_AllTerminalStatesAreTerminal(t *testing.T) {
	terminal := []OrderStatus{StatusDelivered, StatusCancelled}
	allStatuses := []OrderStatus{
		StatusCreated, StatusPaymentPending, StatusConfirmed,
		StatusShipped, StatusDelivered, StatusCancelled,
	}

	for _, from := range terminal {
		for _, to := range allStatuses {
			err := Transition(from, to)
			if err == nil {
				t.Errorf("terminal state %s should not transition to %s", from, to)
			}
		}
	}
}
