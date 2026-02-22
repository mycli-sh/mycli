-- name: CreateMagicLink :one
INSERT INTO magic_links (email, token_hash, device_code, otp_hash, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, email, token_hash, device_code, otp_hash, expires_at, used_at, created_at;

-- name: GetMagicLinkByTokenHash :one
SELECT id, email, token_hash, device_code, otp_hash, expires_at, used_at, created_at
FROM magic_links WHERE token_hash = $1;

-- name: GetMagicLinkByOTPHash :one
SELECT id, email, token_hash, device_code, otp_hash, expires_at, used_at, created_at
FROM magic_links WHERE otp_hash = $1 AND used_at IS NULL AND expires_at > NOW()
ORDER BY created_at DESC LIMIT 1;

-- name: MarkMagicLinkUsed :execrows
UPDATE magic_links SET used_at = NOW() WHERE id = $1 AND used_at IS NULL;
