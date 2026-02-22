-- +goose Up
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS users (
    id          TEXT PRIMARY KEY DEFAULT 'usr_' || gen_random_uuid()::text,
    email       TEXT NOT NULL,
    username    TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX users_email_lower_idx ON users (lower(email));
CREATE UNIQUE INDEX users_username_lower_idx ON users (lower(username)) WHERE username IS NOT NULL;

CREATE TABLE IF NOT EXISTS libraries (
    id              TEXT PRIMARY KEY DEFAULT 'lib_' || gen_random_uuid()::text,
    owner_id        TEXT REFERENCES users(id),
    slug            TEXT NOT NULL,
    name            TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    git_url         TEXT,
    is_public       BOOLEAN NOT NULL DEFAULT false,
    install_count   INTEGER NOT NULL DEFAULT 0,
    latest_version  TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(owner_id, slug)
);

CREATE TABLE IF NOT EXISTS commands (
    id              TEXT PRIMARY KEY DEFAULT 'cmd_' || gen_random_uuid()::text,
    owner_user_id   TEXT NOT NULL REFERENCES users(id),
    name            TEXT NOT NULL,
    slug            TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    tags            JSONB NOT NULL DEFAULT '[]',
    library_id      TEXT REFERENCES libraries(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ,
    UNIQUE(owner_user_id, slug)
);

CREATE INDEX idx_commands_owner ON commands (owner_user_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX idx_commands_library_slug
    ON commands (library_id, slug) WHERE library_id IS NOT NULL AND deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS command_versions (
    id          TEXT PRIMARY KEY DEFAULT 'cv_' || gen_random_uuid()::text,
    command_id  TEXT NOT NULL REFERENCES commands(id),
    version     INTEGER NOT NULL,
    spec_json   JSONB NOT NULL,
    spec_hash   TEXT NOT NULL,
    message     TEXT NOT NULL DEFAULT '',
    created_by  TEXT NOT NULL REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(command_id, version)
);

CREATE INDEX idx_versions_command ON command_versions (command_id, version DESC);

CREATE TABLE IF NOT EXISTS magic_links (
    id          TEXT PRIMARY KEY DEFAULT 'ml_' || gen_random_uuid()::text,
    email       TEXT NOT NULL,
    token_hash  TEXT NOT NULL,
    device_code TEXT NOT NULL,
    otp_hash    TEXT,
    expires_at  TIMESTAMPTZ NOT NULL,
    used_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_magic_links_token ON magic_links (token_hash);
CREATE INDEX idx_magic_links_otp_hash ON magic_links (otp_hash)
    WHERE otp_hash IS NOT NULL AND used_at IS NULL;

CREATE TABLE IF NOT EXISTS sessions (
    id                 TEXT PRIMARY KEY DEFAULT 'ses_' || gen_random_uuid()::text,
    user_id            TEXT NOT NULL REFERENCES users(id),
    refresh_token_hash TEXT NOT NULL UNIQUE,
    user_agent         TEXT NOT NULL DEFAULT '',
    ip_address         TEXT NOT NULL DEFAULT '',
    last_used_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at         TIMESTAMPTZ NOT NULL,
    revoked_at         TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_sessions_user ON sessions (user_id) WHERE revoked_at IS NULL;

CREATE TABLE IF NOT EXISTS library_releases (
    id            TEXT PRIMARY KEY DEFAULT 'lr_' || gen_random_uuid()::text,
    library_id    TEXT NOT NULL REFERENCES libraries(id),
    version       TEXT NOT NULL,
    tag           TEXT NOT NULL,
    commit_hash   TEXT NOT NULL DEFAULT '',
    command_count INTEGER NOT NULL DEFAULT 0,
    released_by   TEXT NOT NULL REFERENCES users(id),
    released_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(library_id, version)
);

CREATE INDEX idx_library_releases_library
    ON library_releases (library_id, released_at DESC);

CREATE TABLE IF NOT EXISTS library_installations (
    id          TEXT PRIMARY KEY DEFAULT 'li_' || gen_random_uuid()::text,
    user_id     TEXT NOT NULL REFERENCES users(id),
    library_id  TEXT NOT NULL REFERENCES libraries(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id, library_id)
);

-- Seed data

INSERT INTO users (id, email, username)
VALUES ('usr_system', 'system@mycli.sh', 'system')
ON CONFLICT (id) DO NOTHING;

INSERT INTO libraries (id, owner_id, slug, name, description, is_public)
VALUES ('lib_kubernetes', 'usr_system', 'kubernetes', 'Kubernetes', 'Commands for Kubernetes cluster management', true)
ON CONFLICT (id) DO NOTHING;

-- Commands are now imported via `my library release` from the mycli-libraries repo.
-- See scripts/dev-reset.sh for the dev workflow.
