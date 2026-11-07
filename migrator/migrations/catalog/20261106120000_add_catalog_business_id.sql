-- +goose Up
-- владелец товара: user_id бизнеса (X-User-Id от gateway) при выкладывании в каталог.
-- у товаров, созданных до миграции, владельца нет - остаётся NULL
ALTER TABLE catalog ADD COLUMN IF NOT EXISTS business_id UUID;

-- товары бизнеса с курсорной пагинацией по product_id
CREATE INDEX IF NOT EXISTS business_products ON catalog(business_id, product_id DESC);

-- +goose Down
DROP INDEX IF EXISTS business_products;
ALTER TABLE catalog DROP COLUMN IF EXISTS business_id;
