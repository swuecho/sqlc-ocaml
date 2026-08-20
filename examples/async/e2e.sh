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
$compose exec -T db psql -U async -d async_demo -v ON_ERROR_STOP=1 \
  -c "TRUNCATE numbers; INSERT INTO numbers (value) SELECT generate_series(1, 1000)"
$compose build app
$compose run --rm app | tee /tmp/sqlc-ocaml-async.txt
grep -F "Async execute and fold verified 1000 rows (sum=500500)." \
  /tmp/sqlc-ocaml-async.txt

echo "Async end-to-end test passed."
