-- name: CreateUser :one
INSERT INTO users (display_name, email)
VALUES ($1, $2)
RETURNING id, display_name, email;

-- name: ListUsers :many
SELECT id, display_name, email
FROM users
ORDER BY id;
