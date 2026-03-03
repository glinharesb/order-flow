package server

import (
	"testing"

	"github.com/glinharesb/order-flow/internal/domain"
	pb "github.com/glinharesb/order-flow/gen/order/v1"
)

func TestToProtoStatus_RoundTrip(t *testing.T) {
	statuses := []domain.OrderStatus{
		domain.StatusCreated,
		domain.StatusPaymentPending,
		domain.StatusConfirmed,
		domain.StatusShipped,
		domain.StatusDelivered,
		domain.StatusCancelled,
	}

	for _, s := range statuses {
		proto := toProtoStatus(s)
		back := fromProtoStatus(proto)
		if back != s {
			t.Errorf("roundtrip failed: %q → %v → %q", s, proto, back)
		}
	}
}

func TestToGRPCError(t *testing.T) {
	tests := []struct {
		err      error
		wantCode string
	}{
		{domain.ErrNotFound, "NotFound"},
		{domain.ErrInvalidInput, "InvalidArgument"},
		{domain.ErrDuplicateOrder, "AlreadyExists"},
		{domain.ErrInvalidStatus, "FailedPrecondition"},
	}

	for _, tt := range tests {
		grpcErr := toGRPCError(tt.err)
		if grpcErr == nil {
			t.Errorf("expected gRPC error for %v", tt.err)
		}
	}
}

func TestFromProtoStatus_Unspecified(t *testing.T) {
	got := fromProtoStatus(pb.OrderStatus_ORDER_STATUS_UNSPECIFIED)
	if got != "" {
		t.Errorf("expected empty string for UNSPECIFIED, got %q", got)
	}
}
