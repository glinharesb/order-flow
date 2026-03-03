package main

import (
	"context"
	"database/sql"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	promhttp "github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "github.com/glinharesb/order-flow/gen/order/v1"
	"github.com/glinharesb/order-flow/internal/cache"
	"github.com/glinharesb/order-flow/internal/config"
	"github.com/glinharesb/order-flow/internal/health"
	"github.com/glinharesb/order-flow/internal/interceptor"
	"github.com/glinharesb/order-flow/internal/kafka"
	"github.com/glinharesb/order-flow/internal/repository"
	"github.com/glinharesb/order-flow/internal/server"
	"github.com/glinharesb/order-flow/internal/service"
	"github.com/glinharesb/order-flow/internal/telemetry"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	slog.Info("starting order-flow", "grpc_addr", cfg.GRPCAddr, "http_addr", cfg.HTTPAddr)

	// OpenTelemetry.
	otelProvider, err := telemetry.Setup(context.Background(), telemetry.Config{
		ServiceName:  cfg.ServiceName,
		OTLPEndpoint: cfg.OTLPEndpoint,
	})
	if err != nil {
		slog.Warn("telemetry setup failed, continuing without tracing", "error", err)
	} else {
		defer otelProvider.Shutdown()
	}

	// Database.
	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		slog.Error("open database", "error", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := repository.Migrate(ctx, db); err != nil {
		slog.Error("migrations", "error", err)
		os.Exit(1)
	}

	// Redis.
	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	defer func() { _ = rdb.Close() }()

	// Repositories.
	orderRepo := repository.NewPostgresOrder(db)
	outboxRepo := repository.NewPostgresOutbox(db)

	// Cache and idempotency.
	redisCache := cache.NewRedisCache(rdb)
	idemStore := cache.NewIdempotencyStore(rdb, 24*time.Hour)

	// Kafka publisher.
	publisher := kafka.NewKafkaPublisher(cfg.KafkaBrokers)
	defer func() { _ = publisher.Close() }()

	// Services.
	orderSvc := service.NewOrderService(orderRepo, redisCache, idemStore, cfg.CacheTTL, logger)
	processor := service.NewEventProcessor(orderRepo, redisCache, logger)

	// Kafka consumer.
	consumer := kafka.NewEventConsumer(
		cfg.KafkaBrokers, cfg.KafkaGroupID,
		[]string{kafka.TopicOrderCreated},
		publisher, logger,
	)
	consumer.RegisterHandler(kafka.TopicOrderCreated, processor.HandleOrderCreated)
	consumer.RegisterHandler(kafka.TopicPaymentProcessed, processor.HandlePaymentProcessed)
	consumer.RegisterHandler(kafka.TopicOrderCancelled, processor.HandleOrderCancelled)

	// Outbox relay.
	relay := kafka.NewOutboxRelay(outboxRepo, publisher, cfg.OutboxPollFreq, logger)

	// gRPC server.
	grpcSrv := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(
			interceptor.RecoveryUnary(),
			interceptor.LoggingUnary(),
			interceptor.RateLimitUnary(cfg.RateLimitRPS),
			interceptor.AuthUnary(cfg.AuthToken),
		),
	)
	pb.RegisterOrderServiceServer(grpcSrv, server.NewOrderServer(orderSvc))
	reflection.Register(grpcSrv)

	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		slog.Error("listen grpc", "error", err)
		os.Exit(1)
	}

	// Health HTTP server with Prometheus metrics.
	healthSrv := health.NewServer(db)
	healthSrv.RegisterHandler("GET /metrics", promhttp.Handler())
	httpSrv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           healthSrv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Start all components.
	go func() {
		slog.Info("grpc server starting", "addr", cfg.GRPCAddr)
		if err := grpcSrv.Serve(lis); err != nil {
			slog.Error("grpc serve", "error", err)
		}
	}()

	go func() {
		slog.Info("http server starting", "addr", cfg.HTTPAddr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http serve", "error", err)
		}
	}()

	go func() {
		slog.Info("outbox relay starting")
		if err := relay.Run(ctx); err != nil {
			slog.Error("outbox relay", "error", err)
		}
	}()

	go func() {
		slog.Info("kafka consumer starting")
		if err := consumer.Run(ctx); err != nil {
			slog.Error("kafka consumer", "error", err)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	// Graceful shutdown with timeout.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		grpcSrv.GracefulStop()
		close(done)
	}()

	_ = httpSrv.Shutdown(shutdownCtx)
	_ = consumer.Close()

	select {
	case <-done:
		slog.Info("shutdown complete")
	case <-shutdownCtx.Done():
		slog.Warn("graceful shutdown timed out, forcing stop")
		grpcSrv.Stop()
	}
}
