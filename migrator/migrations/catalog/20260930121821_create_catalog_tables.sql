-- +goose Up
CREATE TABLE IF NOT EXISTS catalog (
    product_id UUID PRIMARY KEY,
    title TEXT NOT NULL CHECK (LENGTH(title) > 0),
    description TEXT,
    price BIGINT NOT NULL,
    image_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(image_key)
);
-- +goose Down
SELECT 'down SQL query';
