#!/bin/bash

# Start all services script
echo "Starting e-team services..."

ENV_FILE="${1:-.env.local}"

trim() {
  local s="$1"
  # shellcheck disable=SC2001
  s="$(echo "$s" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"
  printf '%s' "$s"
}

load_env_file() {
  local file="$1"
  local loaded=0
  while IFS= read -r line || [[ -n "$line" ]]; do
    line="${line%$'\r'}"
    [[ -z "$line" ]] && continue
    [[ "${line:0:1}" == "#" ]] && continue
    [[ "$line" != *"="* ]] && continue

    local key="${line%%=*}"
    local value="${line#*=}"
    key="$(trim "$key")"
    [[ -z "$key" ]] && continue

    export "${key}=${value}"
    loaded=$((loaded + 1))
  done < "$file"
  echo "Loaded $loaded env vars from $file"
}

if [[ -f "$ENV_FILE" ]]; then
  load_env_file "$ENV_FILE"
else
  echo "Env file not found: $ENV_FILE (continuing with current shell env/defaults)"
fi

# Reporting and orchestrator services need MySQL. Use default if not set (run ./scripts/verify-report-db.sh first).
if [[ -z "${DATABASE_URL:-}" ]] && [[ -z "${REPORT_DB_URL:-}" ]]; then
  export DATABASE_URL="cicd:cicd@tcp(localhost:3306)/reportstore?parseTime=true&charset=utf8mb4&loc=UTC"
  echo "Note: DATABASE_URL not set, using default (start MySQL with: ./scripts/verify-report-db.sh)"
fi

# Start validation service
echo "Starting validation service on port 8001..."
go run ./cmd/validation-service &
VALIDATION_PID=$!

# Start API gateway
echo "Starting API gateway on port 8000..."
go run ./cmd/api-gateway &
GATEWAY_PID=$!

# Start orchestrator service
echo "Starting orchestrator service on port 8002..."
go run ./cmd/orchestrator-service &
ORCHESTRATOR_PID=$!

# Start reporting service
echo "Starting reporting service on port 8004..."
go run ./cmd/reporting-service &
REPORTING_PID=$!

# Start worker service
echo "Starting worker service on port 8003..."
go run ./cmd/worker-service &
WORKER_PID=$!

echo "Services started:"
echo "  - Validation Service: http://localhost:8001 (PID: $VALIDATION_PID)"
echo "  - API Gateway: http://localhost:8000 (PID: $GATEWAY_PID)"
echo "  - Orchestrator Service: http://localhost:8002 (PID: $ORCHESTRATOR_PID)"
echo "  - Reporting Service: http://localhost:8004 (PID: $REPORTING_PID)"
echo "  - Worker Service: http://localhost:8003 (PID: $WORKER_PID)"



# Function to stop services
stop_services() {
    echo "Stopping services..."
    kill $VALIDATION_PID 2>/dev/null
    kill $GATEWAY_PID 2>/dev/null
    kill $ORCHESTRATOR_PID 2>/dev/null
    kill $REPORTING_PID 2>/dev/null
    kill $WORKER_PID 2>/dev/null
    echo "All services stopped."
    exit 0
}

# Trap SIGINT and SIGTERM to stop services gracefully
trap stop_services SIGINT SIGTERM

echo "Press Ctrl+C to stop all services"

# Wait for services
wait
