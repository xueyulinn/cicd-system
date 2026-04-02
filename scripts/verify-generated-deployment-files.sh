#!/usr/bin/env bash
# Fail if generated deployment helper files drift from charts/e-team/values.yaml / Helm chart.
# Run from repo root: ./scripts/verify-generated-deployment-files.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

TMP_ENV="$(mktemp)"
TMP_K8S="$(mktemp)"
trap 'rm -f "$TMP_ENV" "$TMP_K8S"' EXIT

ruby "${ROOT}/scripts/gen-compose-env-from-values.rb" "$TMP_ENV"
if ! cmp -s "${ROOT}/compose.values.env" "$TMP_ENV"; then
  echo "compose.values.env is out of date. Run: ruby scripts/gen-compose-env-from-values.rb"
  exit 1
fi

helm template e-team "${ROOT}/charts/e-team" \
  --namespace e-team \
  -f "${ROOT}/charts/e-team/values.yaml" \
  --set fullnameOverride=e-team \
  >"$TMP_K8S"

if ! cmp -s "${ROOT}/k8s/helm-rendered/e-team.yaml" "$TMP_K8S"; then
  echo "k8s/helm-rendered/e-team.yaml is out of date. Run: ./scripts/render-k8s-manifests.sh"
  exit 1
fi

echo "Generated deployment files are up to date."
