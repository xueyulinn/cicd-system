-- Add trace_id to pipeline_runs for observability correlation.
-- Apply after 002_add_job_runs_failures.sql

ALTER TABLE pipeline_runs
ADD COLUMN trace_id VARCHAR(32) NOT NULL DEFAULT ''
COMMENT 'OpenTelemetry trace ID (32-char hex) for correlating reports with traces.';
