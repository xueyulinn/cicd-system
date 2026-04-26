-- Add request_key to support queue deduplication of identical in-flight runs.
-- Apply after 003_add_trace_id.sql

ALTER TABLE pipeline_runs
ADD COLUMN request_key VARCHAR(64) NULL
COMMENT 'Stable deduplication key for identical in-flight requests',
ADD COLUMN active_request_key VARCHAR(64)
GENERATED ALWAYS AS (
    CASE
        WHEN status IN ('queued', 'running') THEN request_key
        ELSE NULL
    END
) STORED;

CREATE INDEX idx_pipeline_runs_request_key
ON pipeline_runs (pipeline, request_key, run_no);

CREATE UNIQUE INDEX uq_pipeline_runs_active_request
ON pipeline_runs (pipeline, active_request_key);
