-- +goose Up
-- product_likes дописали в уже применённую 20260930120631, поэтому в базах, созданных до этого,
-- таблицы нет и GET /profiles/likes падает с 500. Здесь она докатывается; на новых базах - no-op
CREATE TABLE IF NOT EXISTS product_likes (
    user_id UUID NOT NULL,
    product_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, product_id)
);
CREATE INDEX IF NOT EXISTS idx_product_likes_user_id ON product_likes (user_id);

-- +goose Down
-- таблица принадлежит 20260930120631 и удаляется её откатом
