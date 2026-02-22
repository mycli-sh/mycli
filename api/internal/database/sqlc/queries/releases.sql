-- name: CreateLibraryRelease :one
INSERT INTO library_releases (library_id, version, tag, commit_hash, command_count, released_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, library_id, version, tag, commit_hash, command_count, released_by, released_at;

-- name: GetLibraryRelease :one
SELECT id, library_id, version, tag, commit_hash, command_count, released_by, released_at
FROM library_releases
WHERE library_id = $1 AND version = $2;

-- name: ListLibraryReleases :many
SELECT id, library_id, version, tag, commit_hash, command_count, released_by, released_at
FROM library_releases
WHERE library_id = $1
ORDER BY released_at DESC;

-- name: LibraryReleaseExists :one
SELECT EXISTS(SELECT 1 FROM library_releases WHERE library_id = $1 AND version = $2);
