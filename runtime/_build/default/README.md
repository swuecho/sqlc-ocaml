# sqlc-ocaml runtime packages

These optional packages give applications a consistent interface for creating
and reusing Caqti connection pools. Generated query modules continue to accept
a connection explicitly, so they also work with transactions and direct
connections.

## Lwt

Add `sqlc-ocaml-runtime-lwt` to the executable's Dune libraries:

```ocaml
module Runtime = Sqlc_ocaml_runtime_lwt

let database = Runtime.Database.connect_uri_exn database_url

let find_user params =
  Runtime.Database.run database
    (fun db -> Queries.FindUser.execute db params)
```

## Async

Add `sqlc-ocaml-runtime-async` to the executable's Dune libraries:

```ocaml
module Runtime = Sqlc_ocaml_runtime_async

let database = Runtime.Database.connect_uri_exn database_url

let find_user params =
  Runtime.Database.use database
    (fun db -> Queries.FindUser.execute db params)
```

Both packages expose a resource-first `Database` interface:

- `Database.connect` for a `Uri.t`;
- `Database.connect_uri` for a database URL string;
- `Database.connect_uri_exn` for applications which treat initialization failure
  as fatal;
- `Database.use database operation` for non-blocking use of a pooled
  connection.

For application-level helpers, `Runtime.with_database database operation` is a
short alias for `Runtime.Database.use database operation`.

The Lwt package also provides `Database.run database operation` for synchronous
command-line programs. Servers should use the non-blocking `Database.use`
function. The lower-level `Pool` module remains available when direct access to
Caqti's pool API is needed.
