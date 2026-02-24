-- name: CreateSession :one
INSERT INTO sessions (user_id, refresh_token_hash, user_agent, ip_address, device_id, device_name, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, user_id, refresh_token_hash, user_agent, ip_address, device_id, device_name, last_used_at, expires_at, revoked_at, created_at;

-- name: ListSessionsByUser :many
SELECT id, user_id, refresh_token_hash, user_agent, ip_address, device_id, device_name, last_used_at, expires_at, revoked_at, created_at
FROM sessions
WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > now()
ORDER BY last_used_at DESC;

-- name: GetSessionByTokenHash :one
SELECT id, user_id, refresh_token_hash, user_agent, ip_address, device_id, device_name, last_used_at, expires_at, revoked_at, created_at
FROM sessions
WHERE refresh_token_hash = $1;

-- name: RevokeSession :execrows
UPDATE sessions SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL;

-- name: RevokeAllSessionsExcept :execrows
UPDATE sessions SET revoked_at = now()
WHERE user_id = $1 AND id != $2 AND revoked_at IS NULL;

-- name: RevokeSessionByDeviceID :exec
UPDATE sessions SET revoked_at = now()
WHERE user_id = $1 AND device_id = $2 AND device_id != '' AND revoked_at IS NULL;

-- name: UpdateSessionLastUsed :exec
UPDATE sessions SET last_used_at = now() WHERE id = $1;
