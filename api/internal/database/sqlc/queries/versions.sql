-- name: CreateVersion :one
INSERT INTO command_versions (command_id, version, spec_json, spec_hash, message, created_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, command_id, version, spec_json, spec_hash, message, created_by, created_at;

-- name: GetVersionByCommandAndVersion :one
SELECT id, command_id, version, spec_json, spec_hash, message, created_by, created_at
FROM command_versions
WHERE command_id = $1 AND version = $2;

-- name: GetLatestVersionByCommand :one
SELECT id, command_id, version, spec_json, spec_hash, message, created_by, created_at
FROM command_versions
WHERE command_id = $1
ORDER BY version DESC
LIMIT 1;

-- name: GetLatestHashByCommand :one
SELECT spec_hash
FROM command_versions
WHERE command_id = $1
ORDER BY version DESC
LIMIT 1;

-- name: ListVersionsByCommand :many
SELECT id, command_id, version, spec_json, spec_hash, message, created_by, created_at
FROM command_versions
WHERE command_id = $1
ORDER BY version DESC;
