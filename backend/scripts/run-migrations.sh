#!/bin/sh
set -e

DB_HOST="${DB_HOST:-postgres}"
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:-quill}"
DB_PASSWORD="${DB_PASSWORD:-quill_dev_password}"
DB_NAME="${DB_NAME:-quill}"
MIGRATIONS_DIR="${MIGRATIONS_DIR:-/migrations}"

export PGPASSWORD="$DB_PASSWORD"
PSQL="psql -v ON_ERROR_STOP=1 -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME"

echo "Waiting for PostgreSQL at ${DB_HOST}:${DB_PORT}..."
until pg_isready -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER"; do
  sleep 1
done

echo "Ensuring required extensions are loaded"
$PSQL -c "CREATE EXTENSION IF NOT EXISTS vector;" || true
$PSQL -c "CREATE EXTENSION IF NOT EXISTS age;" || true

echo "Ensuring schema_migrations table exists"
$PSQL -c "CREATE TABLE IF NOT EXISTS schema_migrations (version VARCHAR(255) PRIMARY KEY, applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW());"

echo "Running migrations from ${MIGRATIONS_DIR}"
for f in $(ls -1 "$MIGRATIONS_DIR"/*.up.sql | sort); do
  version=$(basename "$f")
  applied=$($PSQL -t -c "SELECT COUNT(*) FROM schema_migrations WHERE version = '$version';")
  if [ "$(echo "$applied" | tr -d ' ')" -eq 0 ]; then
    echo "  -> applying $version"
    $PSQL -f "$f"
    $PSQL -c "INSERT INTO schema_migrations (version) VALUES ('$version');"
  else
    echo "  -> skipping $version (already applied)"
  fi
done

echo "Migrations finished successfully"
