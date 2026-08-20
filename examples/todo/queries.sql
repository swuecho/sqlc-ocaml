-- name: CreateTodo :one
INSERT INTO todos (title, tags)
VALUES ($1, $2)
RETURNING id, title, tags, completed, created_at;

-- name: ListTodos :many
SELECT id, title, tags, completed, created_at
FROM todos
ORDER BY id;

-- name: ListTodosByIds :many
SELECT id, title, tags, completed, created_at
FROM todos
WHERE id = ANY(sqlc.arg(ids)::bigint[])
ORDER BY id;

-- name: CompleteTodo :one
UPDATE todos
SET completed = true
WHERE id = $1
RETURNING id, title, tags, completed, created_at;

-- name: DeleteTodo :execrows
DELETE FROM todos
WHERE id = $1;
