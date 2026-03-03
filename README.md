# order-flow

Order processing system with event-driven architecture built in Go. Demonstrates gRPC API, Kafka event streaming, PostgreSQL with transactional outbox pattern, Redis caching and idempotency, and OpenTelemetry distributed tracing.

## Architecture

**Request flow:** Client → gRPC Interceptors → OrderService → PostgreSQL / Redis / Outbox

**Event flow:** Outbox Relay → Kafka → EventProcessor (with DLQ)

| Component | Responsibility |
|-----------|---------------|
| **gRPC Interceptors** | Recovery, logging, rate limiting, auth, OpenTelemetry |
| **OrderService** | Business logic, cache-aside reads, idempotent writes |
| **PostgreSQL** | Orders, items, outbox table (single transaction) |
| **Redis** | Cache-aside for reads, `SET NX` idempotency store |
| **Outbox Relay** | Polls unpublished outbox entries, publishes to Kafka |
| **Kafka** | Event streaming (`order.created`, `order.cancelled`) |
| **EventProcessor** | Consumes events, DLQ after 3 failed retries |

## Order State Machine

```
created → payment_pending → confirmed → shipped → delivered
   ↓           ↓                ↓
 cancelled   cancelled       cancelled
```

## Key Patterns

- **Transactional Outbox**: Order, items, and outbox event written in a single database transaction. Guarantees at-least-once delivery.
- **Outbox Relay**: Background goroutine polls unpublished outbox entries and publishes to Kafka.
- **Idempotency**: Redis `SET NX` with client-provided idempotency key prevents duplicate order creation.
- **Dead Letter Queue**: After 3 failed retries, messages are routed to `order.dlq` topic.
- **Cache-aside**: `GetOrder` checks Redis first, populates on miss from PostgreSQL.
- **OTel Trace Propagation**: Trace context flows through gRPC interceptors and Kafka headers.

## API

gRPC service `order.v1.OrderService`:

| RPC | Description |
|-----|-------------|
| `CreateOrder` | Place a new order (requires idempotency key) |
| `GetOrder` | Retrieve order by ID |
| `ListOrders` | List orders with optional filters (customer, status, pagination) |
| `CancelOrder` | Cancel an order (validates state machine) |

## Quickstart

### Prerequisites

- Go 1.25+
- Docker and Docker Compose
- `protoc` with Go plugins (for proto regeneration)

### Run with Docker Compose

```bash
# Start all services (PostgreSQL, Redis, Kafka, Jaeger, order-server)
docker compose up -d

# Verify health
curl http://localhost:8081/healthz
curl http://localhost:8081/readyz

# View Prometheus metrics
curl http://localhost:8081/metrics

# View traces in Jaeger UI
open http://localhost:16686

# Stop and clean up
docker compose down -v
```

### gRPC API Examples

```bash
# Create an order
grpcurl -plaintext -d '{
  "customer_id": "cust-123",
  "idempotency_key": "order-abc-001",
  "items": [
    {"product_id": "prod-1", "quantity": 2, "unit_price": 1500},
    {"product_id": "prod-2", "quantity": 1, "unit_price": 3000}
  ]
}' localhost:50051 order.v1.OrderService/CreateOrder

# Get an order
grpcurl -plaintext -d '{"id": "<order-uuid>"}' \
  localhost:50051 order.v1.OrderService/GetOrder

# List orders by customer
grpcurl -plaintext -d '{"customer_id": "cust-123", "page_size": 10}' \
  localhost:50051 order.v1.OrderService/ListOrders

# Cancel an order
grpcurl -plaintext -d '{"id": "<order-uuid>", "reason": "changed my mind"}' \
  localhost:50051 order.v1.OrderService/CancelOrder
```

### Local Development

```bash
# Build
make build

# Run tests (unit + integration)
make test

# Run unit tests only
make test-short

# Lint
make lint

# Regenerate protobuf
make proto

# Build Docker image
make docker
```

## Kafka Topics

| Topic | Producer | Consumer |
|-------|----------|----------|
| `order.created` | Outbox relay | EventProcessor |
| `order.cancelled` | Outbox relay | EventProcessor |
| `payment.processed` | External (simulated) | EventProcessor |
| `order.dlq` | Consumer (retry) | Manual review |

## Database Schema

- **orders**: UUID PK, customer_id, status (CHECK constraint), total_amount, idempotency_key (UNIQUE), timestamps
- **order_items**: UUID PK, FK to orders (CASCADE), product_id, quantity, unit_price
- **outbox**: UUID PK, event_type, payload (JSONB), published_at (nullable, indexed WHERE NULL)

## Configuration

All configuration is via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `GRPC_ADDR` | `:50051` | gRPC listen address |
| `HTTP_ADDR` | `:8081` | HTTP health/metrics address |
| `DATABASE_URL` | `postgres://...localhost:5432/orderflow` | PostgreSQL connection string |
| `REDIS_ADDR` | `localhost:6379` | Redis address |
| `KAFKA_BROKERS` | `localhost:9092` | Kafka bootstrap servers |
| `AUTH_TOKEN` | (empty = disabled) | Bearer token for gRPC auth |
| `RATE_LIMIT_RPS` | `100` | Requests per second limit |
| `CACHE_TTL` | `5m` | Redis cache TTL |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `localhost:4317` | OTLP collector endpoint |
| `SERVICE_NAME` | `order-flow` | OTel service name |

## Stack

- **gRPC** — API layer with interceptor chain
- **PostgreSQL** (pgx/v5) — Primary storage with embedded SQL migrations
- **Redis** (go-redis/v9) — Cache-aside + idempotency store
- **Kafka** (segmentio/kafka-go) — Event streaming with transactional outbox
- **OpenTelemetry** — Distributed tracing (OTLP) + Prometheus metrics bridge
- **Jaeger** — Trace visualization
- **Kubernetes** — Production deployment manifests with HPA
