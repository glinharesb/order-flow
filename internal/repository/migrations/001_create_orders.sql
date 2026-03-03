CREATE TABLE IF NOT EXISTS orders (
    id              UUID PRIMARY KEY,
    customer_id     TEXT NOT NULL,
    status          TEXT NOT NULL CHECK (status IN ('created','payment_pending','confirmed','shipped','delivered','cancelled')),
    total_amount    BIGINT NOT NULL DEFAULT 0,
    idempotency_key TEXT NOT NULL UNIQUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_orders_customer_id ON orders (customer_id);
CREATE INDEX idx_orders_status ON orders (status);
