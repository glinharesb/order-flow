package interceptor

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// AuthUnary returns a unary interceptor that validates a bearer token.
// If token is empty, auth is disabled (all requests pass).
func AuthUnary(token string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if token == "" {
			return handler(ctx, req)
		}

		// Skip auth for health/reflection endpoints.
		if strings.HasPrefix(info.FullMethod, "/grpc.health") || strings.HasPrefix(info.FullMethod, "/grpc.reflection") {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Errorf(codes.Unauthenticated, "missing metadata")
		}

		values := md.Get("authorization")
		if len(values) == 0 {
			return nil, status.Errorf(codes.Unauthenticated, "missing authorization header")
		}

		authVal := values[0]
		if !strings.HasPrefix(authVal, "Bearer ") {
			return nil, status.Errorf(codes.Unauthenticated, "invalid authorization format")
		}

		if strings.TrimPrefix(authVal, "Bearer ") != token {
			return nil, status.Errorf(codes.Unauthenticated, "invalid token")
		}

		return handler(ctx, req)
	}
}
