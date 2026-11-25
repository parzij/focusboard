#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

# Always use a new project and test credentials, even when a local .env exists.
export COMPOSE_PROJECT_NAME="focusboard-smoke-$(date +%s)-$$"
export POSTGRES_DB=focusboard_test POSTGRES_USER=focusboard_test
export POSTGRES_PASSWORD=compose-smoke-only
export DATABASE_URL="postgres://$POSTGRES_USER:$POSTGRES_PASSWORD@db:5432/$POSTGRES_DB?sslmode=disable"
export API_PORT=0 API_CONTAINER_PORT=8080
export APP_ENV=test APP_ORIGIN=http://127.0.0.1:8080 COOKIE_SECURE=false
export REFRESH_COOKIE_NAME=focusboard_refresh
export JWT_SECRET
JWT_SECRET=$(openssl rand -base64 32)
compose() { docker compose --env-file .env.example "$@"; }
cleanup() {
  local status=$?
  if (( status != 0 )); then
    compose ps -a || true
    compose logs --no-color db migrate api || true
  fi
  compose down
  printf 'Test volume retained for %s\n' "$COMPOSE_PROJECT_NAME"
}
trap cleanup EXIT

probe() {
  local endpoint=$1 expected=$2 body=$3 output
  for ((attempt = 0; attempt < 40; attempt++)); do
    if output=$(curl -s --max-time 3 -w '%{http_code}' "$api_url/$endpoint") &&
      [[ $output == "$body"$'\n'"$expected" ]]; then
      printf '%s: HTTP %s\n' "$endpoint" "$expected"
      return
    fi
    sleep 0.5
  done
  printf 'Unexpected %s response: %s\n' "$endpoint" "$output" >&2
  return 1
}

sql() {
  compose exec -T db psql -XAt -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" "$@"
}

compose config --quiet
compose up --build -d
migration=$(compose ps -aq migrate)
api=$(compose ps -q api)
[[ $(docker inspect --format '{{.State.ExitCode}}' "$migration") == 0 ]]
migration_finished=$(docker inspect --format '{{.State.FinishedAt}}' "$migration")
api_started=$(docker inspect --format '{{.State.StartedAt}}' "$api")
[[ $migration_finished < $api_started ]]
[[ $(sql -c "SELECT count(*) FROM goose_db_version WHERE version_id > 0 AND is_applied") == 4 ]]
[[ -z $(docker port "$(compose ps -q db)") ]]
printf 'All four migrations finished before API start; PostgreSQL has no published port\n'

api_url="http://$(compose port api 8080)/api/v1"
health='{"status":"ok"}'
ready='{"status":"ready"}'
unavailable='{"error":{"code":"SERVICE_UNAVAILABLE","message":"Service temporarily unavailable."}}'
probe health 200 "$health"
probe ready 200 "$ready"
sql -c 'CREATE TABLE compose_smoke (id integer PRIMARY KEY); INSERT INTO compose_smoke VALUES (42);'

compose stop db
probe health 200 "$health"
probe ready 503 "$unavailable"

compose start db
probe ready 200 "$ready"
[[ $(compose ps -q api) == "$api" ]]
[[ $(sql -c 'SELECT id FROM compose_smoke') == 42 ]]
printf 'Readiness recovered in the same API container; data persisted\n'

repeat_output=$(compose run --rm --no-deps migrate 2>&1)
printf '%s\n' "$repeat_output"
[[ $repeat_output == *'"applied":0'* ]]
compose stop api
[[ $(docker inspect --format '{{.State.ExitCode}}' "$api") == 0 ]]
compose up -d
api_url="http://$(compose port api 8080)/api/v1"
probe ready 200 "$ready"
[[ $(sql -c 'SELECT id FROM compose_smoke') == 42 ]]
python3 scripts/test-auth.py "${api_url%/api/v1}" "$APP_ORIGIN"
python3 scripts/test-tasks.py "${api_url%/api/v1}" "$APP_ORIGIN"
python3 scripts/test-statistics.py "${api_url%/api/v1}" "$APP_ORIGIN"
printf 'Repeated migration and stack startup succeeded\n'
