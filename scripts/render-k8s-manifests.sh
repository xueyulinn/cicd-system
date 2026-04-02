#!/usr/bin/env bash
# Render static Kubernetes manifests from the Helm chart (canonical source of truth).
# Usage: from repo root, ./scripts/render-k8s-manifests.sh
# Requires: helm
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

OUT_DIR="${ROOT}/k8s/helm-rendered"
OUT_FILE="${OUT_DIR}/e-team.yaml"
VALUES="${ROOT}/charts/e-team/values.yaml"

mkdir -p "$OUT_DIR"

# fullnameOverride avoids doubled release/chart name segments (e.g. e-team-e-team-*).
helm template e-team "${ROOT}/charts/e-team" \
  --namespace e-team \
  -f "$VALUES" \
  --set fullnameOverride=e-team \
  >"$OUT_FILE"

echo "Wrote ${OUT_FILE}"
