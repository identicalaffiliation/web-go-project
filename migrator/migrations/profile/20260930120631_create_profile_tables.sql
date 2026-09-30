-- +goose Up
CREATE TYPE profile_type AS ENUM (
    'business',
    'customer'
);

CREATE TABLE IF NOT EXISTS profiles (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    profile_type profile_type NOT NULL,
    first_name VARCHAR(100) CHECK (LENGTH(first_name) >= 1),
    last_name VARCHAR(100) CHECK (LENGTH(last_name) >= 3),
    phone VARCHAR(12) CHECK (phone ~ '^(\+7|8|7)\d{10}$'), -- format: +71234567890
    company_name TEXT,
    idempotency_key UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(phone, company_name, idempotency_key) -- idempotency_key - event_id from auth outbox table
);

-- +goose Down
DROP TABLE IF EXISTS profiles;
DROP TYPE IF EXISTS profile_type;