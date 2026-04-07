# CI/CD Pipeline Management Tool

A Go-based CLI tool for validating and managing CI/CD pipeline configurations. This tool parses YAML pipeline files, verifies their structure and dependencies, and can execute pipelines locally through a microservices architecture.

## Overview

The e-team project is a CI/CD pipeline management system that provides:

- **Pipeline Validation**: Parse and validate YAML pipeline configurations (via Validation Service)
- **API Gateway**: Single entry point that routes requests to validation, dry-run, and execution
- **Execution Service**: Orchestrates pipeline runs (stages, jobs, dependencies)
- **Worker Layer**: Executes individual jobs (e.g. in containers) and reports status
- **Queue Deduplication**: Reuses the oldest in-flight run for identical requests and drops duplicates
- **CLI Interface**: Command-line tool for verify, dryrun, and run

## Architecture (Current Sprint)

| Component            | Port | Description                    |
|----------------------|------|--------------------------------|
| API Gateway          | 8000 | Routes to validation/dryrun/execution |
| Validation Service   | 8001 | Validates pipeline YAML        |
| Execution Service    | 8002 | Runs pipelines, coordinates jobs |
| Worker Service       | 8003 | Executes job steps             |
| Reporting Service    | 8004 | Pipeline run reports           |
| PostgreSQL           | 5432 | Report store database          |
| Prometheus           | 9090 | Metrics collection & storage   |
| Loki                 | 3100 | Log aggregation & storage      |
| Tempo                | 3200 | Distributed tracing storage    |
| OTel Collector       | 4318 | Telemetry ingestion & routing  |
| Grafana              | 3000 | Unified observability UI       |

## Kubernetes Support

The repository supports Kubernetes deployment for all current stateless services and optional in-cluster deployment for the report database.

| Component | Type | K8s Enabled | Notes |
|-----------|------|-------------|-------|
| API Gateway | Stateless | Yes | Deploy via Helm (`charts/e-team/`) |
| Validation Service | Stateless | Yes | Same as API Gateway |
| Execution Service | Stateless | Yes | Same as API Gateway |
| Worker Service | Stateless | Yes | Requires Docker socket access on the cluster node |
| Reporting Service | Stateless | Yes | Same as API Gateway |
| PostgreSQL report store | Stateful | Optional | Can run in-cluster via StatefulSet + PVC or externally |

### Kubernetes Deployment Modes

- **All-in-cluster**: install the Helm chart with `postgres.enabled=true` to run stateless services, Postgres, and the migration Job inside Kubernetes.
- **Hybrid**: install the Helm chart with `postgres.enabled=false` and point `externalDatabase.*` / `externalDatabase.url` at a database outside Kubernetes.

Service-to-service communication inside the cluster is done through Kubernetes Services and environment variables:

- `VALIDATION_URL`
- `EXECUTION_URL`
- `REPORTING_URL`
- `WORKER_URL`
- `DATABASE_URL`

For Helm packaging, install/upgrade/uninstall commands, log access, Minikube validation, and troubleshooting, see [`charts/e-team/README.md`](https://github.com/CS7580-SEA-SP26/e-team/blob/review/charts/e-team/README.md).

**Single source of truth:** local Compose reads `compose.values.env`, generated from `charts/e-team/values.yaml` (`ruby scripts/gen-compose-env-from-values.rb`) — images, Postgres, RabbitMQ image/credentials/`RABBITMQ_URL`, `WORKER_CONCURRENCY` (from `workerService.concurrency`), and worker `EXECUTION_URL`. Cluster deployment uses Helm (`charts/e-team/`); run `helm template` if you need to inspect rendered YAML.

### Queue Deduplication

The execution service deduplicates identical in-flight run requests. If a second request arrives while an equivalent run for the same pipeline is still `queued` or `running`, the oldest request continues and the duplicate request is dropped. The duplicate response returns the original `run_no` together with a message indicating that the existing in-flight run was reused.

The deduplication key is derived from the pipeline name, YAML content, branch, commit, and repository URL. It intentionally ignores the caller's temporary workspace path so repeated CLI invocations against the same Git revision can deduplicate correctly.

### Repository-Backed Runs in Kubernetes

For repo-backed runs, the worker clones the requested repository revision inside Kubernetes before executing the job container. Public repositories work without extra configuration. Private repositories require Git credentials to be injected into the worker pod.

For Helm deployments, configure one of:

- `workerService.gitAuth.githubToken`
- `workerService.gitAuth.username` with `workerService.gitAuth.password`
- `workerService.gitAuth.existingSecret`

The worker uses those credentials when cloning private repositories. If repo-backed runs fail with GitHub `401` or `Repository not found`, verify that the token has read access to the target repository.

## Observability

All CI/CD services are instrumented across three pillars — metrics, logs, and traces — using an open-source stack deployed alongside the application via Docker Compose.

### Stack

| Role | Component | Config location |
|------|-----------|-----------------|
| Metrics collection & storage | Prometheus | `observability/prometheus/prometheus.yml` |
| Log aggregation | Loki + Promtail | `observability/loki/loki-config.yml`, `observability/promtail/promtail-config.yml` |
| Distributed tracing | Tempo | `observability/tempo/tempo.yml` |
| Telemetry routing | OpenTelemetry Collector | `observability/otel-collector/config.yml` |
| Visualization | Grafana | `observability/grafana/` |

This implementation uses the recommended stack with one documented substitution: traces are emitted to the OTel Collector, Prometheus scrapes `/metrics` endpoints directly, and Promtail scrapes structured container logs and forwards them to Loki. This keeps job-container stdout/stderr observable without modifying job images while still using only open-source components. Grafana queries Prometheus, Loki, and Tempo.

### Metrics

Every service exposes a `/metrics` endpoint scraped by Prometheus. The following CI/CD-specific metrics are recorded:

| Metric | Type | Labels | Emitted by |
|--------|------|--------|------------|
| `cicd_pipeline_runs_total` | Counter | `pipeline`, `status` (+ `run_no`) | Execution Service |
| `cicd_pipeline_duration_seconds` | Histogram | `pipeline` (+ `run_no`) | Execution Service |
| `cicd_stage_duration_seconds` | Histogram | `pipeline`, `stage` (+ `run_no`) | Execution Service |
| `cicd_job_duration_seconds` | Histogram | `pipeline`, `stage`, `job` (+ `run_no`) | Worker Service |
| `cicd_job_runs_total` | Counter | `pipeline`, `stage`, `job`, `status` (+ `run_no`) | Worker Service |
| `cicd_mq_jobs_published_total` | Counter | `queue`, `outcome` (`success` / `failure`) | Execution Service (via MQ publisher) |
| `cicd_mq_delivery_outcomes_total` | Counter | `queue`, `outcome` (`acked`, `nack_requeue`, `ack_error`) | Worker Service (RabbitMQ consumer) |
| `cicd_execution_ready_batch_size` | Histogram | — | Execution Service (batch of parallel-ready jobs per dispatch) |
| `cicd_execution_jobs_enqueued_total` | Counter | `pipeline`, `stage` | Execution Service |

Inbound HTTP metrics (`http_requests_total`, `http_request_duration_seconds`) are recorded by all services via middleware. Outbound calls from the API Gateway and Execution Service use `http_client_requests_total` and `http_client_request_duration_seconds` (labels `client`, `upstream`, and `code` for the counter). The Grafana dashboard **HTTP Latency (Server & Client)** (`observability/grafana/dashboards/http-latency.json`) charts both. Async pipeline execution (RabbitMQ and parallel-ready job batches) is covered by **Parallel execution & RabbitMQ** (`observability/grafana/dashboards/parallel-mq.json`).

### Structured Logs

All services emit JSON-structured logs via Go's `log/slog`. Each entry includes `time`, `level`, `service`, and `msg`. Context fields (`pipeline`, `run_no`, `stage`, `job`, `trace_id`, `span_id`) are added where applicable. When the execution service dispatches more than one parallel-ready job in a single batch, it logs `event: "mq-dispatch-batch"` with `batch_size`. Job container stdout/stderr is forwarded by the Worker Service with a `source: "job-container"` label.

### Distributed Tracing

Services use OpenTelemetry SDK with W3C Trace Context propagation. The following spans are emitted:

- **`pipeline.run`** (root span) — full pipeline execution, with `pipeline` and `run_no` attributes
- **`stage.run`** (child) — per-stage execution
- **`job.run`** (child) — per-job execution in the Worker
- **`mq.job.publish`** — execution service publishes one job message to the queue (`pipeline`, `run_no`, `stage`, `job`)
- **`mq.job.consume`** — worker handles one delivery (`pipeline`, `run_no`, `stage`, `job`)

Outbound HTTP calls between services automatically propagate trace context.

The `trace-id` is persisted in the report database and included in `report --run` output, allowing operators to correlate a report with its trace in Grafana/Tempo.

### Accessing the Observability Stack

After `docker compose --env-file compose.values.env up -d`:

| Tool | URL | Credentials |
|------|-----|-------------|
| Grafana | http://localhost:3000 | admin / admin |
| Prometheus | http://localhost:9090 | — |
| Tempo (API) | http://localhost:3200 | — |
| Loki (API) | http://localhost:3100 | — |

In Grafana, use the Explore view to query Prometheus (metrics), Loki (logs), or Tempo (traces). Paste a `trace-id` from a report into Tempo to view the full span hierarchy.

The following dashboards are provisioned from files committed to the repository:

- `Pipeline Overview`
- `Stage and Job Breakdown`
- `Logs Viewer`
- `Trace Explorer`
- `Parallel execution & RabbitMQ`

`Pipeline Overview` includes a recent-runs table backed by structured execution logs, including pipeline name, run number, branch, commit hash, status, duration, and `trace_id`. The `trace_id` column links into `Trace Explorer`.

### Configuration

Observability is controlled by environment variables set in `docker-compose.yaml`:

| Variable | Purpose |
|----------|---------|
| `SERVICE_NAME` | Identifies the service in logs and traces |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTel Collector endpoint for traces |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | Transport protocol (`http/protobuf`) |

All observability configuration files are committed under `observability/`. The shared instrumentation library lives in `internal/observability/`.

## Features

- Validate pipeline configuration files (YAML format)
- Check for circular dependencies between jobs
- Verify stage definitions and job assignments
- Support for complex dependency graphs
- Detailed error reporting with file locations
- Batch validation of multiple pipeline files
- Git repository validation
- Dryrun: show execution order without running
- **Run**: execute a pipeline locally (requires services to be running)

## Installation

### Prerequisites

- Go 1.25.6 or later
- Git repository (validation requires git repo)

### Build from Source

```bash
# Clone the repository
git clone https://github.com/CS7580-SEA-SP26/e-team.git
cd e-team

# Build (Windows / macOS / Linux)
make build
# or manually:
go build -o bin/cicd ./cmd/cicd

# Install to $HOME/bin (optional)
make install
# or: go install ./cmd/cicd

# If "cicd: command not found" on macOS/Linux, add Go bin to PATH:
echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.zshrc
source ~/.zshrc
```

The binary will be built as `bin/cicd` and can be installed to `$HOME/bin` by default.

## Quick Start: Run a Pipeline

1. **Start all services** (API Gateway, Validation, Execution, Worker):

   ```bash
   ./scripts/start-services.sh
   ```

   Leave this terminal running. In another terminal, continue with the steps below.

2. **Build the CLI** (if not already installed):

   ```bash
   make build
   ```

3. **Run a pipeline**:

   ```bash
   # Run default pipeline (.pipelines/pipeline.yaml), current branch & latest commit
   ./bin/cicd run --file .pipelines/pipeline.yaml

   # Or by pipeline name (resolved under .pipelines/)
   ./bin/cicd run --name pipeline.yaml

   # With specific branch and commit
   ./bin/cicd run --file .pipelines/pipeline.yaml --branch main --commit HEAD
   ```

   The CLI sends the pipeline YAML to the Execution Service (`http://localhost:8002` by default; override with `EXECUTION_URL`).

## Usage: How to Run and Observe Pipelines

This section describes how a user can run example pipelines and observe both successful and failing behavior, as well as reports generated by the system.

### Repository and Example Pipelines

- **Repository**: `CS7580-SEA-SP26/e-team` (this repository).
- **Source code**: Go implementation of the CLI and microservices under `cmd/` and `internal/`.
- **Pipeline definitions**: YAML pipelines under the `.pipelines/` directory, including:
  - `.pipelines/pipeline.yaml`: a complete, **successful** example pipeline (`Default Pipeline`).
  - Additional pipelines that are **intentionally invalid** to demonstrate failure behavior, such as `.pipelines/circular_dependency.yaml` and other files in `.pipelines/`.

All examples compile, have tests, and exercise real pipeline validation and execution behavior.

### Basic Commands

```bash
# Verify default pipeline file (.pipelines/pipeline.yaml)
cicd verify

# Verify specific pipeline file
cicd verify path/to/pipeline.yaml

# Verify all YAML files in a directory
cicd verify .pipelines/
```

```bash
# Dryrun (show execution order)
cicd dryrun
cicd dryrun path/to/pipeline.yaml
```

```bash
# Run pipeline (services must be running)
cicd run --file .pipelines/pipeline.yaml
cicd run --name pipeline.yaml --branch main --commit HEAD
```

### Commands to Run After Installation

> All commands below are assumed to be run from the repository root `e-team/`.  
> Make sure the components are installed and verified as described in `dev-docs/evaluator-verification.md`.

#### Pipelines That Succeed

1. **Start the services (if not already running)**:

   ```bash
   ./scripts/start-services.sh
   ```

2. **Run the default successful pipeline**:

   ```bash
   cicd run --file .pipelines/pipeline.yaml
   # or, using the logical pipeline name:
   cicd run --name "Default Pipeline"
   ```

   Expected behavior:

   - Logs show the execution of stages `build`, `test`, and `deploy`.
   - The command prints `Run completed successfully.` and exits with status code `0`.

#### Pipelines That Fail During a Run

1. **Validation‑time failure (invalid configuration)**:

   ```bash
   cicd verify .pipelines/circular_dependency.yaml
   ```

   This pipeline (`Circular Dependencies Pipeline`) contains a circular dependency between jobs.  
   Expected behavior:

   - The CLI prints clear validation errors that describe the cycle.
   - The command exits with a non‑zero status code.

2. **Execution‑time failure (job script error)**:

   - Modify `.pipelines/pipeline.yaml` so that one job’s `script` contains a failing command, for example:

     ```yaml
     - script:
       - "go test -v ./internal/... ./cmd/..."
       - "exit 1"  # force a runtime failure for demonstration
     ```

   - Re‑run:

     ```bash
     cicd run --file .pipelines/pipeline.yaml
     ```

   Expected behavior:

   - Logs show which job failed and why.
   - The CLI prints `run failed` and exits with a non‑zero status code.

#### Reports That Users Can Generate

1. **Prepare data by running a pipeline**:

   ```bash
   ./scripts/verify-report-db.sh       # ensure report DB and schema are ready
   ./scripts/start-services.sh         # start services (if not already running)
   cicd run --file .pipelines/pipeline.yaml
   ```

2. **Generate reports at different granularities**:

   - All runs for a pipeline:

     ```bash
     cicd report --pipeline "Default Pipeline"
     ```

   - A specific run (for example, run number 1):

     ```bash
     cicd report --pipeline "Default Pipeline" --run 1
     ```

   - A specific stage within a run:

     ```bash
     cicd report --pipeline "Default Pipeline" --run 1 --stage build
     ```

   - A specific job within a stage and run:

     ```bash
     cicd report --pipeline "Default Pipeline" --run 1 --stage test --job unit-tests
     ```

   - JSON output for programmatic consumption:

     ```bash
     cicd report --pipeline "Default Pipeline" --run 1 -f json
     ```

#### Pipelines That Fail Validation

To observe how the system reports invalid configurations (without executing them), run `cicd verify` against the example files in `.pipelines/`, for example:

```bash
# No stages defined
cicd verify .pipelines/no_stages.yaml

# Duplicate stages or jobs
cicd verify .pipelines/duplicate_stages.yaml
cicd verify .pipelines/duplicate_jobs.yaml

# References to undefined jobs or stages
cicd verify .pipelines/undefined_jobs.yaml
cicd verify .pipelines/wrong_stage_type.yaml
```

For each invalid pipeline, the CLI prints detailed error messages (including file name and location where available) and exits with a non‑zero status code, demonstrating the system’s behavior on malformed inputs.

### Available Sub-commands

- `verify [config-file]` - Validate pipeline configuration files (via gateway or direct)
- `dryrun [config-file]` - Show execution order for stages and jobs
- `run` - Execute a pipeline locally (requires Execution Service; use `--file` or `--name`, optional `--branch`, `--commit`)
- `help` - Show help information

## Pipeline Configuration Format

The tool expects YAML files with the following structure:

```yaml
# Pipeline metadata (optional)
pipeline:
  name: "Example Pipeline"

# Stage definitions
stages:
  - build
  - test
  - deploy

# Job definitions
job-name:
  - stage: build
  - image: golang:1.21
  - script:
    - "make build"

another-job:
  - stage: test
  - needs: [job-name]
  - image: golang:1.21
  - script:
    - "make test"
```

### Configuration Elements

- **pipeline**: Optional metadata with pipeline name
- **stages**: Array of stage names (must be defined before use)
- **jobs**: Job definitions with the following properties:
  - `stage`: Which stage the job belongs to (required)
  - `image`: Docker image to use (required)
  - `script`: Commands to execute (required)
  - `needs`: List of job dependencies (optional)

## Dryrun Output

``` yaml
build:
    compile:
        image: golang:1.21
        script:
            - make build
test:
    unit-tests:
        image: golang:1.21
        script:
            - make test
    integration-tests:
        image: golang:1.21
        script:
            - make integration
deploy:
    deploy-staging:
        image: alpine:latest
        script:
            - deploy staging
```

## Development

### Running the full stack (Docker Compose)

The recommended way to run everything locally — all services, database, migrations, and the observability stack:

```bash
# Align image tags and DB defaults with charts/e-team/values.yaml (committed compose.values.env)
ruby scripts/gen-compose-env-from-values.rb

# Start all containers (builds services from local source via docker-compose.override.yaml)
docker compose --env-file compose.values.env up -d

# Verify everything is running
docker compose --env-file compose.values.env ps

# View logs
docker compose --env-file compose.values.env logs -f execution-service worker-service
```

`docker-compose.yaml` uses `${...}` values from `compose.values.env`. `docker-compose.override.yaml` (automatically loaded) adds `build:` directives so services are built from local source code. This means:

- `docker compose --env-file compose.values.env up -d` — builds from local source (development)
- `docker compose --env-file compose.values.env up -d --build` — forces a rebuild after code changes
- `docker compose --env-file compose.values.env -f docker-compose.yaml up -d` — uses registry images only (CI/production)

`compose.values.env` is generated from `charts/e-team/values.yaml` (same knobs as Helm where applicable: Postgres, images, RabbitMQ credentials and URL, `workerService.concurrency` as `WORKER_CONCURRENCY`, worker `EXECUTION_URL` for in-network DNS). Regenerate after editing values: `ruby scripts/gen-compose-env-from-values.rb`.

#### Local parallel execution (RabbitMQ + worker)

To exercise **multiple ready jobs in one stage** (parallel dispatch to the queue and, with enough consumers, overlapping job runs):

1. Ensure RabbitMQ is healthy and execution/worker were not started before the broker was ready. If execution returns `503` / `fail to create RabbitMQ client`, recreate it: `docker compose --env-file compose.values.env up -d --force-recreate execution-service`.
2. Set concurrency via `workerService.concurrency` in `values.yaml`, then run `ruby scripts/gen-compose-env-from-values.rb` so `WORKER_CONCURRENCY` matches (default `2`).
3. Run the sample pipeline:

   ```bash
   ./bin/cicd run --file .pipelines/parallel-local.yaml
   ```

   See `.pipelines/parallel-local.yaml` for the DAG (two independent jobs in `build`). With `WORKER_CONCURRENCY>=2`, both can run concurrently; with `1`, they still dispatch in one batch but run serially.

### Running services without Docker

To run services directly (without Docker Compose):

```bash
./scripts/start-services.sh
```

Use another terminal for CLI commands. To point the CLI at a different Execution Service, set `EXECUTION_URL` (default `http://localhost:8002`). Press Ctrl+C in the script terminal to stop all services.

### Report store database

For the `report` subcommand, execution and report services need a PostgreSQL database. With Docker Compose this is handled automatically (Postgres + migrations start together). For manual setup:

```bash
docker compose --env-file compose.values.env up -d postgres db-migrate
export DATABASE_URL="postgres://cicd:cicd@localhost:5432/reportstore?sslmode=disable"
```

See [dev-docs/report-db-setup.md](dev-docs/report-db-setup.md) for connection config (`DATABASE_URL` or `REPORT_DB_URL`) and CI notes.

### Project Structure

```
e-team/
├── cmd/                          # Application entry points
│   ├── cicd/                     # CLI (verify, dryrun, run, report)
│   ├── api-gateway/              # API Gateway (port 8000)
│   ├── validation-service/       # Validation Service (port 8001)
│   ├── execution-service/        # Execution Service (port 8002)
│   ├── worker-service/           # Worker Service (port 8003)
│   └── reporting-service/        # Reporting Service (port 8004)
├── internal/
│   ├── cli/                      # CLI commands and gateway client
│   ├── models/                   # Data models and types
│   ├── observability/            # Shared instrumentation (metrics, logs, tracing)
│   ├── store/                    # Database access layer (report store)
│   └── services/                 # Gateway, validation, execution, worker, reporting
├── migrations/                   # SQL schema migrations and Dockerfile
├── observability/                # Observability stack configuration
│   ├── prometheus/               # Prometheus scrape config
│   ├── loki/                     # Loki log aggregation config
│   ├── tempo/                    # Tempo tracing config
│   ├── otel-collector/           # OpenTelemetry Collector pipeline config
│   └── grafana/                  # Grafana provisioning and dashboards
├── charts/e-team/                # Helm chart for Kubernetes deployment (canonical)
├── k8s/                          # Notes for Kubernetes (see k8s/README.md)
├── scripts/                      # Dev scripts (start services, verify DB)
├── .pipelines/                   # Pipeline configurations
├── compose.values.env            # Compose env (generated from chart values; includes MQ + worker concurrency)
├── docker-compose.yaml           # Full stack (services + observability)
├── docker-compose.override.yaml  # Local dev overrides (build from source)
└── Makefile                      # Build automation
```


### Running Tests

```bash
# Run all tests
go test -v ./internal/...

# Run tests with coverage
go test -coverprofile=coverage.out ./internal/...
go tool cover -html=coverage.out -o coverage.html

# Open coverage report in browser
start coverage.html
```

## Validation Rules

The tool enforces the following validation rules:

1. **Git Repository**: Must be run within a Git repository
2. **Stage Definitions**: All stages must be defined before use
3. **Job References**: Jobs in `needs` must exist and are from the same stage
4. **Circular Dependencies**: No circular dependencies allowed
5. **Stage Assignment**: Every job must be assigned to a valid stage
6. **YAML Syntax**: Files must be valid YAML

## Error Examples

### Invalid Configuration
```yaml
# Missing stage definition
stages: []
job-name:
  - stage: undefined_stage
```

**Error Output:**
```
.pipelines/pipeline.yaml:8:3: job 'job-name' references undefined stage 'undefined_stage'
```

### Circular Dependency
```yaml
stages: [build]
job-a:
  - stage: build
  - needs: [job-b]
job-b:
  - stage: build
  - needs: [job-a]
```

**Error Output:**
```
.pipelines/pipeline.yaml: circular dependency detected between jobs: job-a -> job-b -> job-a
```

## Technology Stack

- **Language**: Go 1.25
- **CLI Framework**: Cobra
- **YAML Parsing**: gopkg.in/yaml.v3
- **Database**: PostgreSQL 16 with pgx driver
- **Observability**: OpenTelemetry SDK, Prometheus, Loki, Tempo, Grafana
- **Containers**: Docker, Docker Compose
- **Kubernetes**: Helm, Kustomize, raw manifests
- **Testing**: Go standard testing package

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Run `go test -coverprofile=coverage.out ./internal/...` to ensure coverage
6. Submit a pull request

## License

This project is licensed under the terms specified in the [LICENSE](LICENSE) file.

## Team

This project is developed by the e-team for CS7580 SEA-SP26.
