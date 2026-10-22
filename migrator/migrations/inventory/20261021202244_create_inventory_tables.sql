-- +goose Up
CREATE TABLE IF NOT EXISTS stock (
      product_id UUID PRIMARY KEY,
      quantity SMALLINT NOT NULL CHECK(quantity >= 0 AND quantity < 100),
      created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
      updated TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE idempotency_keys (
      order_id    UUID,
      operation   TEXT,
      created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
      PRIMARY KEY (order_id, operation)
);

-- +goose Down
DROP TABLE IF EXISTS stock;
DROP TABLE IF EXISTS idempotency_keys;