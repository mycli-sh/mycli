-- name: CreateDeviceSession :exec
INSERT INTO device_sessions (device_code, user_code, email, expires_at)
VALUES ($1, $2, $3, $4);

-- name: GetDeviceSessionByCode :one
SELECT * FROM device_sessions WHERE device_code = $1 AND expires_at > NOW();

-- name: GetDeviceSessionByUserCode :one
SELECT * FROM device_sessions WHERE user_code = $1 AND expires_at > NOW()
ORDER BY created_at DESC LIMIT 1;

-- name: AuthorizeDeviceSession :execrows
UPDATE device_sessions SET authorized = true, user_id = $2 WHERE device_code = $1 AND expires_at > NOW();

-- name: IncrementDeviceOTPAttempts :one
UPDATE device_sessions SET otp_attempts = otp_attempts + 1 WHERE device_code = $1 AND expires_at > NOW()
RETURNING otp_attempts;

-- name: ResetDeviceOTPAndExtend :exec
UPDATE device_sessions SET otp_attempts = 0, expires_at = $2 WHERE device_code = $1 AND expires_at > NOW();

-- name: DeleteDeviceSession :exec
DELETE FROM device_sessions WHERE device_code = $1;

-- name: DeleteExpiredDeviceSessions :exec
DELETE FROM device_sessions WHERE expires_at < NOW();
