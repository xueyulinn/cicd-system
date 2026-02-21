-- Report store schema for pipeline run / stage / job (see dev-docs/design/design-db-schema.md).
-- Apply once: psql "$DATABASE_URL" -f migrations/001_report_store_schema.sql

-- Pipeline runs (one per execution)
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
    UNIQUE (pipeline, run_no)
);

CREATE INDEX IF NOT EXISTS idx_pipeline_runs_pipeline ON pipeline_runs (pipeline);
CREATE INDEX IF NOT EXISTS idx_pipeline_runs_lookup ON pipeline_runs (pipeline, run_no);

-- Stage runs (one per stage per run)
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

-- Job runs (one per job per run)
CREATE TABLE IF NOT EXISTS job_runs (
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

CREATE INDEX IF NOT EXISTS idx_job_runs_lookup ON job_runs (pipeline, run_no);
CREATE INDEX IF NOT EXISTS idx_job_runs_stage ON job_runs (pipeline, run_no, stage);
CREATE INDEX IF NOT EXISTS idx_job_runs_job ON job_runs (pipeline, run_no, stage, job);
