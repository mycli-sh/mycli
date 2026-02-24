-- +goose Up
CREATE TABLE device_sessions
(
    id           TEXT PRIMARY KEY     DEFAULT 'ds_' || gen_random_uuid()::text,
    device_code  TEXT        NOT NULL UNIQUE,
    user_code    TEXT        NOT NULL,
    email        TEXT        NOT NULL DEFAULT '',
    expires_at   TIMESTAMPTZ NOT NULL,
    authorized   BOOLEAN     NOT NULL DEFAULT false,
    user_id      TEXT,
    otp_attempts INTEGER     NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS device_sessions;
