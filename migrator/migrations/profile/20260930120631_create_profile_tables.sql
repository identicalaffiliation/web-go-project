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
    phone VARCHAR(12) CHECK (phone ~ '^(\+7|8|7)\d{10}$'),
    company_name TEXT,
    idempotency_key UUID NOT NULL UNIQUE, -- Ключ события уникален САМ ПО СЕБЕ (защита от дублей Кафки)
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(user_id, profile_type) -- Один юзер = максимум один профиль каждого типа
);

-- Индекс для мгновенного поиска профилей при авторизации
CREATE INDEX idx_profiles_user_id ON profiles(user_id);

-- +goose Down
DROP INDEX IF EXISTS idx_profiles_user_id;
DROP TABLE IF EXISTS profiles;
DROP TYPE IF EXISTS profile_type;