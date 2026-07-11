-- name: CreateSession :one
INSERT INTO sessions (user_id, refresh_token_hash, user_agent, ip_address, device_id, device_name, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, user_id, refresh_token_hash, user_agent, ip_address, device_id, device_name, last_used_at, expires_at, revoked_at, created_at, previous_refresh_token_hash, previous_hash_valid_until;

-- name: ListSessionsByUser :many
SELECT id, user_id, refresh_token_hash, user_agent, ip_address, device_id, device_name, last_used_at, expires_at, revoked_at, created_at, previous_refresh_token_hash, previous_hash_valid_until
FROM sessions
WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > now()
ORDER BY last_used_at DESC;

-- name: GetSessionByTokenHash :one
-- Matches the current refresh token, or the immediately-previous one while it is
-- still within its reuse-grace window. The session itself must not have expired.
SELECT id, user_id, refresh_token_hash, user_agent, ip_address, device_id, device_name, last_used_at, expires_at, revoked_at, created_at, previous_refresh_token_hash, previous_hash_valid_until
FROM sessions
WHERE (refresh_token_hash = $1
       OR (previous_refresh_token_hash = $1 AND previous_hash_valid_until > NOW()))
  AND expires_at > NOW();

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

-- name: UpdateSessionRefreshTokenHash :exec
-- Rotates the refresh token, demoting the current hash to previous_refresh_token_hash
-- and keeping it valid until $4 (the reuse-grace deadline). All SET right-hand
-- sides reference the pre-update row, so previous_refresh_token_hash captures the
-- hash being superseded.
UPDATE sessions
SET previous_refresh_token_hash = refresh_token_hash,
    previous_hash_valid_until   = $4,
    refresh_token_hash          = $2,
    expires_at                  = $3
WHERE id = $1;
