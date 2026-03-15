-- Add failures column to job_runs (allow-failure jobs: do not affect stage status).
-- Apply after 001_report_store_schema.sql

ALTER TABLE job_runs
ADD COLUMN IF NOT EXISTS failures BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN job_runs.failures IS 'When true, job is allowed to fail and does not affect stage status.';
