-- name: CreateTodo :one
INSERT INTO todos (title, completed, todo_order)
VALUES ($1, $2, $3)
RETURNING todos.id, todos.title, todos.completed, todos.todo_order AS "order";

-- name: ListTodos :many
SELECT id, title, completed, todo_order AS "order"
FROM todos
ORDER BY id;

-- name: GetTodo :many
SELECT id, title, completed, todo_order AS "order"
FROM todos
WHERE id = $1;

-- name: PatchTodo :many
UPDATE todos
SET title = COALESCE(sqlc.narg(title), title),
    completed = COALESCE(sqlc.narg(completed), completed),
    todo_order = COALESCE(sqlc.narg(todo_order), todo_order)
WHERE id = sqlc.arg(id)
RETURNING todos.id, todos.title, todos.completed, todos.todo_order AS "order";

-- name: DeleteTodo :exec
DELETE FROM todos WHERE id = $1;

-- name: DeleteAllTodos :exec
DELETE FROM todos;
