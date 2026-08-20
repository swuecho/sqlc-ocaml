#!/bin/sh
set -eu

api="http://localhost:8080/todos"
compose="docker compose -f compose.yaml"

$compose up -d --build --wait
$compose exec -T db psql -U todo -d todobackend -v ON_ERROR_STOP=1 \
  -c "TRUNCATE todos RESTART IDENTITY"

attempt=0
until curl -fsS "$api" >/dev/null; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 30 ]; then
    echo "Todo-Backend API did not become ready" >&2
    exit 1
  fi
  sleep 1
done

curl -fsS -X DELETE "$api" >/dev/null
created=$(curl -fsS -X POST -H 'Content-Type: application/json' \
  -d '{"title":"spec todo","order":10}' "$api")
echo "$created" | grep -F '"title":"spec todo"'
echo "$created" | grep -F '"completed":false'
echo "$created" | grep -F '"order":10'
url=$(echo "$created" | sed -n 's/.*"url":"\([^"]*\)".*/\1/p')
curl -fsS "$url" | grep -F '"title":"spec todo"'
curl -fsS -X PATCH -H 'Content-Type: application/json' \
  -d '{"title":"changed","completed":true,"order":95}' "$url" \
  | grep -F '"completed":true'
partial=$(curl -fsS -X PATCH -H 'Content-Type: application/json' \
  -d '{"title":"partial"}' "$url")
echo "$partial" | grep -F '"title":"partial"'
echo "$partial" | grep -F '"completed":true'
echo "$partial" | grep -F '"order":95'
empty=$(curl -fsS -X PATCH -H 'Content-Type: application/json' -d '{}' "$url")
echo "$empty" | grep -F '"title":"partial"'
test "$(curl -sS -o /dev/null -w '%{http_code}' -X PATCH \
  -H 'Content-Type: application/json' -d '{}' "$api/999999")" = "404"
curl -fsS "$api" | grep -F '"order":95'
curl -fsS -X DELETE "$url" >/dev/null
test "$(curl -fsS "$api")" = "[]"
curl -fsSI -X OPTIONS "$api" | grep -i '^access-control-allow-origin: \*'

echo "Todo-Backend smoke test passed."
