-- Report store schema for pipeline run / stage / job (MySQL 8).
-- Apply through migrations image (or manually with mysql client).

-- Pipeline runs (one per execution)
CREATE TABLE IF NOT EXISTS pipeline_runs (
    id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    pipeline    VARCHAR(255) NOT NULL,
    run_no      INT NOT NULL,
    start_time  DATETIME(6) NOT NULL,
    end_time    DATETIME(6) NULL,
    status      VARCHAR(32) NOT NULL,
    git_hash    VARCHAR(64) NULL,
    git_branch  VARCHAR(256) NULL,
    git_repo    VARCHAR(1024) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_pipeline_runs_pipeline_run (pipeline, run_no),
    KEY idx_pipeline_runs_pipeline (pipeline),
    KEY idx_pipeline_runs_lookup (pipeline, run_no)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Stage runs (one per stage per run)
CREATE TABLE IF NOT EXISTS stage_runs (
    id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    pipeline    VARCHAR(255) NOT NULL,
    run_no      INT NOT NULL,
    stage       VARCHAR(256) NOT NULL,
    start_time  DATETIME(6) NOT NULL,
    end_time    DATETIME(6) NULL,
    status      VARCHAR(32) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_stage_runs_pipeline_run_stage (pipeline, run_no, stage),
    KEY idx_stage_runs_lookup (pipeline, run_no),
    KEY idx_stage_runs_stage (pipeline, run_no, stage)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Job runs (one per job per run)
CREATE TABLE IF NOT EXISTS job_runs (
    id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    pipeline    VARCHAR(255) NOT NULL,
    run_no      INT NOT NULL,
    stage       VARCHAR(256) NOT NULL,
    job         VARCHAR(256) NOT NULL,
    start_time  DATETIME(6) NOT NULL,
    end_time    DATETIME(6) NULL,
    status      VARCHAR(32) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_job_runs_pipeline_run_stage_job (pipeline, run_no, stage, job),
    KEY idx_job_runs_lookup (pipeline, run_no),
    KEY idx_job_runs_stage (pipeline, run_no, stage),
    KEY idx_job_runs_job (pipeline, run_no, stage, job)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Per-pipeline run number allocator used by execution service.
CREATE TABLE IF NOT EXISTS pipeline_sequences (
    pipeline    VARCHAR(255) NOT NULL,
    next_run_no INT NOT NULL DEFAULT 1,
    PRIMARY KEY (pipeline)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
