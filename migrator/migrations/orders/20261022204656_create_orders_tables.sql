-- +goose Up
CREATE TYPE order_status AS ENUM (
    'created',
    'paid',
    'failed'
);

CREATE TABLE IF NOT EXISTS orders (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    product_id UUID NOT NULL,
    operation_id UUID NOT NULL UNIQUE,
    quantity SMALLINT NOT NULL CHECK(quantity > 0),
    total_price BIGINT NOT NULL CHECK(total_price > 0),
    status order_status NOT NULL DEFAULT 'created',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_orders_by_user_id ON orders(user_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS orders;
DROP TYPE IF EXISTS order_status;

