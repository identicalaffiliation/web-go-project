-- +goose Up
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(token_hash)
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens(user_id);

-- +goose Down
DROP TABLE IF EXISTS refresh_tokens;