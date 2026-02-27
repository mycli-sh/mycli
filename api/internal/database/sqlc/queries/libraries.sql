-- name: GetLibraryBySlug :one
SELECT id, owner_id, slug, name, description, git_url, is_public, install_count, latest_version, created_at, updated_at, aliases
FROM libraries WHERE slug = $1;

-- name: GetLibraryByOwnerSlug :one
SELECT id, owner_id, slug, name, description, git_url, is_public, install_count, latest_version, created_at, updated_at, aliases
FROM libraries WHERE owner_id = $1 AND slug = $2;

-- name: GetLibraryByOwnerUsernameAndSlug :one
SELECT l.id, l.owner_id, l.slug, l.name, l.description, l.git_url, l.is_public, l.install_count, l.latest_version, l.created_at, l.updated_at, l.aliases
FROM libraries l
JOIN users u ON u.id = l.owner_id
WHERE u.username = $1
  AND l.slug = $2
  AND l.is_public = true;

-- name: ListLibraries :many
SELECT id, owner_id, slug, name, description, git_url, is_public, install_count, latest_version, created_at, updated_at, aliases
FROM libraries ORDER BY name ASC;

-- name: CreateOrUpdateLibrary :one
INSERT INTO libraries (owner_id, slug, name, description, git_url, aliases, is_public)
VALUES ($1, $2, $3, $4, $5, $6, true)
ON CONFLICT (owner_id, slug)
DO UPDATE SET name = EXCLUDED.name,
              description = EXCLUDED.description,
              git_url = EXCLUDED.git_url,
              aliases = EXCLUDED.aliases,
              updated_at = now()
RETURNING id, owner_id, slug, name, description, git_url, is_public, install_count, latest_version, created_at, updated_at, aliases;

-- name: InstallLibrary :exec
INSERT INTO library_installations (user_id, library_id)
VALUES ($1, $2)
ON CONFLICT (user_id, library_id) DO NOTHING;

-- name: IncrementInstallCount :exec
UPDATE libraries SET install_count = install_count + 1 WHERE id = $1;

-- name: UninstallLibrary :execrows
DELETE FROM library_installations WHERE user_id = $1 AND library_id = $2;

-- name: DecrementInstallCount :exec
UPDATE libraries SET install_count = GREATEST(install_count - 1, 0) WHERE id = $1;

-- name: GetInstalledLibraries :many
SELECT l.id, l.owner_id, l.slug, l.name, l.description, l.git_url, l.is_public, l.install_count, l.latest_version, l.created_at, l.updated_at, l.aliases
FROM libraries l
JOIN library_installations li ON li.library_id = l.id
WHERE li.user_id = $1
ORDER BY l.name ASC;

-- name: IsLibraryInstalled :one
SELECT EXISTS(SELECT 1 FROM library_installations WHERE user_id = $1 AND library_id = $2);

-- name: ListCommandsByLibrary :many
SELECT id, slug, name, description, updated_at
FROM commands
WHERE library_id = $1 AND deleted_at IS NULL
ORDER BY slug ASC;

-- name: UpdateLibraryLatestVersion :exec
UPDATE libraries SET latest_version = $1, updated_at = now() WHERE id = $2;

-- name: GetOwnerName :one
SELECT username FROM users WHERE id = $1;
