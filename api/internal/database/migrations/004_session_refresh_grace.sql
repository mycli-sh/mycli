-- +goose Up

-- Refresh-token reuse grace: on rotation, keep the immediately-previous token's
-- hash valid for a short window so a concurrent/duplicate refresh isn't rejected.
ALTER TABLE sessions
    ADD COLUMN previous_refresh_token_hash TEXT,
    ADD COLUMN previous_hash_valid_until   TIMESTAMPTZ;

-- +goose Down
ALTER TABLE sessions
    DROP COLUMN previous_refresh_token_hash,
    DROP COLUMN previous_hash_valid_until;
