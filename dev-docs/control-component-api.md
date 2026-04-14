# Control Component API Reference

This document describes the HTTP APIs exposed by the control-plane services in this repository. The public client-facing surface is the API Gateway. Validation, execution, and reporting services also expose internal HTTP endpoints used for service-to-service communication and health checks.

## 1. API Gateway

Base URL:

- local host-process development: `http://localhost:8000`
- Compose: `http://localhost:8000`
- Kubernetes: service or ingress address for the gateway

Primary responsibility:

- single entry point for CLI requests
- proxies validation, dry-run, run, and report requests to downstream services
- aggregates dependency readiness into `/ready`

### `GET /health`

Purpose:

- liveness check for the gateway process only

Response:

```json
{"status":"healthy"}
```

Errors:

- `405 Method Not Allowed` for non-`GET`

### `GET /services`

Purpose:

- return the configured upstream URLs the gateway will use

Response:

```json
{
  "services": {
    "validation": "http://localhost:8001",
    "execution": "http://localhost:8002",
    "reporting": "http://localhost:8004",
    "gateway": "http://localhost:8000"
  }
}
```

### `POST /validate`

Request body:

```json
{
  "yaml_content": "pipeline:\n  name: Demo\nstages:\n  - build\ncompile:\n  - stage: build\n  - image: alpine:latest\n  - script:\n    - echo ok\n"
}
```

Success response:

```json
{"valid":true}
```

Validation failure response:

```json
{
  "valid": false,
  "errors": [
    "content:1:1: pipeline must have at least one job defined"
  ]
}
```

Status codes:

- `200 OK` for valid input
- `400 Bad Request` for malformed JSON, missing `yaml_content`, or validation errors
- `502 Bad Gateway` if the validation service is unreachable

### `POST /dryrun`

Request body:

- same as `POST /validate`

Success response:

```json
{
  "valid": true,
  "output": "build:\n  compile:\n    image: alpine:latest\n    script:\n      - echo ok\n"
}
```

Status codes:

- `200 OK` for valid input
- `400 Bad Request` for malformed JSON, missing `yaml_content`, or dry-run validation errors
- `502 Bad Gateway` if the validation service is unreachable

### `POST /run`

Request body:

```json
{
  "yaml_content": "pipeline:\n  name: Demo\nstages:\n  - smoke\nsmoke:\n  - stage: smoke\n  - image: alpine:latest\n  - script:\n    - echo ok\n",
  "branch": "main",
  "commit": "abc123",
  "repo_url": "https://github.com/example/repo.git",
  "workspace_path": "/tmp/cicd-run-wt-12345"
}
```

Accepted response:

```json
{
  "pipeline": "Demo",
  "run_no": 5,
  "status": "queued"
}
```

Dedup response example:

```json
{
  "pipeline": "Demo",
  "run_no": 5,
  "status": "queued",
  "message": "Duplicate run request dropped; using in-flight run 5."
}
```

Status codes:

- `200 OK` for accepted / queued runs
- `400 Bad Request` for invalid request bodies or execution-service validation failures
- `502 Bad Gateway` when the execution service cannot be reached

### `GET /report`

Query parameters:

- `pipeline` required
- `run` optional integer
- `stage` optional, requires `run`
- `job` optional, requires `run` and `stage`

Examples:

```text
GET /report?pipeline=Default%20Pipeline
GET /report?pipeline=Default%20Pipeline&run=1
GET /report?pipeline=Default%20Pipeline&run=1&stage=build
GET /report?pipeline=Default%20Pipeline&run=1&stage=test&job=unit-tests
```

Success response example:

```json
{
  "pipeline": {
    "name": "Default Pipeline",
    "run-no": 1,
    "status": "success",
    "trace-id": "4593788d3bc7b9b4b9b3592b5969f7b8",
    "stages": [
      {
        "name": "build",
        "status": "success"
      }
    ]
  }
}
```

Status codes:

- `200 OK` for successful queries
- `400 Bad Request` for invalid query parameters
- `404 Not Found` when the pipeline / run / stage / job does not exist
- `502 Bad Gateway` when the reporting service is unreachable

### `GET /ready`

Response example:

```json
{
  "services": {
    "validation": "ready",
    "reporting": "ready",
    "execution": "not ready"
  },
  "status": "not ready"
}
```

Status codes:

- `200 OK` when all required downstream services are ready
- `503 Service Unavailable` when one or more downstream services are not ready

## 2. Validation Service

Base URL:

- local: `http://localhost:8001`

Routes:

- `GET /health`
- `GET /ready`
- `POST /validate`
- `POST /dryrun`

Failure behavior:

- `400 Bad Request` for malformed JSON or invalid pipeline definitions
- `405 Method Not Allowed` for unsupported methods

## 3. Execution Service

Base URL:

- local: `http://localhost:8002`

Primary responsibility:

- validate submitted pipelines
- create persisted run / stage / job records
- deduplicate identical in-flight requests
- enqueue ready jobs to RabbitMQ
- process worker callbacks

Routes:

- `GET /health`
- `GET /ready`
- `POST /run`
- `POST /callbacks/job-started`
- `POST /callbacks/job-finished`

### `POST /callbacks/job-started`

Input example:

```json
{
  "pipeline": "Demo",
  "run_no": 5,
  "stage": "build",
  "job": "compile",
  "status": "running"
}
```

Output:

```json
{"status":"ok"}
```

### `POST /callbacks/job-finished`

Input example:

```json
{
  "pipeline": "Demo",
  "run_no": 5,
  "stage": "build",
  "job": "compile",
  "status": "success",
  "logs": "build output..."
}
```

Failure modes for `/run` and callback endpoints:

- `400 Bad Request` for malformed JSON or validation failures
- `500 Internal Server Error` for orchestration, persistence, or queue failures
- `503 Service Unavailable` when the service failed to initialize or is not ready

## 4. Reporting Service

Base URL:

- local: `http://localhost:8004`

Routes:

- `GET /health`
- `GET /ready`
- `GET /report`

Failure behavior:

- `400 Bad Request` for invalid query parameters
- `404 Not Found` when the requested entity does not exist
- `503 Service Unavailable` from `/ready` when the report store is unavailable

## 5. Shared request and response models

The shared HTTP types live in `internal/api/types.go`.

### `ValidateRequest`

```json
{
  "yaml_content": "..."
}
```

### `ValidateResponse`

```json
{
  "valid": true,
  "errors": []
}
```

### `DryRunResponse`

```json
{
  "valid": true,
  "output": "..."
}
```

### `RunRequest`

Fields:

- `yaml_content`
- `branch`
- `commit`
- `repo_url` optional
- `workspace_path` optional

### `RunResponse`

Fields:

- `pipeline`
- `run_no`
- `status`
- `errors`
- `message`

### `JobStatusCallbackRequest`

Fields:

- `pipeline`
- `run_no`
- `stage`
- `job`
- `status`
- `logs` optional
- `error` optional
