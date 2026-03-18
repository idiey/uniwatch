#!/bin/bash
set -e
DB_URL="${DATABASE_URL:-postgres://postgres:dev@localhost:5432/uniwatch?sslmode=disable}"
echo "Running migrations: $DB_URL"
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
for f in "$DIR"/*.sql; do
    echo "  $(basename "$f")"
    psql "$DB_URL" -f "$f"
done
echo "Done."
