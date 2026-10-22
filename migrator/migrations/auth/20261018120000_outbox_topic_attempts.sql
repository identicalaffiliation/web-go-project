-- +goose Up
ALTER TABLE outbox ADD COLUMN topic TEXT NOT NULL DEFAULT 'users.created';
-- дефолт нужен только чтобы добавить NOT NULL колонку в уже существующую таблицу
ALTER TABLE outbox ALTER COLUMN topic DROP DEFAULT;
ALTER TABLE outbox ADD COLUMN attempts INT NOT NULL DEFAULT 0;

-- признак отправки теперь только sent_at (как в catalog), status дублировал его
DROP INDEX IF EXISTS not_sent_events;
ALTER TABLE outbox DROP COLUMN status;
DROP TYPE IF EXISTS event_status;

CREATE INDEX IF NOT EXISTS outbox_unsent_created_at ON outbox(created_at) WHERE sent_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS outbox_unsent_created_at;

CREATE TYPE event_status AS ENUM (
    'pending',
    'processing',
    'sent'
);
ALTER TABLE outbox ADD COLUMN status event_status NOT NULL DEFAULT 'pending';
UPDATE outbox SET status = 'sent' WHERE sent_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS not_sent_events ON outbox(created_at) WHERE status = 'pending';

ALTER TABLE outbox DROP COLUMN attempts;
ALTER TABLE outbox DROP COLUMN topic;
