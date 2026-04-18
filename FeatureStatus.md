# Feature Status

This document summarizes the current implementation status of user-facing and system features in the repository as of April 13, 2026.

## Implemented Features

- Pipeline YAML parsing and structural validation.
- Dependency verification, including stage/job reference checks and cycle detection.
- CLI commands for `verify`, `dryrun`, `run`, and `report`.
- API Gateway as the single client-facing HTTP entry point.
- Validation Service endpoints for `/validate`, `/dryrun`, `/health`, and `/ready`.
- Execution Service orchestration of pipeline runs, stages, and jobs.
- Worker Service execution of jobs inside containers.
- Reporting Service with pipeline-, run-, stage-, and job-scoped reports.
- MySQL 8 report store for persisted pipeline run history.
- RabbitMQ-backed asynchronous job dispatch between execution and worker services.
- Parallel dispatch of multiple ready jobs in the same stage when dependencies are satisfied.
- Deduplication of identical in-flight run requests using a persisted request key.
- Trace ID persistence in reports for observability correlation.
- Optional allowed-failure job tracking in reports (`failures` field).
- Repo-backed runs for public repositories.
- GitHub credential injection for private repository access in Kubernetes worker pods.
- Docker-based local development stack.
- Kubernetes manifests and Helm chart for deploying all services and supporting dependencies.
- OpenTelemetry, Prometheus, Loki, Tempo, and Grafana integration for metrics, logs, and traces.

## Partly Implemented Features

- Local developer bootstrap is functional but still host-sensitive:
  - local MySQL 8 startup may require a non-default host port if `3306` is already in use;
  - local RabbitMQ startup may require the same care for `5672`.
- Dependency restart recovery is only partially automated:
  - if RabbitMQ or MySQL 8 is restarted while services are already running, some services may need to be restarted to recover cleanly.
- Private repository support is implemented for the documented GitHub token / username-password flow:
  - other Git hosting providers are not documented or tested as first-class authentication targets.

