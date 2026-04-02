#!/usr/bin/env bash
# Verify report store DB inside Kubernetes (Postgres + migrations in namespace e-team).
# Expects manifests from the Helm chart (see k8s/helm-rendered/e-team.yaml) or an equivalent Helm install.
# Prerequisites: kubectl configured; stack applied; migrate image pullable.
# Run from repo root. Exit 0 if checks pass.

set -euo pipefail
cd "$(dirname "$0")/.."

NS="${K8S_NAMESPACE:-e-team}"
MIGRATE_JOB="${MIGRATE_JOB_NAME:-e-team-report-db-migrate}"

echo "=== Namespace: $NS ==="

echo "=== Waiting for Postgres pod ==="
kubectl -n "$NS" wait pod -l app.kubernetes.io/component=postgres,app.kubernetes.io/name=e-team --for=condition=Ready --timeout=180s

echo "=== Waiting for migration Job ==="
if ! kubectl -n "$NS" get "job/${MIGRATE_JOB}" >/dev/null 2>&1; then
  echo "Job ${MIGRATE_JOB} not found. Apply k8s/helm-rendered/e-team.yaml (or helm install) first."
  exit 1
fi
kubectl -n "$NS" wait "job/${MIGRATE_JOB}" --for=condition=complete --timeout=300s

PGPOD=$(kubectl -n "$NS" get pod -l app.kubernetes.io/component=postgres,app.kubernetes.io/name=e-team -o jsonpath='{.items[0].metadata.name}')
if [[ -z "$PGPOD" ]]; then
  echo "Could not find postgres pod."
  exit 1
fi

psql_exec() {
  kubectl -n "$NS" exec "$PGPOD" -- env PGPASSWORD=cicd psql -h 127.0.0.1 -U cicd -d reportstore -v ON_ERROR_STOP=1 "$@"
}

echo "=== Verifying core tables ==="
cnt=$(psql_exec -t -A -c \
  "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name IN ('pipeline_runs', 'stage_runs', 'job_runs');")
if [[ "${cnt// /}" != "3" ]]; then
  echo "Expected 3 tables (pipeline_runs, stage_runs, job_runs); got count: ${cnt:-empty}"
  exit 1
fi

echo "=== Verifying job_runs.failures (migration 002) ==="
fc=$(psql_exec -t -A -c \
  "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'job_runs' AND column_name = 'failures';")
if [[ "${fc// /}" != "1" ]]; then
  echo "Expected job_runs.failures column; got count: ${fc:-empty}"
  exit 1
fi

echo "=== Verifying tables are queryable ==="
psql_exec -c "SELECT 0 FROM pipeline_runs LIMIT 1;" >/dev/null
psql_exec -c "SELECT 0 FROM stage_runs LIMIT 1;" >/dev/null
psql_exec -c "SELECT failures FROM job_runs LIMIT 1;" >/dev/null

echo "=== Report DB (K8s) verified successfully ==="
