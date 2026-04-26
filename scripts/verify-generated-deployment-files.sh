#!/usr/bin/env bash
# Fail if compose.values.env drifts from charts/cicd/values.yaml.
# Run from repo root: ./scripts/verify-generated-deployment-files.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

TMP_ENV="$(mktemp)"
trap 'rm -f "$TMP_ENV"' EXIT

ruby "${ROOT}/scripts/gen-compose-env-from-values.rb" "$TMP_ENV"
if ! cmp -s "${ROOT}/compose.values.env" "$TMP_ENV"; then
  echo "compose.values.env is out of date. Run: ruby scripts/gen-compose-env-from-values.rb"
  exit 1
fi

echo "compose.values.env is up to date with charts/cicd/values.yaml."
