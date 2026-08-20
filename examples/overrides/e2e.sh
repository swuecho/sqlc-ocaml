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
$compose exec -T db psql -U override -d override_demo -v ON_ERROR_STOP=1 \
  -c "TRUNCATE users RESTART IDENTITY"
$compose build app
$compose run --rm app | tee /tmp/sqlc-ocaml-overrides.txt
grep -F "1 Ada ada@example.com" /tmp/sqlc-ocaml-overrides.txt
grep -F "2 Grace <none>" /tmp/sqlc-ocaml-overrides.txt

echo "Column and nullable override end-to-end test passed."
