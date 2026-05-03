# Report Database Schema Design

This document defines the MySQL 8 schema used for historical pipeline reporting.

Target database:

- MySQL 8, connected through `DATABASE_URL` or `REPORT_DB_URL`

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

Core columns:

- `id` `BIGINT UNSIGNED AUTO_INCREMENT` (PK)
- `pipeline` `VARCHAR(512)` (not null)
- `run_no` `INT` (not null, unique per pipeline)
- `start_time` `DATETIME(6)` (not null)
- `end_time` `DATETIME(6)` (nullable)
- `status` `VARCHAR(32)` (not null)
- `git_hash` `VARCHAR(64)` (nullable)
- `git_repo` `VARCHAR(1024)` (nullable)
- `trace_id` `VARCHAR(32)` (not null, default empty string)
- `request_key` `VARCHAR(64)` (nullable)
- `active_request_key` generated column:
  - `CASE WHEN status IN ('queued','running') THEN request_key ELSE NULL END`

Constraints and indexes:

- unique `(pipeline, run_no)`
- unique `(pipeline, active_request_key)` to prevent duplicate active runs for the same request key
- lookup indexes on `(pipeline)` and `(pipeline, run_no)`
- request key index on `(pipeline, request_key, run_no)`

### 2.2 `stage_runs`

Core columns:

- `id` `BIGINT UNSIGNED AUTO_INCREMENT` (PK)
- `pipeline` `VARCHAR(512)` (not null)
- `run_no` `INT` (not null)
- `stage` `VARCHAR(256)` (not null)
- `start_time` `DATETIME(6)` (not null)
- `end_time` `DATETIME(6)` (nullable)
- `status` `VARCHAR(32)` (not null)

Constraints and indexes:

- unique `(pipeline, run_no, stage)`
- lookup indexes on `(pipeline, run_no)` and `(pipeline, run_no, stage)`

### 2.3 `job_runs`

Core columns:

- `id` `BIGINT UNSIGNED AUTO_INCREMENT` (PK)
- `pipeline` `VARCHAR(512)` (not null)
- `run_no` `INT` (not null)
- `stage` `VARCHAR(256)` (not null)
- `job` `VARCHAR(256)` (not null)
- `start_time` `DATETIME(6)` (not null)
- `end_time` `DATETIME(6)` (nullable)
- `status` `VARCHAR(32)` (not null)
- `failures` `BOOLEAN` (not null, default false)

Constraints and indexes:

- unique `(pipeline, run_no, stage, job)`
- lookup indexes on `(pipeline, run_no)`, `(pipeline, run_no, stage)`, `(pipeline, run_no, stage, job)`

### 2.4 `pipeline_sequences`

Used for monotonic per-pipeline `run_no` allocation:

- `pipeline` `VARCHAR(512)` (PK)
- `next_run_no` `INT` (not null)

## 3. Status Values

The implementation uses:

- `queued`
- `running`
- `success`
- `failed`

## 4. Migration History

- `001_report_store_schema.sql`
  - creates `pipeline_runs`, `stage_runs`, `job_runs`, `pipeline_sequences`
- `002_add_job_runs_failures.sql`
  - adds `job_runs.failures`
- `003_add_trace_id.sql`
  - adds `pipeline_runs.trace_id`
- `004_add_pipeline_run_request_key.sql`
  - adds `pipeline_runs.request_key`
  - adds generated `active_request_key`
  - adds dedupe/request-key indexes
- `005_drop_pipeline_run_git_branch.sql`
  - removes `pipeline_runs.git_branch`

## 5. Run Number Allocation and Deduplication

Run number allocation:

- per-pipeline monotonic integer sequence via `pipeline_sequences`
- sequence allocation happens inside the same DB transaction as run insert

Deduplication:

- `CreateRunOrGetActive` first checks for oldest active run (`queued`/`running`) by `(pipeline, request_key)`
- unique index on `(pipeline, active_request_key)` prevents duplicate active inserts under concurrency
