# CI/CD Pipeline Management Tool

A Go-based CLI tool for validating and managing CI/CD pipeline configurations. This tool parses YAML pipeline files, verifies their structure and dependencies, and can execute pipelines locally through a microservices architecture.

## Overview

The e-team project is a CI/CD pipeline management system that provides:

- **Pipeline Validation**: Parse and validate YAML pipeline configurations (via Validation Service)
- **API Gateway**: Single entry point that routes requests to validation, dry-run, and execution
- **Execution Service**: Orchestrates pipeline runs (stages, jobs, dependencies)
- **Worker Layer**: Executes individual jobs (e.g. in containers) and reports status
- **CLI Interface**: Command-line tool for verify, dryrun, and run

## Architecture (Current Sprint)

| Component            | Port | Description                    |
|----------------------|------|--------------------------------|
| API Gateway          | 8000 | Routes to validation/dryrun/execution |
| Validation Service   | 8001 | Validates pipeline YAML        |
| Execution Service    | 8002 | Runs pipelines, coordinates jobs |
| Worker Service       | 8003 | Executes job steps             |

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

## Usage

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

### Running the services (development)

To run the full stack (API Gateway, Validation, Execution, Worker) in development:

```bash
./scripts/start-services.sh
```

Use another terminal for CLI commands. To point the CLI at a different Execution Service, set `EXECUTION_URL` (default `http://localhost:8002`). Press Ctrl+C in the script terminal to stop all services.

### Report store database (optional)

For the `report` subcommand, execution and report services need a PostgreSQL database. Start Postgres and apply the schema:

```bash
docker compose up -d
export DATABASE_URL="postgres://cicd:cicd@localhost:5432/reportstore?sslmode=disable"
psql "$DATABASE_URL" -f migrations/001_report_store_schema.sql
```

See [dev-docs/report-db-setup.md](dev-docs/report-db-setup.md) for connection config (`DATABASE_URL` or `REPORT_DB_URL`) and CI notes.

### Project Structure

```
e-team/
├── cmd/                     # Application entry points
│   ├── cicd/                # CLI (verify, dryrun, run)
│   │   └── main.go
│   ├── api-gateway/         # API Gateway (port 8000)
│   ├── validation-service/  # Validation Service (port 8001)
│   ├── execution-service/   # Execution Service (port 8002)
│   └── worker-service/      # Worker Service (port 8003)
├── internal/
│   ├── cli/                 # CLI commands and gateway client
│   │   ├── root.go, verify.go, dryrun.go, run.go
│   │   └── gateway_client.go
│   ├── models/              # Data models and types
│   ├── parser/              # YAML parsing logic
│   ├── verifier/            # Validation logic
│   ├── scheduler/           # Scheduler logic
│   ├── dryrun/              # Dryrun logic
│   └── services/            # Gateway, validation, execution, worker
├── scripts/
│   └── start-services.sh    # Start all services (gateway, validation, execution, worker)
├── .pipelines/              # Pipeline configurations
├── dev-docs/                # Development documentation
└── Makefile                 # Build automation
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

- **Language**: Go 1.25.6
- **CLI Framework**: Cobra
- **YAML Parsing**: gopkg.in/yaml.v3
- **Testing**: Go standard testing package
- **Services**: HTTP APIs (API Gateway, Validation, Execution, Worker) for validation, dry-run, and pipeline execution

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
