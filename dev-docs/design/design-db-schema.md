# Report database schema design

This document defines the database schema for storing pipeline execution data used by the `report` subcommand. It aligns with the report implementation plan (`report-subcommand-plan.md`) and the three specification tables: pipeline execution, stage, and job.

**Target database:** PostgreSQL (connection via `DATABASE_URL` or `REPORT_DB_URL`).

---

## 1. Overview

- **Writer:** Execution service (creates/updates rows during and after each run).
- **Reader:** Report API (e.g. on execution service or gateway), used by `cicd report`.
- **Scope:** One row per pipeline run; one row per stage per run; one row per job per run. Timestamps and status are recorded at each level.

---

## 2. Tables

### 2.1 `pipeline_runs`

Records each execution of a pipeline (Table 1: information logged for every execution).

| Column       | Type         | Nullable | Description |
|-------------|--------------|----------|-------------|
| id          | BIGSERIAL    | no       | Primary key (internal). |
| pipeline    | VARCHAR(512) | no       | Pipeline name (`pipeline.name` from YAML). |
| run_no      | INT          | no       | Per-pipeline run number (1, 2, 3, …). |
| start_time  | TIMESTAMPTZ  | no       | When the pipeline run started. |
| end_time    | TIMESTAMPTZ  | yes      | When the pipeline run ended (NULL while running). |
| status      | VARCHAR(32)  | no       | `success`, or `failed`. |
| git_hash    | VARCHAR(64)  | yes      | Git commit hash for this run. |
| git_branch  | VARCHAR(256) | yes      | Git branch for this run. |
| git_repo    | VARCHAR(1024)| yes      | Git repository URL for this run. |

- **Unique constraint:** `(pipeline, run_no)`.
- **Indexes:** `(pipeline)`, `(pipeline, run_no)` for report lookups.

### 2.2 `stage_runs`

Records each stage within a pipeline run (Table 3: information logged for every stage).

| Column      | Type         | Nullable | Description |
|------------|--------------|----------|-------------|
| id         | BIGSERIAL     | no       | Primary key (internal). |
| pipeline   | VARCHAR(512)  | no       | Pipeline this stage belongs to. |
| run_no     | INT           | no       | Run number (references pipeline_runs). |
| stage      | VARCHAR(256)  | no       | Stage name. |
| start_time | TIMESTAMPTZ   | no       | When the stage started. |
| end_time   | TIMESTAMPTZ   | yes      | When the stage ended (NULL while running). |
| status     | VARCHAR(32)   | no       | `success`, or `failed`. |

- **Unique constraint:** `(pipeline, run_no, stage)`.
- **Foreign key:** `(pipeline, run_no)` → `pipeline_runs(pipeline, run_no)` (optional; can be enforced in app or via FK).
- **Indexes:** `(pipeline, run_no)`, `(pipeline, run_no, stage)` for report lookups.

### 2.3 `job_runs`

Records each job within a stage (Table 4: information logged for every job).

| Column      | Type         | Nullable | Description |
|------------|--------------|----------|-------------|
| id         | BIGSERIAL     | no       | Primary key (internal). |
| pipeline   | VARCHAR(512)  | no       | Pipeline this job belongs to. |
| run_no     | INT           | no       | Run number. |
| stage      | VARCHAR(256)  | no       | Stage this job belongs to. |
| job        | VARCHAR(256)  | no       | Job name. |
| start_time | TIMESTAMPTZ   | no       | When the job started. |
| end_time   | TIMESTAMPTZ   | yes      | When the job ended (NULL while running). |
| status     | VARCHAR(32)   | no       | `success`, or `failed`. |

- **Unique constraint:** `(pipeline, run_no, stage, job)`.
- **Indexes:** `(pipeline, run_no)`, `(pipeline, run_no, stage)`, `(pipeline, run_no, stage, job)` for report lookups.

---

## 3. Status values

- **Pipeline / stage / job:** `running` (in progress), `success`, `failed`.
- Report output may expose only `success` / `failed` for completed runs; `running` is used for live runs.

---

## 4. PostgreSQL DDL

```sql
-- Pipeline runs (one per execution)
CREATE TABLE pipeline_runs (
    id          BIGSERIAL PRIMARY KEY,
    pipeline    VARCHAR(512) NOT NULL,
    run_no      INT NOT NULL,
    start_time  TIMESTAMPTZ NOT NULL,
    end_time    TIMESTAMPTZ,
    status      VARCHAR(32) NOT NULL,
    git_hash    VARCHAR(64),
    git_branch  VARCHAR(256),
    git_repo    VARCHAR(1024),
    UNIQUE (pipeline, run_no)
);

CREATE INDEX idx_pipeline_runs_pipeline ON pipeline_runs (pipeline);
CREATE INDEX idx_pipeline_runs_lookup ON pipeline_runs (pipeline, run_no);

-- Stage runs (one per stage per run)
CREATE TABLE stage_runs (
    id          BIGSERIAL PRIMARY KEY,
    pipeline    VARCHAR(512) NOT NULL,
    run_no      INT NOT NULL,
    stage       VARCHAR(256) NOT NULL,
    start_time  TIMESTAMPTZ NOT NULL,
    end_time    TIMESTAMPTZ,
    status      VARCHAR(32) NOT NULL,
    UNIQUE (pipeline, run_no, stage)
);

CREATE INDEX idx_stage_runs_lookup ON stage_runs (pipeline, run_no);
CREATE INDEX idx_stage_runs_stage ON stage_runs (pipeline, run_no, stage);

-- Job runs (one per job per run)
CREATE TABLE job_runs (
    id          BIGSERIAL PRIMARY KEY,
    pipeline    VARCHAR(512) NOT NULL,
    run_no      INT NOT NULL,
    stage       VARCHAR(256) NOT NULL,
    job         VARCHAR(256) NOT NULL,
    start_time  TIMESTAMPTZ NOT NULL,
    end_time    TIMESTAMPTZ,
    status      VARCHAR(32) NOT NULL,
    UNIQUE (pipeline, run_no, stage, job)
);

CREATE INDEX idx_job_runs_lookup ON job_runs (pipeline, run_no);
CREATE INDEX idx_job_runs_stage ON job_runs (pipeline, run_no, stage);
CREATE INDEX idx_job_runs_job ON job_runs (pipeline, run_no, stage, job);
```

---

## 5. Report query mapping

| Report level | CLI flags | Query focus |
|--------------|-----------|-------------|
| All runs for pipeline | `--pipeline <name>` | `SELECT * FROM pipeline_runs WHERE pipeline = $1 ORDER BY run_no` |
| Specific run | `--pipeline <name> --run n` | `pipeline_runs` + `stage_runs` + `job_runs` WHERE `pipeline = $1 AND run_no = $2` |
| Specific stage | `--pipeline <name> --run n --stage s` | Above + filter stage_runs and job_runs by `stage = $3` |
| Specific job | `--pipeline <name> --run n --stage s --job j` | Above + filter job_runs by `job = $4` |

---

## 6. Run number (run_no) generation

- **Rule:** Per-pipeline monotonic integer (1, 2, 3, …).
- **Implementation:** On `CreateRun`, e.g. `SELECT COALESCE(MAX(run_no), 0) + 1 FROM pipeline_runs WHERE pipeline = $1` inside a transaction, then insert the new row with that `run_no`. Alternatively use a small counter table keyed by `pipeline` and increment under lock.
