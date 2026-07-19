-- +goose Up

-- Per-release content hash used to make POST /v1/releases idempotent under retries:
-- identical content returns the existing release; different content returns 409
-- RELEASE_CONTENT_MISMATCH. Pre-existing rows carry NULL and fall back to the
-- legacy RELEASE_EXISTS behavior (no content comparison possible).
ALTER TABLE library_releases
    ADD COLUMN content_sha256 TEXT;

-- +goose Down
ALTER TABLE library_releases
    DROP COLUMN content_sha256;
