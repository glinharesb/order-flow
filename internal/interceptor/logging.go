package interceptor

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// LoggingUnary returns a unary interceptor that logs request metadata.
func LoggingUnary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start)

		code := status.Code(err)
		slog.Info("grpc request",
			"method", info.FullMethod,
			"code", code.String(),
			"duration_ms", duration.Milliseconds(),
		)

		return resp, err
	}
}
