package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	mysqlDriver "github.com/go-sql-driver/mysql"
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
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	runNo, err = s.createRunTx(ctx, tx, in)
	if err != nil {
		return 0, err
	}

	err = tx.Commit()
	return runNo, err
}

// CreateRunOrGetActive allocates a new run unless an equivalent request is already in flight.
// It returns the oldest active run for the same request_key when deduplication matches.
func (s *Store) CreateRunOrGetActive(ctx context.Context, in CreateRunInput) (result CreateRunResult, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return CreateRunResult{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if in.RequestKey != "" {
		err = tx.QueryRowContext(ctx,
			`SELECT run_no, status
			 FROM pipeline_runs
			 WHERE pipeline = ? AND request_key = ? AND status IN (?, ?)
			 ORDER BY run_no ASC
			 LIMIT 1`,
			in.Pipeline, in.RequestKey, StatusQueued, StatusRunning,
		).Scan(&result.RunNo, &result.ExistingStatus)
		if err == nil {
			result.Deduped = true
			err = tx.Commit()
			return result, err
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return CreateRunResult{}, err
		}
		err = nil
	}

	runNo, err := s.createRunTx(ctx, tx, in)
	if err != nil {
		if in.RequestKey != "" && isDuplicateKeyError(err) {
			err = tx.QueryRowContext(ctx,
				`SELECT run_no, status
				 FROM pipeline_runs
				 WHERE pipeline = ? AND request_key = ? AND status IN (?, ?)
				 ORDER BY run_no ASC
				 LIMIT 1`,
				in.Pipeline, in.RequestKey, StatusQueued, StatusRunning,
			).Scan(&result.RunNo, &result.ExistingStatus)
			if err != nil {
				return CreateRunResult{}, err
			}
			result.Deduped = true
			err = tx.Commit()
			return result, err
		}
		return CreateRunResult{}, err
	}
	result.RunNo = runNo
	result.ExistingStatus = in.Status
	err = tx.Commit()
	return result, err
}

func (s *Store) createRunTx(ctx context.Context, tx *sql.Tx, in CreateRunInput) (runNo int, err error) {
	_, err = tx.ExecContext(ctx,
		`INSERT INTO pipeline_sequences (pipeline, next_run_no)
		 VALUES (?, LAST_INSERT_ID(2))
		 ON DUPLICATE KEY UPDATE next_run_no = LAST_INSERT_ID(next_run_no + 1)`,
		in.Pipeline,
	)
	if err != nil {
		return 0, err
	}
	var nextRunNo int64
	err = tx.QueryRowContext(ctx, `SELECT LAST_INSERT_ID()`).Scan(&nextRunNo)
	if err != nil {
		return 0, err
	}
	if nextRunNo <= 1 {
		return 0, fmt.Errorf("failed to allocate run_no for pipeline %q", in.Pipeline)
	}
	runNo = int(nextRunNo - 1)

	_, err = tx.ExecContext(ctx,
		`INSERT INTO pipeline_runs (pipeline, run_no, start_time, status, git_hash, git_branch, git_repo, trace_id, request_key)
		 VALUES (?, ?, ?, ?, NULLIF(?,''), NULLIF(?,''), NULLIF(?,''), NULLIF(?,''), NULLIF(?,''))`,
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
		_, err := s.db.ExecContext(ctx,
			`UPDATE pipeline_runs SET end_time = ?, status = ? WHERE pipeline = ? AND run_no = ?`,
			*in.EndTime, in.Status, pipeline, runNo,
		)
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE pipeline_runs SET status = ? WHERE pipeline = ? AND run_no = ?`,
		in.Status, pipeline, runNo,
	)
	return err
}

// CreateStage inserts a new stage run.
func (s *Store) CreateStage(ctx context.Context, in CreateStageInput) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO stage_runs (pipeline, run_no, stage, start_time, status)
		 VALUES (?, ?, ?, ?, ?)`,
		in.Pipeline, in.RunNo, in.Stage, in.StartTime, in.Status,
	)
	return err
}

// UpdateStage updates end_time and status for a stage run.
func (s *Store) UpdateStage(ctx context.Context, pipeline string, runNo int, stage string, in UpdateStageInput) error {
	if in.EndTime != nil {
		_, err := s.db.ExecContext(ctx,
			`UPDATE stage_runs SET end_time = ?, status = ? WHERE pipeline = ? AND run_no = ? AND stage = ?`,
			*in.EndTime, in.Status, pipeline, runNo, stage,
		)
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE stage_runs SET status = ? WHERE pipeline = ? AND run_no = ? AND stage = ?`,
		in.Status, pipeline, runNo, stage,
	)
	return err
}

// CreateJob inserts a new job run.
func (s *Store) CreateJob(ctx context.Context, in CreateJobInput) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO job_runs (pipeline, run_no, stage, job, start_time, status, failures)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		in.Pipeline, in.RunNo, in.Stage, in.Job, in.StartTime, in.Status, in.Failures,
	)
	return err
}

// UpdateJob updates end_time and status for a job run.
func (s *Store) UpdateJob(ctx context.Context, pipeline string, runNo int, stage, job string, in UpdateJobInput) error {
	if in.EndTime != nil {
		_, err := s.db.ExecContext(ctx,
			`UPDATE job_runs SET end_time = ?, status = ? WHERE pipeline = ? AND run_no = ? AND stage = ? AND job = ?`,
			*in.EndTime, in.Status, pipeline, runNo, stage, job,
		)
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE job_runs SET status = ? WHERE pipeline = ? AND run_no = ? AND stage = ? AND job = ?`,
		in.Status, pipeline, runNo, stage, job,
	)
	return err
}

func isDuplicateKeyError(err error) bool {
	var sqlErr *mysqlDriver.MySQLError
	return errors.As(err, &sqlErr) && sqlErr.Number == 1062
}
