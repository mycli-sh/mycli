-- +goose Up
ALTER TABLE sessions
    ADD COLUMN device_id TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions
    ADD COLUMN device_name TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE sessions
    DROP COLUMN IF EXISTS device_name;
ALTER TABLE sessions
    DROP COLUMN IF EXISTS device_id;
