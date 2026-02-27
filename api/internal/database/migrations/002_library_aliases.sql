-- +goose Up
ALTER TABLE libraries
    ADD COLUMN aliases TEXT[] NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE libraries
    DROP COLUMN aliases;
