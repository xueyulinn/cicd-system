#!/usr/bin/env bash
# Verify report store DB: start MySQL, apply schema, check tables exist.
# Run from repo root. Exit 0 if setup is correct.

set -euo pipefail
cd "$(dirname "$0")/.."

echo "=== Starting MySQL (if not running) ==="
docker compose up -d

echo "=== Waiting for MySQL to be ready ==="
for i in {1..60}; do
  if docker compose exec -T mysql mysqladmin ping -h 127.0.0.1 -ucicd -pcicd --silent >/dev/null 2>&1; then
    break
  fi
  if [[ $i -eq 60 ]]; then
    echo "MySQL did not become ready in time."
    exit 1
  fi
  sleep 1
done

echo "=== Applying migrations ==="
docker compose run --rm db-migrate >/dev/null

mysql_exec() {
  docker compose exec -T mysql mysql -ucicd -pcicd reportstore "$@"
}

echo "=== Verifying core tables exist ==="
TABLE_COUNT="$(mysql_exec -N -B -e "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name IN ('pipeline_runs', 'stage_runs', 'job_runs');")"
if [[ "${TABLE_COUNT}" != "3" ]]; then
  echo "Expected 3 core tables, got ${TABLE_COUNT}"
  exit 1
fi

echo "=== Verifying job_runs.failures (migration 002) ==="
FAILURES_COL_COUNT="$(mysql_exec -N -B -e "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'job_runs' AND column_name = 'failures';")"
if [[ "${FAILURES_COL_COUNT}" != "1" ]]; then
  echo "Expected job_runs.failures column, got ${FAILURES_COL_COUNT}"
  exit 1
fi

echo "=== Verifying tables are queryable ==="
mysql_exec -e "SELECT 0 FROM pipeline_runs LIMIT 1;" >/dev/null
mysql_exec -e "SELECT 0 FROM stage_runs LIMIT 1;" >/dev/null
mysql_exec -e "SELECT failures FROM job_runs LIMIT 1;" >/dev/null

echo "=== Report DB setup verified successfully ==="
