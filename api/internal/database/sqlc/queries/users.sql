-- name: CreateUser :one
INSERT INTO users (email) VALUES (lower(trim($1)))
RETURNING id, email, username, created_at;

-- name: GetUserByID :one
SELECT id, email, username, created_at FROM users WHERE id = $1;

-- name: GetUserByEmail :one
SELECT id, email, username, created_at FROM users WHERE lower(email) = $1;

-- name: SetUsername :execrows
UPDATE users SET username = $2 WHERE id = $1 AND username IS NULL;

-- name: GetUserByUsername :one
SELECT id, email, username, created_at FROM users WHERE lower(username) = lower($1);

-- name: IsUsernameTaken :one
SELECT EXISTS(SELECT 1 FROM users WHERE lower(username) = lower($1));
