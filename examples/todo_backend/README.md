# Todo-Backend API example

This example implements the [Todo-Backend specification](https://www.todobackend.com/)
with OCaml 5, Dream, Lwt, PostgreSQL, sqlc-ocaml, Caqti 2.x, Yojson, and Dune.

The API root is `http://localhost:8080/todos`; PostgreSQL is exposed on host
port 5439.

From the repository root:

```sh
make todo-backend-generate
make todo-backend-e2e
```

Manual requests:

```sh
curl -X POST -H 'Content-Type: application/json' \
  -d '{"title":"buy milk","order":1}' http://localhost:8080/todos
curl http://localhost:8080/todos
```

To run the official browser suite, open:

```text
https://www.todobackend.com/specs/index.html?http://localhost:8080/todos
```

The implementation is verified against all 16 tests in that suite (16 passes,
0 failures).

Stop and remove the containers with:

```sh
docker compose -f examples/todo_backend/compose.yaml down -v
```
