package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// CreateRunResult holds the outcome of CreateRunOrGetActive, indicating
// whether a new run was created or an existing in-flight run was returned.
type CreateRunResult struct {
	RunNo          int
	Deduped        bool
	ExistingStatus string
}

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

	runNo, err = s.createRunTx(ctx, tx, in)
	if err != nil {
		return 0, err
	}

	return runNo, tx.Commit(ctx)
}

// CreateRunOrGetActive allocates a new run unless an equivalent request is already in flight.
// It returns the oldest active run for the same request_key when deduplication matches.
func (s *Store) CreateRunOrGetActive(ctx context.Context, in CreateRunInput) (result CreateRunResult, err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return CreateRunResult{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	_, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, in.Pipeline+":"+in.RequestKey)
	if err != nil {
		return CreateRunResult{}, err
	}

	if in.RequestKey != "" {
		err = tx.QueryRow(ctx,
			`SELECT run_no, status
			 FROM pipeline_runs
			 WHERE pipeline = $1 AND request_key = $2 AND status IN ($3, $4)
			 ORDER BY run_no ASC
			 LIMIT 1`,
			in.Pipeline, in.RequestKey, StatusQueued, StatusRunning,
		).Scan(&result.RunNo, &result.ExistingStatus)
		if err == nil {
			result.Deduped = true
			return result, tx.Commit(ctx)
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return CreateRunResult{}, err
		}
		err = nil
	}

	runNo, err := s.createRunTx(ctx, tx, in)
	if err != nil {
		return CreateRunResult{}, err
	}
	result.RunNo = runNo
	result.ExistingStatus = in.Status
	return result, tx.Commit(ctx)
}

func (s *Store) createRunTx(ctx context.Context, tx pgx.Tx, in CreateRunInput) (runNo int, err error) {
	err = tx.QueryRow(ctx,
		`SELECT COALESCE(MAX(run_no), 0) + 1 FROM pipeline_runs WHERE pipeline = $1`,
		in.Pipeline,
	).Scan(&runNo)
	if err != nil {
		return 0, err
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO pipeline_runs (pipeline, run_no, start_time, status, git_hash, git_branch, git_repo, trace_id, request_key)
		 VALUES ($1, $2, $3, $4, NULLIF($5,''), NULLIF($6,''), NULLIF($7,''), NULLIF($8,''), NULLIF($9,''))`,
		in.Pipeline, runNo, in.StartTime, in.Status, in.GitHash, in.GitBranch, in.GitRepo, in.TraceID, in.RequestKey,
	)
	if err != nil {
		return 0, err
	}
	return runNo, nil
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
