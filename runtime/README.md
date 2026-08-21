# sqlc-ocaml runtime packages

These optional packages give applications a consistent interface for creating
and reusing Caqti connection pools. Generated query modules continue to accept
a connection explicitly, so they also work with transactions and direct
connections.

## Lwt

Add `sqlc-ocaml-runtime-lwt` to the executable's Dune libraries:

```ocaml
module Runtime = Sqlc_ocaml_runtime_lwt

let pool = Runtime.Pool.connect_uri_exn database_url

let find_user params =
  Runtime.Pool.run
    (fun db -> Queries.FindUser.execute db params)
    pool
```

## Async

Add `sqlc-ocaml-runtime-async` to the executable's Dune libraries:

```ocaml
module Runtime = Sqlc_ocaml_runtime_async

let pool = Runtime.Pool.connect_uri_exn database_url

let find_user params =
  Runtime.Pool.use
    (fun db -> Queries.FindUser.execute db params)
    pool
```

Both packages expose Caqti's complete `Pool` interface and add:

- `Pool.connect` for a `Uri.t`;
- `Pool.connect_uri` for a database URL string;
- `Pool.connect_uri_exn` for applications which treat initialization failure
  as fatal.

The Lwt package also provides `Pool.run` for synchronous command-line programs.
Servers should use the non-blocking `Pool.use` function.
