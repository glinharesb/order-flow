package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	GRPCAddr     string
	HTTPAddr     string
	DatabaseURL  string
	RedisAddr    string
	KafkaBrokers []string
	AuthToken    string
	RateLimitRPS int

	// Cache
	CacheTTL time.Duration

	// Kafka
	KafkaGroupID   string
	OutboxPollFreq time.Duration

	// Telemetry
	OTLPEndpoint string
	ServiceName  string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() Config {
	return Config{
		GRPCAddr:       envOr("GRPC_ADDR", ":50051"),
		HTTPAddr:       envOr("HTTP_ADDR", ":8081"),
		DatabaseURL:    envOr("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/orderflow?sslmode=disable"),
		RedisAddr:      envOr("REDIS_ADDR", "localhost:6379"),
		KafkaBrokers:   []string{envOr("KAFKA_BROKERS", "localhost:9092")},
		AuthToken:      envOr("AUTH_TOKEN", ""),
		RateLimitRPS:   envOrInt("RATE_LIMIT_RPS", 100),
		CacheTTL:       envOrDuration("CACHE_TTL", 5*time.Minute),
		KafkaGroupID:   envOr("KAFKA_GROUP_ID", "order-flow"),
		OutboxPollFreq: envOrDuration("OUTBOX_POLL_FREQ", 1*time.Second),
		OTLPEndpoint:   envOr("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
		ServiceName:    envOr("SERVICE_NAME", "order-flow"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envOrInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envOrDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
