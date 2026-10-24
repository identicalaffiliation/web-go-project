-- +goose Up
CREATE TABLE IF NOT EXISTS accounts (
    user_id UUID PRIMARY KEY,
    balance BIGINT NOT NULL CHECK(balance >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- inbox консьюмера: event_id обработанных событий users.created
CREATE TABLE IF NOT EXISTS processed_events (
    event_id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TYPE payment_operation AS ENUM (
    'charge',
    'refund'
);

-- журнал списаний и возвратов, он же ключ идемпотентности активити саги:
-- одна операция каждого типа на заказ
CREATE TABLE IF NOT EXISTS payment_operations (
    order_id UUID NOT NULL,
    operation payment_operation NOT NULL,
    user_id UUID NOT NULL,
    amount BIGINT NOT NULL CHECK(amount >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (order_id, operation)
);

-- +goose Down
DROP TABLE IF EXISTS payment_operations;
DROP TYPE IF EXISTS payment_operation;
DROP TABLE IF EXISTS processed_events;
DROP TABLE IF EXISTS accounts;
