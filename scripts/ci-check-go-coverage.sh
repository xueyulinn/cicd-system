#!/usr/bin/env bash
set -euo pipefail

# Reads a Go coverprofile produced by: go test ./... -coverprofile=...
# Fails if total statement coverage is below MIN_COVERAGE (default 70).

MIN_COVERAGE="${MIN_COVERAGE:-70}"
PROFILE="${1:-coverage.out}"

if [[ ! -f "$PROFILE" ]]; then
  echo "coverage profile not found: $PROFILE" >&2
  exit 1
fi

total="$(go tool cover -func="$PROFILE" | awk '/^total:/{gsub(/%/,"",$3); print $3; exit}')"
if [[ -z "${total}" ]]; then
  echo "could not parse total coverage from go tool cover output" >&2
  exit 1
fi

echo "Total statement coverage: ${total}% (minimum ${MIN_COVERAGE}%)"

awk -v t="${total}" -v m="${MIN_COVERAGE}" 'BEGIN {
  if (t + 0 >= m + 0) exit 0
  exit 1
}' || {
  echo "error: coverage ${total}% is below required ${MIN_COVERAGE}%" >&2
  exit 1
}
