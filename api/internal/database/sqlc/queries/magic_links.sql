-- name: CreateMagicLink :one
INSERT INTO magic_links (email, token_hash, device_code, otp_hash, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, email, token_hash, device_code, otp_hash, authorized, user_id, otp_attempts, expires_at, used_at, created_at;

-- name: GetMagicLinkByTokenHash :one
SELECT id, email, token_hash, device_code, otp_hash, authorized, user_id, otp_attempts, expires_at, used_at, created_at
FROM magic_links WHERE token_hash = $1 AND expires_at > NOW() AND used_at IS NULL;

-- name: GetMagicLinkByOTPHash :one
SELECT id, email, token_hash, device_code, otp_hash, authorized, user_id, otp_attempts, expires_at, used_at, created_at
FROM magic_links WHERE otp_hash = $1 AND used_at IS NULL AND expires_at > NOW()
ORDER BY created_at DESC LIMIT 1;

-- name: MarkMagicLinkUsed :execrows
UPDATE magic_links SET used_at = NOW() WHERE id = $1 AND used_at IS NULL;

-- name: GetMagicLinkByDeviceCode :one
SELECT id, email, token_hash, device_code, otp_hash, authorized, user_id, otp_attempts, expires_at, used_at, created_at
FROM magic_links WHERE device_code = $1 AND expires_at > NOW()
ORDER BY created_at DESC LIMIT 1;

-- name: AuthorizeMagicLinkByDeviceCode :execrows
UPDATE magic_links SET authorized = true, user_id = $2
WHERE device_code = $1 AND expires_at > NOW();

-- name: IncrementMagicLinkOTPAttempts :one
UPDATE magic_links SET otp_attempts = otp_attempts + 1
WHERE id = $1
RETURNING otp_attempts;

-- name: DeleteMagicLinksByDeviceCode :exec
DELETE FROM magic_links WHERE device_code = $1;

-- name: CountOTPAttemptsByDeviceCode :one
SELECT COALESCE(SUM(otp_attempts), 0)::int AS total FROM magic_links WHERE device_code = $1;

-- name: DeleteExpiredMagicLinks :exec
DELETE FROM magic_links WHERE expires_at < NOW();
