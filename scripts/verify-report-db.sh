#!/usr/bin/env bash
# Verify report store DB: start Postgres, apply schema, check tables exist.
# Run from repo root. Exit 0 if setup is correct.

set -e
cd "$(dirname "$0")/.."

echo "=== Starting Postgres (if not running) ==="
docker compose up -d

echo "=== Waiting for Postgres to be ready ==="
for i in {1..30}; do
  if docker compose exec -T postgres pg_isready -U cicd -d reportstore >/dev/null 2>&1; then
    break
  fi
  if [[ $i -eq 30 ]]; then
    echo "Postgres did not become ready in time."
    exit 1
  fi
  sleep 1
done

echo "=== Applying schema ==="
docker compose exec -T postgres psql -U cicd -d reportstore -f - < migrations/001_report_store_schema.sql

echo "=== Verifying tables exist ==="
TABLES=$(docker compose exec -T postgres psql -U cicd -d reportstore -t -A -c \
  "SELECT string_agg(table_name, ' ') FROM information_schema.tables WHERE table_schema = 'public' AND table_name IN ('pipeline_runs', 'stage_runs', 'job_runs');")
for t in pipeline_runs stage_runs job_runs; do
  if [[ "$TABLES" != *"$t"* ]]; then
    echo "Missing table: $t (found: $TABLES)"
    exit 1
  fi
done

echo "=== Verifying tables are queryable ==="
docker compose exec -T postgres psql -U cicd -d reportstore -c "SELECT 0 FROM pipeline_runs LIMIT 1;" >/dev/null
docker compose exec -T postgres psql -U cicd -d reportstore -c "SELECT 0 FROM stage_runs LIMIT 1;" >/dev/null
docker compose exec -T postgres psql -U cicd -d reportstore -c "SELECT 0 FROM job_runs LIMIT 1;" >/dev/null

echo "=== Report DB setup verified successfully ==="
