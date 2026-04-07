-- Add request_key to support queue deduplication of identical in-flight runs.
-- Apply after 003_add_trace_id.sql

ALTER TABLE pipeline_runs
ADD COLUMN IF NOT EXISTS request_key VARCHAR(64);

CREATE INDEX IF NOT EXISTS idx_pipeline_runs_request_key_active
ON pipeline_runs (pipeline, request_key, run_no)
WHERE status IN ('queued', 'running');
