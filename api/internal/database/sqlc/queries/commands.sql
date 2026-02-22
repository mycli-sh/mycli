-- name: CreateCommand :one
INSERT INTO commands (owner_user_id, name, slug, description, tags)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, owner_user_id, name, slug, description, tags, library_id, created_at, updated_at, deleted_at;

-- name: GetCommandByID :one
SELECT id, owner_user_id, name, slug, description, tags, library_id, created_at, updated_at, deleted_at
FROM commands
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetCommandByOwnerAndSlug :one
SELECT id, owner_user_id, name, slug, description, tags, library_id, created_at, updated_at, deleted_at
FROM commands
WHERE owner_user_id = $1 AND slug = $2 AND deleted_at IS NULL;

-- name: GetCommandByLibraryAndSlug :one
SELECT id, owner_user_id, name, slug, description, tags, library_id, created_at, updated_at, deleted_at
FROM commands
WHERE library_id = $1 AND slug = $2 AND deleted_at IS NULL;

-- name: CreateCommandForLibrary :one
INSERT INTO commands (owner_user_id, library_id, name, slug, description, tags)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, owner_user_id, name, slug, description, tags, library_id, created_at, updated_at, deleted_at;

-- name: CountCommandsByOwner :one
SELECT count(*) FROM commands WHERE owner_user_id = $1 AND deleted_at IS NULL AND library_id IS NULL;

-- name: UpdateCommandMeta :exec
UPDATE commands
SET name = $2, description = $3, tags = $4, updated_at = now()
WHERE id = $1 AND deleted_at IS NULL;

-- name: SoftDeleteCommand :execrows
UPDATE commands SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL;
