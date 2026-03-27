package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

var ErrNotFound = errors.New("not found")

// GetRunsByPipeline returns all runs for a pipeline, ordered by run_no ascending.
func (s *Store) GetRunsByPipeline(ctx context.Context, pipeline string) ([]Run, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT pipeline, run_no, start_time, end_time, status, COALESCE(git_hash,''), COALESCE(git_branch,''), COALESCE(git_repo,''), COALESCE(trace_id,'')
		 FROM pipeline_runs WHERE pipeline = $1 ORDER BY run_no ASC`,
		pipeline,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []Run
	for rows.Next() {
		var r Run
		err := rows.Scan(&r.Pipeline, &r.RunNo, &r.StartTime, &r.EndTime, &r.Status, &r.GitHash, &r.GitBranch, &r.GitRepo, &r.TraceID)
		if err != nil {
			return nil, err
		}
		runs = append(runs, r)
	}
	return runs, rows.Err()
}

// GetRun returns a single pipeline run by pipeline name and run_no. Returns ErrNotFound if not found.
func (s *Store) GetRun(ctx context.Context, pipeline string, runNo int) (*Run, error) {
	var r Run
	err := s.pool.QueryRow(ctx,
		`SELECT pipeline, run_no, start_time, end_time, status, COALESCE(git_hash,''), COALESCE(git_branch,''), COALESCE(git_repo,''), COALESCE(trace_id,'')
		 FROM pipeline_runs WHERE pipeline = $1 AND run_no = $2`,
		pipeline, runNo,
	).Scan(&r.Pipeline, &r.RunNo, &r.StartTime, &r.EndTime, &r.Status, &r.GitHash, &r.GitBranch, &r.GitRepo, &r.TraceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &r, nil
}

// GetStagesForRun returns all stages for a pipeline run, ordered by stage name.
// If stageFilter is non-empty, only that stage is returned (one or zero rows).
func (s *Store) GetStagesForRun(ctx context.Context, pipeline string, runNo int, stageFilter string) ([]Stage, error) {
	var rows pgx.Rows
	var err error
	if stageFilter != "" {
		rows, err = s.pool.Query(ctx,
			`SELECT pipeline, run_no, stage, start_time, end_time, status
			 FROM stage_runs WHERE pipeline = $1 AND run_no = $2 AND stage = $3 ORDER BY stage`,
			pipeline, runNo, stageFilter,
		)
	} else {
		rows, err = s.pool.Query(ctx,
			`SELECT pipeline, run_no, stage, start_time, end_time, status
			 FROM stage_runs WHERE pipeline = $1 AND run_no = $2 ORDER BY stage`,
			pipeline, runNo,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

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
	query := `SELECT pipeline, run_no, stage, job, start_time, end_time, status, COALESCE(failures, false) FROM job_runs WHERE pipeline = $1 AND run_no = $2`
	args := []interface{}{pipeline, runNo}
	pos := 3
	if stageFilter != "" {
		query += fmt.Sprintf(" AND stage = $%d", pos)
		args = append(args, stageFilter)
		pos++
	}
	if jobFilter != "" {
		query += fmt.Sprintf(" AND job = $%d", pos)
		args = append(args, jobFilter)
	}
	query += ` ORDER BY stage, job`

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

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
