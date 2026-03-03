package server

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/glinharesb/order-flow/gen/order/v1"
	"github.com/glinharesb/order-flow/internal/domain"
	"github.com/glinharesb/order-flow/internal/service"
)

// OrderServer implements the gRPC OrderServiceServer.
type OrderServer struct {
	pb.UnimplementedOrderServiceServer
	svc *service.OrderService
}

// NewOrderServer creates a new gRPC order server.
func NewOrderServer(svc *service.OrderService) *OrderServer {
	return &OrderServer{svc: svc}
}

func (s *OrderServer) CreateOrder(ctx context.Context, req *pb.CreateOrderRequest) (*pb.CreateOrderResponse, error) {
	items := make([]domain.CreateOrderItemRequest, len(req.Items))
	for i, item := range req.Items {
		items[i] = domain.CreateOrderItemRequest{
			ProductID: item.ProductId,
			Quantity:  item.Quantity,
			UnitPrice: item.UnitPrice,
		}
	}

	order, err := s.svc.CreateOrder(ctx, domain.CreateOrderRequest{
		CustomerID:     req.CustomerId,
		Items:          items,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &pb.CreateOrderResponse{Order: toProtoOrder(order)}, nil
}

func (s *OrderServer) GetOrder(ctx context.Context, req *pb.GetOrderRequest) (*pb.GetOrderResponse, error) {
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid order id: %v", err)
	}

	order, err := s.svc.GetOrder(ctx, id)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &pb.GetOrderResponse{Order: toProtoOrder(order)}, nil
}

func (s *OrderServer) ListOrders(ctx context.Context, req *pb.ListOrdersRequest) (*pb.ListOrdersResponse, error) {
	filter := domain.ListOrdersFilter{
		CustomerID: req.CustomerId,
		PageSize:   int(req.PageSize),
		PageToken:  req.PageToken,
	}
	if req.Status != pb.OrderStatus_ORDER_STATUS_UNSPECIFIED {
		filter.Status = fromProtoStatus(req.Status)
	}

	orders, total, err := s.svc.ListOrders(ctx, filter)
	if err != nil {
		return nil, toGRPCError(err)
	}

	pbOrders := make([]*pb.Order, len(orders))
	for i, o := range orders {
		pbOrders[i] = toProtoOrder(o)
	}

	var nextToken string
	if len(orders) > 0 && len(orders) == filter.PageSize {
		nextToken = orders[len(orders)-1].ID.String()
	}

	return &pb.ListOrdersResponse{
		Orders:        pbOrders,
		NextPageToken: nextToken,
		TotalCount:    int32(total),
	}, nil
}

func (s *OrderServer) CancelOrder(ctx context.Context, req *pb.CancelOrderRequest) (*pb.CancelOrderResponse, error) {
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid order id: %v", err)
	}

	order, err := s.svc.CancelOrder(ctx, id)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &pb.CancelOrderResponse{Order: toProtoOrder(order)}, nil
}

func toProtoOrder(o domain.Order) *pb.Order {
	items := make([]*pb.OrderItem, len(o.Items))
	for i, item := range o.Items {
		items[i] = &pb.OrderItem{
			ProductId: item.ProductID,
			Quantity:  item.Quantity,
			UnitPrice: item.UnitPrice,
		}
	}

	return &pb.Order{
		Id:             o.ID.String(),
		CustomerId:     o.CustomerID,
		Status:         toProtoStatus(o.Status),
		Items:          items,
		TotalAmount:    o.TotalAmount,
		IdempotencyKey: o.IdempotencyKey,
		CreatedAt:      timestamppb.New(o.CreatedAt),
		UpdatedAt:      timestamppb.New(o.UpdatedAt),
	}
}

func toProtoStatus(s domain.OrderStatus) pb.OrderStatus {
	switch s {
	case domain.StatusCreated:
		return pb.OrderStatus_ORDER_STATUS_CREATED
	case domain.StatusPaymentPending:
		return pb.OrderStatus_ORDER_STATUS_PAYMENT_PENDING
	case domain.StatusConfirmed:
		return pb.OrderStatus_ORDER_STATUS_CONFIRMED
	case domain.StatusShipped:
		return pb.OrderStatus_ORDER_STATUS_SHIPPED
	case domain.StatusDelivered:
		return pb.OrderStatus_ORDER_STATUS_DELIVERED
	case domain.StatusCancelled:
		return pb.OrderStatus_ORDER_STATUS_CANCELLED
	default:
		return pb.OrderStatus_ORDER_STATUS_UNSPECIFIED
	}
}

func fromProtoStatus(s pb.OrderStatus) domain.OrderStatus {
	switch s {
	case pb.OrderStatus_ORDER_STATUS_CREATED:
		return domain.StatusCreated
	case pb.OrderStatus_ORDER_STATUS_PAYMENT_PENDING:
		return domain.StatusPaymentPending
	case pb.OrderStatus_ORDER_STATUS_CONFIRMED:
		return domain.StatusConfirmed
	case pb.OrderStatus_ORDER_STATUS_SHIPPED:
		return domain.StatusShipped
	case pb.OrderStatus_ORDER_STATUS_DELIVERED:
		return domain.StatusDelivered
	case pb.OrderStatus_ORDER_STATUS_CANCELLED:
		return domain.StatusCancelled
	default:
		return ""
	}
}

func toGRPCError(err error) error {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return status.Errorf(codes.NotFound, "%v", err)
	case errors.Is(err, domain.ErrInvalidInput):
		return status.Errorf(codes.InvalidArgument, "%v", err)
	case errors.Is(err, domain.ErrDuplicateOrder):
		return status.Errorf(codes.AlreadyExists, "%v", err)
	case errors.Is(err, domain.ErrInvalidStatus):
		return status.Errorf(codes.FailedPrecondition, "%v", err)
	default:
		return status.Errorf(codes.Internal, "internal error")
	}
}
