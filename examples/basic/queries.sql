-- name: FindUser :one
SELECT id, email, display_name, status, created_at
FROM users
WHERE id = $1;

-- name: ListUsers :many
SELECT id, email, display_name, status, created_at
FROM users
ORDER BY id;

-- name: DeleteUser :exec
DELETE FROM users
WHERE id = $1;
