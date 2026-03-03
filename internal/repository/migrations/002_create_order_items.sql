CREATE TABLE IF NOT EXISTS order_items (
    id          UUID PRIMARY KEY,
    order_id    UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id  TEXT NOT NULL,
    quantity    INT NOT NULL CHECK (quantity > 0),
    unit_price  BIGINT NOT NULL CHECK (unit_price > 0)
);

CREATE INDEX idx_order_items_order_id ON order_items (order_id);
