-- +goose Up
ALTER TABLE sessions
    ADD COLUMN device_id TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions
    ADD COLUMN device_name TEXT NOT NULL DEFAULT '';
ALTER TABLE magic_links
    ADD COLUMN authorized BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE magic_links
    ADD COLUMN user_id TEXT;
ALTER TABLE magic_links
    ADD COLUMN otp_attempts INTEGER NOT NULL DEFAULT 0;

CREATE INDEX idx_magic_links_device_code ON magic_links (device_code, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_magic_links_device_code;
ALTER TABLE magic_links
    DROP COLUMN IF EXISTS otp_attempts;
ALTER TABLE magic_links
    DROP COLUMN IF EXISTS user_id;
ALTER TABLE magic_links
    DROP COLUMN IF EXISTS authorized;
ALTER TABLE sessions
    DROP COLUMN IF EXISTS device_name;
ALTER TABLE sessions
    DROP COLUMN IF EXISTS device_id;
