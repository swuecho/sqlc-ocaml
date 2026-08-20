# Todo CLI example

This application runs PostgreSQL in Docker on host port **5438** and uses
sqlc-generated OCaml modules with Caqti and Lwt.

From the repository root:

```sh
make todo-generate
cd examples/todo
docker compose up -d --wait db
docker compose run --rm app add "buy milk"
docker compose run --rm app list
docker compose run --rm app done 1
docker compose run --rm app delete 1
```

Run the complete test with `make todo-e2e` from the repository root. Stop and
remove the database with `docker compose -f examples/todo/compose.yaml down -v`.

When running the compiled CLI directly on the host it defaults to
`postgresql://todo:todo@localhost:5438/todo`. Set `DATABASE_URL` to override it.

