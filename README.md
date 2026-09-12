# sqlc-ocaml

`sqlc-ocaml` is an experimental [sqlc](https://sqlc.dev/) process plugin that generates typed OCaml modules for PostgreSQL queries using Caqti and Lwt.

## MVP scope

- PostgreSQL
- `:one`, `:many`, `:exec`, and `:execrows`
- generated parameter and result records
- Lwt or Async runtime support
- constant-memory `fold` functions for `:many` queries
- reusable table models and nested `sqlc.embed(...)` results
- nullable scalar values
- one-dimensional scalar array parameters and results
- `.ml` and `.mli` output
- UUID, JSON/JSONB, dates, timestamps, PostgreSQL enums, and custom type overrides

Multidimensional and nullable-element arrays, batch commands, SQLite/MySQL,
and Eio are deliberately outside this first release. Array
parameters may need an explicit SQL cast, for example `id = ANY($1::bigint[])`.
Supported array elements are text/character types, booleans, integer and float
types, UUIDs, and PostgreSQL enums. Empty arrays and quoted text containing
commas, quotes, or backslashes are encoded and decoded.
Repeated and out-of-order PostgreSQL placeholders are supported through an
explicit typed parameter-occurrence plan;
MVP queries must still reference every declared parameter at least once.

## Type mapping

| PostgreSQL | OCaml |
| --- | --- |
| `bool` | `bool` |
| `smallint`, `int2` | `int` |
| `integer`, `int4`, `serial` | `int32` |
| `bigint`, `int8`, `bigserial` | `int64` |
| `real`, `float4`, `double precision`, `float8` | `float` |
| `text`, `varchar`, `character`, `bpchar` | `string` |
| `bytea` | `string` (octets) |
| `uuid` | `Uuidm.t` |
| `date` | `Ptime.date` |
| `timestamp`, `timestamptz` | `Ptime.t` |
| `json`, `jsonb` | `Yojson.Safe.t` |
| `numeric`, `decimal` | `string` (use an override for a decimal type) |
| enum | generated variant type |
| one-dimensional array of a supported element | `'a list` |

Nullable columns wrap the OCaml type in `option` and the Caqti codec in
`Caqti_type.option`. Import `ptime`, `uuidm`, and `yojson` when the mapped
columns use them.

## Generated names

- Table models keep the table name (`registry_entries`).
- A query that projects exactly one table's columns is shared as
  `<singular table>_row` (`registry_entries` → `registry_entry_row`,
  `todos` → `todo_row`). Singularization handles `-ies` (`entries` → `entry`),
  `-sses`/`-shes`/`-ches`/`-xes`/`-zes`, and a few irregulars (`people`,
  `children`).
- Any other result — a partial projection or an aggregate — is named
  `<first query>_row`. A shape is only shared when the field names, types, and
  **source tables** match, so identical projections from different tables stay
  distinct instead of aliasing one another.

The generated `query` GADT declares an `exec_rows` cardinality whose error type
is `[ Caqti_error.t | \`Unsupported ]`, because Caqti's
`exec_with_affected_count` can report an unsupported backend. A consumer's
shared `QUERIES` module type must declare that same error type for `Exec_rows`.

## Install and configure

Prebuilt `sqlc-gen-ocaml` archives for Linux, macOS, and Windows (amd64 and
arm64) are attached to each [release](https://github.com/swuecho/sqlc-ocaml/releases).
Extract the archive for your platform and point the plugin `process.cmd` at the
executable.

With a Go toolchain you can install or build the plugin from source:

```sh
go install github.com/swuecho/sqlc-ocaml/cmd/sqlc-gen-ocaml@latest
# or, from a checkout:
go build -o sqlc-gen-ocaml ./cmd/sqlc-gen-ocaml
```

```yaml
version: "2"
plugins:
  - name: ocaml
    process:
      cmd: ./sqlc-gen-ocaml
      format: json
sql:
  - engine: postgresql
    schema: schema.sql
    queries: queries.sql
    codegen:
      - plugin: ocaml
        out: lib/generated
        options:
          filename: queries
          runtime: lwt # or async
          overrides:
            - db_type: numeric
              type: Decimal.t
              codec: Db_types.decimal
            - column: users.email
              nullable: true
              type: Email.t
              codec: Email.codec
```

Each query is emitted as a module with `params`, optional `row`, and `execute`. Query parameters are always records, except a parameterless query uses `unit`.
When two or more queries return the same record shape, the generator emits one
top-level row type and aliases each module's `row` to it. This makes the query
results interchangeable while preserving the module-local `row` API.
When generated queries use PostgreSQL arrays, the plugin also emits
`sqlc_runtime.ml` and `sqlc_runtime.mli`. Include both files in explicit build
manifests and container `COPY` instructions alongside the generated query files.
The `filename` also determines the enclosing OCaml compilation-unit name
(`queries.ml` becomes `Queries`).
The `runtime` option selects `Caqti_lwt.CONNECTION`/`Lwt.t` (the default) or
`Caqti_async.CONNECTION`/`Async_kernel.Deferred.t`. A `:many` query also emits
`fold`, which processes rows without first materializing the complete result
list. Callers which need all rows can continue to use `execute`.

Catalog objects are resolved by their catalog/schema/name identity. If two
schemas contain a table or enum with the same name, generated OCaml type names
are schema-prefixed (for example, `public_users` and `audit_users`). Unique
names retain the previous unqualified output for backwards compatibility.

Overrides select either `db_type` or `column`. A column may be written as
`column`, `table.column`, `schema.table.column`, or
`catalog.schema.table.column`; `*.column` is also accepted. The optional
`nullable` selector restricts a rule to nullable or non-nullable values.
Column rules take precedence over database-type rules. Nullable values still
receive the generated OCaml `option` wrapper, so override `type` and `codec`
describe the non-null element.
Unknown plugin option names are rejected so configuration typos cannot silently
select defaults.

```ocaml
Queries.FindUser.execute db { id = 42L }
```

Lwt output needs `caqti-lwt` and `lwt`; Async output needs `caqti-async` and
`async_kernel`. Depending on the SQL types used, generated applications also
need `ptime`, `uuidm`, and `yojson`.

Applications can optionally use `sqlc-ocaml-runtime-lwt` or
`sqlc-ocaml-runtime-async` for a consistent
`Database.connect`/`Database.use` API. The
pool remains application-owned and generated query functions continue to
accept an explicit database connection, preserving transaction support. See
[`runtime`](runtime) for package setup and examples.

The MVP uses sqlc's supported JSON process-plugin wire format. This keeps the
binary dependency-free and makes the protocol easy to inspect during early
development.

## Generator architecture

The generator normalizes sqlc's protocol objects into an internal typed
representation before rendering OCaml:

```text
GenerateRequest
    -> Program
       -> Enum
       -> Model
       -> Query
          -> Cardinality
          -> Record
             -> Field
                -> OCamlType
    -> .ml / .mli renderer
```

The normalization pass owns validation, naming, SQL placeholder rewriting,
and PostgreSQL-to-OCaml type resolution. The renderer consumes only this IR;
it does not inspect sqlc protocol objects.

See [`examples/basic`](examples/basic) for a complete schema, query set, and
sqlc configuration.

[`examples/todo`](examples/todo) is a complete command-line application backed
by a Dockerized PostgreSQL database exposed on port 5438. Run its full
generation/build/database test with `make todo-e2e`.

[`examples/todo_backend`](examples/todo_backend) is a Dream HTTP service that
implements the Todo-Backend specification using the complete OCaml/sqlc/Caqti
stack. Run it with `make todo-backend-e2e`.

[`examples/overrides`](examples/overrides) demonstrates a nullable,
column-specific custom type and codec with a compiled PostgreSQL round trip.
Run it with `make overrides-e2e`; its database is exposed on port 5440.

[`examples/async`](examples/async) runs generated code with Caqti Async against
1,000 PostgreSQL rows and verifies both list-returning `execute` and
constant-memory `fold`. Run it with `make async-e2e`; its database is exposed
on port 5441.

## Development checks

Use the narrowest check that covers your change:

```sh
make quickcheck       # Go tests and vet; no Docker or database required
make fuzz             # fuzz the PostgreSQL placeholder lexer
make generated-check # regenerate examples and check for stale output; needs Docker
make e2e              # complete OCaml/PostgreSQL integration suite; needs Docker
```

`make check` remains an alias for `make quickcheck`. Run `make e2e` before
submitting changes that affect generated OCaml or database behavior.

Run every database-backed example with `make e2e`. The example
applications share the `sqlc-ocaml-example-base:local` Docker image, so the
OCaml compiler and common dependencies are installed only once. Set
`EXAMPLE_BASE_IMAGE` to use a differently tagged prebuilt image.
