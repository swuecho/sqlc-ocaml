# Column override example

This example maps only the nullable PostgreSQL column `users.email` to the
application type `Email.t option`. The other `text` column, `display_name`,
remains a normal OCaml `string`.

Run the complete generation, compilation, and PostgreSQL round-trip test from
the repository root:

```sh
make overrides-e2e
```

PostgreSQL is exposed on host port 5440.
