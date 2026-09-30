-- +goose Up
CREATE TYPE user_role AS ENUM (
    'business',
    'customer'
);

CREATE TYPE event_status AS ENUM (
    'pending',
    'processing',
    'sent'
);

CREATE TABLE IF NOT EXISTS auth_users (
    id UUID PRIMARY KEY,
    email TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    role user_role NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CHECK(email = lower(email)), -- lowercase
    CHECK(email ~ '^[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}$'),  -- classic format a@a.ru/com
    UNIQUE(email)
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
DROP TABLE IF EXISTS auth_users;
DROP TYPE IF EXISTS event_status;
DROP TYPE IF EXISTS user_role;