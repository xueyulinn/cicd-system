#!/bin/bash

# Start all services script
echo "Starting e-team services..."

# Reporting and execution services need Postgres. Use default if not set (run ./scripts/verify-report-db.sh first).
if [[ -z "${DATABASE_URL:-}" ]] && [[ -z "${REPORT_DB_URL:-}" ]]; then
  export DATABASE_URL="postgres://cicd:cicd@localhost:5432/reportstore?sslmode=disable"
  echo "Note: DATABASE_URL not set, using default (start Postgres with: ./scripts/verify-report-db.sh)"
fi

# Start validation service
echo "Starting validation service on port 8001..."
go run ./cmd/validation-service &
VALIDATION_PID=$!

# Start API gateway
echo "Starting API gateway on port 8000..."
go run ./cmd/api-gateway &
GATEWAY_PID=$!

# Start execution service
echo "Starting execution service on port 8002..."
go run ./cmd/execution-service &
EXECUTION_PID=$!

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
echo "  - Execution Service: http://localhost:8002 (PID: $EXECUTION_PID)"
echo "  - Reporting Service: http://localhost:8004 (PID: $REPORTING_PID)"
echo "  - Worker Service: http://localhost:8003 (PID: $WORKER_PID)"



# Function to stop services
stop_services() {
    echo "Stopping services..."
    kill $VALIDATION_PID 2>/dev/null
    kill $GATEWAY_PID 2>/dev/null
    kill $EXECUTION_PID 2>/dev/null
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
