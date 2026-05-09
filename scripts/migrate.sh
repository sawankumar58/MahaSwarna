#!/usr/bin/env sh
# MahaSwarna — Database migrations (one-shot, run via docker compose run migrate)
# Runs migrations in schema dependency order: core → pricing → intelligence
# Uses golang-migrate/migrate CLI (bundled in migrate/migrate:v4.18.1 image)
set -eu

DB="${DATABASE_URL:?DATABASE_URL is required}"

echo "[migrate] Starting migrations..."

run() {
  SCHEMA=$1
  echo "[migrate] Running schema: $SCHEMA"
  migrate -path "/migrations/$SCHEMA" -database "$DB" up
  echo "[migrate] $SCHEMA OK"
}

run core
run pricing
run intelligence

echo "[migrate] All migrations complete."
