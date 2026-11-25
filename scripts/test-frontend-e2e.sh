#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

export COMPOSE_PROJECT_NAME="focusboard-e2e-$(date +%s)-$$"
export POSTGRES_DB=focusboard_test POSTGRES_USER=focusboard_test
export POSTGRES_PASSWORD=e2e-postgres-fixture-only
export DATABASE_URL="postgres://$POSTGRES_USER:$POSTGRES_PASSWORD@db:5432/$POSTGRES_DB?sslmode=disable"
export API_PORT=8080 API_CONTAINER_PORT=8080
export APP_ENV=test APP_ORIGIN=http://127.0.0.1:5173 COOKIE_SECURE=false
export JWT_SECRET
JWT_SECRET=$(openssl rand -base64 32)

compose() { docker compose --env-file .env.example "$@"; }
cleanup() {
  compose down
  printf 'Test volume retained for %s\n' "$COMPOSE_PROJECT_NAME"
}
trap cleanup EXIT

compose config --quiet
compose up --build -d
curl -fsS --retry 10 --retry-connrefused --retry-all-errors http://127.0.0.1:8080/api/v1/ready
(cd frontend && ./node_modules/.bin/playwright test)
