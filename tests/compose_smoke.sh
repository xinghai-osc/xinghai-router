#!/usr/bin/env bash
set -Eeuo pipefail

# Run against an isolated Compose project and temporary bind-mounted data paths.
project="${COMPOSE_PROJECT_NAME:-xinghai-router-e2e}"
router_url="http://127.0.0.1:${ROUTER_PORT:-18080}"
web_url="http://127.0.0.1:${WEB_PORT:-13000}"

: "${POSTGRES_PASSWORD:=router-e2e-postgres}"
: "${REDIS_PASSWORD:=router-e2e-redis}"
: "${ENCRYPTION_KEY:=router-e2e-encryption-key-change-me-2026}"
: "${BOOTSTRAP_ADMIN_EMAIL:=e2e-admin@example.test}"
: "${BOOTSTRAP_ADMIN_PASSWORD:=router-e2e-password}"
export POSTGRES_PASSWORD REDIS_PASSWORD ENCRYPTION_KEY BOOTSTRAP_ADMIN_EMAIL BOOTSTRAP_ADMIN_PASSWORD
export POSTGRES_DATA_PATH="${POSTGRES_DATA_PATH:-${TMPDIR:-/tmp}/xinghai-router-postgres-${project}}"
export REDIS_DATA_PATH="${REDIS_DATA_PATH:-${TMPDIR:-/tmp}/xinghai-router-redis-${project}}"
: "${ROUTER_PORT:=18080}"
: "${WEB_PORT:=13000}"
export ROUTER_PORT WEB_PORT

cleanup() {
  docker compose -p "$project" down --remove-orphans || true
}
trap cleanup EXIT

docker compose -p "$project" up -d --build
ready=false
for _ in $(seq 1 60); do
  router_status=$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}starting{{end}}' "${project}-router-1" 2>/dev/null || true)
  web_status=$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}starting{{end}}' "${project}-web-1" 2>/dev/null || true)
  if [[ "$router_status" == healthy && "$web_status" == healthy ]]; then ready=true; break; fi
  sleep 2
done
[[ "$ready" == true ]]
[[ "$(curl -fsS "$router_url/healthz" | python3 -c 'import json,sys; print(json.load(sys.stdin)["status"])')" == ok ]]
[[ "$(curl -fsS "$router_url/readyz" | python3 -c 'import json,sys; print(json.load(sys.stdin)["status"])')" == ready ]]
curl -fsS "$web_url/" >/dev/null

login=$(curl -fsS -X POST "$router_url/auth/login" -H 'Content-Type: application/json' -d "{\"email\":\"$BOOTSTRAP_ADMIN_EMAIL\",\"password\":\"$BOOTSTRAP_ADMIN_PASSWORD\"}")
session=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["token"])' <<<"$login")
me=$(curl -fsS "$router_url/account/me" -H "Authorization: Bearer $session")
key_response=$(curl -fsS -X POST "$router_url/account/keys" -H "Authorization: Bearer $session" -H 'Content-Type: application/json' -d '{"name":"compose-e2e"}')
api_key=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["key"])' <<<"$key_response")
[[ "$api_key" == sk-xh-* ]]
models=$(curl -fsS "$router_url/v1/models" -H "Authorization: Bearer $api_key")
python3 -c 'import json,sys; body=json.load(sys.stdin); assert body["object"] == "list" and isinstance(body["data"], list)' <<<"$models"
printf '%s\n' "Compose smoke passed: $me"
