#!/bin/sh
set -eu

compose="docker compose -f compose.yaml"

cleanup() {
  $compose down --volumes --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

$compose up -d --wait db
$compose exec -T db psql -U todo -d todo -v ON_ERROR_STOP=1 \
  -c "ALTER TABLE todos ADD COLUMN IF NOT EXISTS tags TEXT[] NOT NULL DEFAULT '{}'; TRUNCATE todos RESTART IDENTITY"
$compose build app
$compose run --rm app add "write generated OCaml"
$compose run --rm app add "test against PostgreSQL"
$compose run --rm app list | tee /tmp/sqlc-ocaml-todo-list-before.txt
grep -F "1 [ ] write generated OCaml" /tmp/sqlc-ocaml-todo-list-before.txt
grep -F "2 [ ] test against PostgreSQL" /tmp/sqlc-ocaml-todo-list-before.txt
grep -F 'tags=cli|comma,value|quote"slash\' /tmp/sqlc-ocaml-todo-list-before.txt
$compose run --rm app ids 2 | tee /tmp/sqlc-ocaml-todo-list-selected.txt
grep -F "2 [ ] test against PostgreSQL" /tmp/sqlc-ocaml-todo-list-selected.txt
if grep -F "write generated OCaml" /tmp/sqlc-ocaml-todo-list-selected.txt; then
  echo "array filter returned an unselected todo" >&2
  exit 1
fi
test -z "$($compose run --rm app ids)"
$compose run --rm app done 1
$compose run --rm app list | tee /tmp/sqlc-ocaml-todo-list-after.txt
grep -F "1 [x] write generated OCaml" /tmp/sqlc-ocaml-todo-list-after.txt
$compose run --rm app delete 2
$compose run --rm app list | tee /tmp/sqlc-ocaml-todo-list-final.txt
if grep -F "test against PostgreSQL" /tmp/sqlc-ocaml-todo-list-final.txt; then
  echo "deleted todo is still present" >&2
  exit 1
fi

echo "Todo end-to-end test passed."
