-- +goose Up
CREATE TYPE event_status AS ENUM (
    'pending',
    'processing',
    'sent'
);

CREATE TABLE IF NOT EXISTS catalog (
    product_id UUID PRIMARY KEY,
    title TEXT NOT NULL CHECK (LENGTH(title) > 0),
    description TEXT,
    price BIGINT NOT NULL CHECK (price > 0),
    image_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(image_key)
);

CREATE TABLE IF NOT EXISTS outbox (
    id UUID PRIMARY KEY, -- event_id to kafka header
    key UUID NOT NULL, -- kafka key
    payload JSONB NOT NULL, -- kafka body,
    status event_status NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS not_sent_events ON outbox(created_at) WHERE status = 'pending';

-- +goose Down
DROP TABLE IF EXISTS outbox;
DROP TABLE IF EXISTS catalog;
DROP TYPE IF EXISTS event_status;
