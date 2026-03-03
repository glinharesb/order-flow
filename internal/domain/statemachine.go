package domain

import "fmt"

// transitions defines the valid state transitions for an order.
// Each key is a current status, and the value is the set of statuses it can transition to.
var transitions = map[OrderStatus]map[OrderStatus]bool{
	StatusCreated: {
		StatusPaymentPending: true,
		StatusCancelled:      true,
	},
	StatusPaymentPending: {
		StatusConfirmed: true,
		StatusCancelled: true,
	},
	StatusConfirmed: {
		StatusShipped:   true,
		StatusCancelled: true,
	},
	StatusShipped: {
		StatusDelivered: true,
	},
}

// Transition validates and returns nil if the state change from → to is allowed.
func Transition(from, to OrderStatus) error {
	allowed, ok := transitions[from]
	if !ok {
		return fmt.Errorf("%w: no transitions from %s", ErrInvalidStatus, from)
	}
	if !allowed[to] {
		return fmt.Errorf("%w: cannot transition from %s to %s", ErrInvalidStatus, from, to)
	}
	return nil
}
