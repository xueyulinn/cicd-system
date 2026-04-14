# Report database schema design

This document defines the PostgreSQL schema used for historical pipeline reporting.

Target database:

- PostgreSQL, connected through `DATABASE_URL` or `REPORT_DB_URL`

Writers:

- Execution Service
- Worker Service (indirectly, through execution callbacks and job status persistence)

Readers:

- Reporting Service
- API Gateway (via Reporting Service)
- CLI `report` command (via Gateway / Reporting Service)

## 1. Overview

The schema stores one row per:

- pipeline run
- stage run
- job run

It is optimized for:

- pipeline-level run history lookups
- run-level expansion into stages and jobs
- stage/job filtering for the `report` command
- deduplication of identical in-flight run requests
- observability correlation through persisted trace IDs

## 2. Tables

### 2.1 `pipeline_runs`

| Column | Type | Nullable | Description |
|--------|------|----------|-------------|
| `id` | `BIGSERIAL` | no | Internal primary key |
| `pipeline` | `VARCHAR(512)` | no | Logical pipeline name |
| `run_no` | `INT` | no | Per-pipeline monotonic run number |
| `start_time` | `TIMESTAMPTZ` | no | Run start time |
| `end_time` | `TIMESTAMPTZ` | yes | Run completion time |
| `status` | `VARCHAR(32)` | no | `queued`, `running`, `success`, or `failed` |
| `git_hash` | `VARCHAR(64)` | yes | Git commit hash |
| `git_branch` | `VARCHAR(256)` | yes | Git branch |
| `git_repo` | `VARCHAR(1024)` | yes | Git repository URL |
| `trace_id` | `VARCHAR(32)` | yes | OpenTelemetry trace ID |
| `request_key` | `VARCHAR(64)` | yes | Stable deduplication key for identical in-flight requests |

Constraints:

- unique: `(pipeline, run_no)`

Indexes:

- `idx_pipeline_runs_pipeline` on `(pipeline)`
- `idx_pipeline_runs_lookup` on `(pipeline, run_no)`
- `idx_pipeline_runs_request_key_active` on `(pipeline, request_key, run_no)` where `request_key IS NOT NULL AND status IN ('queued', 'running')`

### 2.2 `stage_runs`

| Column | Type | Nullable | Description |
|--------|------|----------|-------------|
| `id` | `BIGSERIAL` | no | Internal primary key |
| `pipeline` | `VARCHAR(512)` | no | Logical pipeline name |
| `run_no` | `INT` | no | Run number within the pipeline |
| `stage` | `VARCHAR(256)` | no | Stage name |
| `start_time` | `TIMESTAMPTZ` | no | Stage start time |
| `end_time` | `TIMESTAMPTZ` | yes | Stage completion time |
| `status` | `VARCHAR(32)` | no | `queued`, `running`, `success`, or `failed` |

Constraints:

- unique: `(pipeline, run_no, stage)`

Indexes:

- `idx_stage_runs_lookup` on `(pipeline, run_no)`
- `idx_stage_runs_stage` on `(pipeline, run_no, stage)`

### 2.3 `job_runs`

| Column | Type | Nullable | Description |
|--------|------|----------|-------------|
| `id` | `BIGSERIAL` | no | Internal primary key |
| `pipeline` | `VARCHAR(512)` | no | Logical pipeline name |
| `run_no` | `INT` | no | Run number within the pipeline |
| `stage` | `VARCHAR(256)` | no | Stage name |
| `job` | `VARCHAR(256)` | no | Job name |
| `start_time` | `TIMESTAMPTZ` | no | Job start time |
| `end_time` | `TIMESTAMPTZ` | yes | Job completion time |
| `status` | `VARCHAR(32)` | no | `queued`, `running`, `success`, or `failed` |
| `failures` | `BOOLEAN` | no | `true` when the job is allowed to fail without failing the stage |

Constraints:

- unique: `(pipeline, run_no, stage, job)`

Indexes:

- `idx_job_runs_lookup` on `(pipeline, run_no)`
- `idx_job_runs_stage` on `(pipeline, run_no, stage)`
- `idx_job_runs_job` on `(pipeline, run_no, stage, job)`

## 3. Status values

The implementation uses the following status values:

- `queued`
- `running`
- `success`
- `failed`

`queued` is used for runs, stages, and jobs that have been created but have not started yet.

## 4. Migration history

The schema evolves through these migrations:

- `001_report_store_schema.sql`
  - creates `pipeline_runs`, `stage_runs`, and `job_runs`
- `002_add_job_runs_failures.sql`
  - adds `job_runs.failures`
- `003_add_trace_id.sql`
  - adds `pipeline_runs.trace_id`
- `004_add_pipeline_run_request_key.sql`
  - adds `pipeline_runs.request_key`
  - adds the active-request deduplication index

## 5. Current DDL summary

```sql
CREATE TABLE IF NOT EXISTS pipeline_runs (
    id          BIGSERIAL PRIMARY KEY,
    pipeline    VARCHAR(512) NOT NULL,
    run_no      INT NOT NULL,
    start_time  TIMESTAMPTZ NOT NULL,
    end_time    TIMESTAMPTZ,
    status      VARCHAR(32) NOT NULL,
    git_hash    VARCHAR(64),
    git_branch  VARCHAR(256),
    git_repo    VARCHAR(1024),
    trace_id    VARCHAR(32) DEFAULT '',
    request_key VARCHAR(64),
    UNIQUE (pipeline, run_no)
);

CREATE INDEX IF NOT EXISTS idx_pipeline_runs_pipeline ON pipeline_runs (pipeline);
CREATE INDEX IF NOT EXISTS idx_pipeline_runs_lookup ON pipeline_runs (pipeline, run_no);
CREATE INDEX IF NOT EXISTS idx_pipeline_runs_request_key_active
ON pipeline_runs (pipeline, request_key, run_no)
WHERE request_key IS NOT NULL AND status IN ('queued', 'running');

CREATE TABLE IF NOT EXISTS stage_runs (
    id          BIGSERIAL PRIMARY KEY,
    pipeline    VARCHAR(512) NOT NULL,
    run_no      INT NOT NULL,
    stage       VARCHAR(256) NOT NULL,
    start_time  TIMESTAMPTZ NOT NULL,
    end_time    TIMESTAMPTZ,
    status      VARCHAR(32) NOT NULL,
    UNIQUE (pipeline, run_no, stage)
);

CREATE INDEX IF NOT EXISTS idx_stage_runs_lookup ON stage_runs (pipeline, run_no);
CREATE INDEX IF NOT EXISTS idx_stage_runs_stage ON stage_runs (pipeline, run_no, stage);

CREATE TABLE IF NOT EXISTS job_runs (
    id          BIGSERIAL PRIMARY KEY,
    pipeline    VARCHAR(512) NOT NULL,
    run_no      INT NOT NULL,
    stage       VARCHAR(256) NOT NULL,
    job         VARCHAR(256) NOT NULL,
    start_time  TIMESTAMPTZ NOT NULL,
    end_time    TIMESTAMPTZ,
    status      VARCHAR(32) NOT NULL,
    failures    BOOLEAN NOT NULL DEFAULT FALSE,
    UNIQUE (pipeline, run_no, stage, job)
);

CREATE INDEX IF NOT EXISTS idx_job_runs_lookup ON job_runs (pipeline, run_no);
CREATE INDEX IF NOT EXISTS idx_job_runs_stage ON job_runs (pipeline, run_no, stage);
CREATE INDEX IF NOT EXISTS idx_job_runs_job ON job_runs (pipeline, run_no, stage, job);
```

## 6. Query mapping for `cicd report`

| Report level | CLI flags | Query focus |
|--------------|-----------|-------------|
| All runs for a pipeline | `--pipeline <name>` | `pipeline_runs` filtered by `pipeline`, ordered by `run_no` |
| Specific run | `--pipeline <name> --run n` | one `pipeline_runs` row plus `stage_runs` for that run |
| Specific stage | `--pipeline <name> --run n --stage s` | one stage from `stage_runs`, plus related jobs from `job_runs` |
| Specific job | `--pipeline <name> --run n --stage s --job j` | one job from `job_runs` |

## 7. Run number allocation and deduplication

Run number allocation:

- per-pipeline monotonic integer sequence
- implemented by querying `MAX(run_no) + 1` inside a transaction
- guarded by a PostgreSQL advisory transaction lock to avoid races

Deduplication:

- `CreateRunOrGetActive` looks for the oldest row with the same `pipeline` and `request_key`
- only rows with status `queued` or `running` are considered active
- if found, the existing `run_no` is reused instead of creating a new run
