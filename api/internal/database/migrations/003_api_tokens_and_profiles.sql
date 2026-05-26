-- +goose Up

-- Profiles
CREATE TABLE profiles (
    id            UUID PRIMARY KEY DEFAULT uuidv7(),
    owner_user_id UUID NOT NULL REFERENCES users(id),
    slug          TEXT NOT NULL,
    name          TEXT NOT NULL,
    description   TEXT NOT NULL DEFAULT '',
    is_default    BOOLEAN NOT NULL DEFAULT false,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(owner_user_id, slug)
);
CREATE INDEX idx_profiles_owner ON profiles (owner_user_id);

-- Profile-Library join table
CREATE TABLE profile_libraries (
    id         UUID PRIMARY KEY DEFAULT uuidv7(),
    profile_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    library_id UUID NOT NULL REFERENCES libraries(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(profile_id, library_id)
);
CREATE INDEX idx_profile_libraries_profile ON profile_libraries (profile_id);

-- API Tokens
CREATE TABLE api_tokens (
    id           UUID PRIMARY KEY DEFAULT uuidv7(),
    user_id      UUID NOT NULL REFERENCES users(id),
    name         TEXT NOT NULL,
    token_hash   TEXT NOT NULL UNIQUE,
    token_prefix TEXT NOT NULL,
    profile_id   UUID REFERENCES profiles(id) ON DELETE CASCADE,
    last_used_at TIMESTAMPTZ,
    expires_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_api_tokens_user ON api_tokens (user_id);

-- Backfill: every existing user gets a 'default' profile
INSERT INTO profiles (owner_user_id, slug, name, description, is_default)
SELECT id, 'default', 'Default', 'Default profile', true
FROM users
ON CONFLICT (owner_user_id, slug) DO NOTHING;

-- Move legacy installs into each user's default profile
INSERT INTO profile_libraries (profile_id, library_id, created_at)
SELECT p.id, li.library_id, li.created_at
FROM library_installations li
JOIN profiles p
  ON p.owner_user_id = li.user_id AND p.is_default = true
ON CONFLICT (profile_id, library_id) DO NOTHING;

-- Legacy table is no longer read or written by application code
DROP TABLE library_installations;

-- +goose Down
CREATE TABLE IF NOT EXISTS library_installations (
    id          UUID PRIMARY KEY DEFAULT uuidv7(),
    user_id     UUID NOT NULL REFERENCES users(id),
    library_id  UUID NOT NULL REFERENCES libraries(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id, library_id)
);
DROP TABLE IF EXISTS api_tokens;
DROP TABLE IF EXISTS profile_libraries;
DROP TABLE IF EXISTS profiles;
