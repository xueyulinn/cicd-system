package store

import (
	"context"
)

// CreateRun inserts a new pipeline run and allocates run_no in a transaction (concurrency-safe).
// Returns the allocated run_no.
func (s *Store) CreateRun(ctx context.Context, in CreateRunInput) (runNo int, err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	// Serialize run_no allocation per pipeline using advisory lock (Postgres does not allow FOR UPDATE with MAX).
	_, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, in.Pipeline)
	if err != nil {
		return 0, err
	}

	err = tx.QueryRow(ctx,
		`SELECT COALESCE(MAX(run_no), 0) + 1 FROM pipeline_runs WHERE pipeline = $1`,
		in.Pipeline,
	).Scan(&runNo)
	if err != nil {
		return 0, err
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO pipeline_runs (pipeline, run_no, start_time, status, git_hash, git_branch, git_repo, trace_id)
		 VALUES ($1, $2, $3, $4, NULLIF($5,''), NULLIF($6,''), NULLIF($7,''), NULLIF($8,''))`,
		in.Pipeline, runNo, in.StartTime, in.Status, in.GitHash, in.GitBranch, in.GitRepo, in.TraceID,
	)
	if err != nil {
		return 0, err
	}

	return runNo, tx.Commit(ctx)
}

// UpdateRun updates end_time and/or status for a pipeline run.
func (s *Store) UpdateRun(ctx context.Context, pipeline string, runNo int, in UpdateRunInput) error {
	if in.EndTime != nil {
		_, err := s.pool.Exec(ctx,
			`UPDATE pipeline_runs SET end_time = $1, status = $2 WHERE pipeline = $3 AND run_no = $4`,
			*in.EndTime, in.Status, pipeline, runNo,
		)
		return err
	}
	_, err := s.pool.Exec(ctx,
		`UPDATE pipeline_runs SET status = $1 WHERE pipeline = $2 AND run_no = $3`,
		in.Status, pipeline, runNo,
	)
	return err
}

// CreateStage inserts a new stage run.
func (s *Store) CreateStage(ctx context.Context, in CreateStageInput) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO stage_runs (pipeline, run_no, stage, start_time, status)
		 VALUES ($1, $2, $3, $4, $5)`,
		in.Pipeline, in.RunNo, in.Stage, in.StartTime, in.Status,
	)
	return err
}

// UpdateStage updates end_time and status for a stage run.
func (s *Store) UpdateStage(ctx context.Context, pipeline string, runNo int, stage string, in UpdateStageInput) error {
	if in.EndTime != nil {
		_, err := s.pool.Exec(ctx,
			`UPDATE stage_runs SET end_time = $1, status = $2 WHERE pipeline = $3 AND run_no = $4 AND stage = $5`,
			*in.EndTime, in.Status, pipeline, runNo, stage,
		)
		return err
	}
	_, err := s.pool.Exec(ctx,
		`UPDATE stage_runs SET status = $1 WHERE pipeline = $2 AND run_no = $3 AND stage = $4`,
		in.Status, pipeline, runNo, stage,
	)
	return err
}

// CreateJob inserts a new job run.
func (s *Store) CreateJob(ctx context.Context, in CreateJobInput) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO job_runs (pipeline, run_no, stage, job, start_time, status, failures)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		in.Pipeline, in.RunNo, in.Stage, in.Job, in.StartTime, in.Status, in.Failures,
	)
	return err
}

// UpdateJob updates end_time and status for a job run.
func (s *Store) UpdateJob(ctx context.Context, pipeline string, runNo int, stage, job string, in UpdateJobInput) error {
	if in.EndTime != nil {
		_, err := s.pool.Exec(ctx,
			`UPDATE job_runs SET end_time = $1, status = $2 WHERE pipeline = $3 AND run_no = $4 AND stage = $5 AND job = $6`,
			*in.EndTime, in.Status, pipeline, runNo, stage, job,
		)
		return err
	}
	_, err := s.pool.Exec(ctx,
		`UPDATE job_runs SET status = $1 WHERE pipeline = $2 AND run_no = $3 AND stage = $4 AND job = $5`,
		in.Status, pipeline, runNo, stage, job,
	)
	return err
}

