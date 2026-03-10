# Developer Installation and Usage

## Purpose

This document describes how developers can build, start, and use the project from source.

## Prerequisites

- Go 1.25.6 or later
- Docker and Docker Compose
- Git

## Get the Source Code

```bash
git clone https://github.com/CS7580-SEA-SP26/e-team.git
cd e-team
```

## Build the CLI

```bash
make build
```

This creates the CLI binary at `bin/cicd`.

## Install the CLI Locally

```bash
make install
```

If needed, add the install directory to your `PATH`.

## Start Backend Services for Development

Run the backend stack from the repository root:

```bash
docker compose up -d --build
```

This starts:

- PostgreSQL
- API Gateway
- Validation Service
- Execution Service
- Worker Service
- Reporting Service
- Database migration service

## Verify the Services

Check container status:

```bash
docker compose ps
```

Check the gateway health endpoint:

```bash
curl http://localhost:8000/health
```

## Run the CLI

Validate a pipeline:

```bash
./bin/cicd verify .pipelines/pipeline.yaml
```

Show execution plan:

```bash
./bin/cicd dryrun .pipelines/pipeline.yaml
```

Run a pipeline:

```bash
./bin/cicd run --file .pipelines/pipeline.yaml
```

## Run Tests

```bash
go test -v ./internal/... ./cmd/...
```

## Stop the Services

```bash
docker compose down
```
