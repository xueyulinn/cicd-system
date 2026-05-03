package store

import (
	"context"
	"database/sql"
	"errors"
)

// ErrNotFound is returned when a queried entity does not exist.
var ErrNotFound = errors.New("not found")

// Shared column list for pipeline_runs queries (keeps SELECT and Scan in sync).
const runColumns = `pipeline, run_no, start_time, end_time, status,
	COALESCE(git_hash,''), COALESCE(git_repo,''),
	COALESCE(trace_id,''), COALESCE(request_key,'')`

func scanRun(sc interface{ Scan(dest ...any) error }) (Run, error) {
	var r Run
	err := sc.Scan(&r.Pipeline, &r.RunNo, &r.StartTime, &r.EndTime, &r.Status,
		&r.GitHash, &r.GitRepo, &r.TraceID, &r.RequestKey)
	return r, err
}

func closeRows(rows any) {
	switch r := rows.(type) {
	case interface{ Close() error }:
		_ = r.Close()
	case interface{ Close() }:
		r.Close()
	}
}

// GetRunsByPipeline returns all runs for a pipeline, ordered by run_no ascending.
func (s *Store) GetRunsByPipeline(ctx context.Context, pipeline string) ([]Run, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+runColumns+` FROM pipeline_runs WHERE pipeline = ? ORDER BY run_no ASC`,
		pipeline,
	)
	if err != nil {
		return nil, err
	}
	defer closeRows(rows)

	var runs []Run
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, r)
	}
	return runs, rows.Err()
}

// GetRun returns a single pipeline run by pipeline name and run_no. Returns ErrNotFound if not found.
func (s *Store) GetRun(ctx context.Context, pipeline string, runNo int) (*Run, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+runColumns+` FROM pipeline_runs WHERE pipeline = ? AND run_no = ?`,
		pipeline, runNo,
	)
	r, err := scanRun(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &r, nil
}

// GetStagesForRun returns all stages for a pipeline run, ordered by stage name.
// If stageFilter is non-empty, only that stage is returned (one or zero rows).
func (s *Store) GetStagesForRun(ctx context.Context, pipeline string, runNo int, stageFilter string) ([]Stage, error) {
	var rows *sql.Rows
	var err error
	if stageFilter != "" {
		rows, err = s.db.QueryContext(ctx,
			`SELECT pipeline, run_no, stage, start_time, end_time, status
			 FROM stage_runs WHERE pipeline = ? AND run_no = ? AND stage = ? ORDER BY stage`,
			pipeline, runNo, stageFilter,
		)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT pipeline, run_no, stage, start_time, end_time, status
			 FROM stage_runs WHERE pipeline = ? AND run_no = ? ORDER BY stage`,
			pipeline, runNo,
		)
	}
	if err != nil {
		return nil, err
	}
	defer closeRows(rows)

	var stages []Stage
	for rows.Next() {
		var st Stage
		err := rows.Scan(&st.Pipeline, &st.RunNo, &st.Stage, &st.StartTime, &st.EndTime, &st.Status)
		if err != nil {
			return nil, err
		}
		stages = append(stages, st)
	}
	return stages, rows.Err()
}

// GetJobsForRun returns all jobs for a pipeline run (optionally filtered by stage and/or job).
// stageFilter and jobFilter can be empty to mean "all". Results ordered by stage, then job.
func (s *Store) GetJobsForRun(ctx context.Context, pipeline string, runNo int, stageFilter, jobFilter string) ([]Job, error) {
	query := `SELECT pipeline, run_no, stage, job, start_time, end_time, status, COALESCE(failures, false) FROM job_runs WHERE pipeline = ? AND run_no = ?`
	args := []any{pipeline, runNo}
	if stageFilter != "" {
		query += " AND stage = ?"
		args = append(args, stageFilter)
	}
	if jobFilter != "" {
		query += " AND job = ?"
		args = append(args, jobFilter)
	}
	query += ` ORDER BY stage, job`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer closeRows(rows)

	var jobs []Job
	for rows.Next() {
		var j Job
		err := rows.Scan(&j.Pipeline, &j.RunNo, &j.Stage, &j.Job, &j.StartTime, &j.EndTime, &j.Status, &j.Failures)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}
