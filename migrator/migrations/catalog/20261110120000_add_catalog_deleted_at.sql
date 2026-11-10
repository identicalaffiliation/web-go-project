-- +goose Up
-- мягкое удаление товара бизнесом: на строку ссылаются отзывы (FK) и история заказов, поэтому
-- она остаётся, а товар пропадает из выдачи, карточки и цены для заказов
ALTER TABLE catalog ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE catalog DROP COLUMN IF EXISTS deleted_at;
