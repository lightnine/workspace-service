#!/usr/bin/env bash
# Apply all ws_* DDL scripts to MySQL.
# Usage: MYSQL_PWD=xxx ./scripts/apply-schema.sh [database]
set -euo pipefail

DB="${1:-workspace}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

for f in "$ROOT"/sql/0*.sql; do
  [[ -f "$f" ]] || continue
  echo "==> $(basename "$f")"
  mysql -u root "$DB" < "$f"
done

echo "Schema applied to database: $DB"
