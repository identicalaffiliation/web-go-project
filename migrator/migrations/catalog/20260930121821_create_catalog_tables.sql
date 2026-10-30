-- +goose Up
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

CREATE TABLE IF NOT EXISTS outbox_stock (
    id UUID PRIMARY KEY, -- event_id to kafka header
    key UUID NOT NULL, -- kafka key
    payload JSONB NOT NULL, -- kafka body,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS outbox_likes (
    id UUID PRIMARY KEY,
    key UUID NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS outbox_likes_not_sent ON outbox_likes(created_at) WHERE sent_at IS NULL;
CREATE INDEX IF NOT EXISTS outbox_stock_not_sent ON outbox_stock(created_at) WHERE sent_at IS NULL;


-- +goose Down
DROP TABLE IF EXISTS outbox_stock;
DROP TABLE IF EXISTS outbox_likes;
DROP TABLE IF EXISTS catalog;
