package store

import "time"

// Status values for pipeline run, stage, and job.
const (
	StatusRunning = "running"
	StatusSuccess = "success"
	StatusFailed  = "failed"
)

// Run represents a pipeline run (one row in pipeline_runs).
type Run struct {
	Pipeline   string     `json:"pipeline"`
	RunNo      int        `json:"run_no"`
	StartTime  time.Time  `json:"start_time"`
	EndTime    *time.Time `json:"end_time,omitempty"`
	Status     string     `json:"status"`
	GitHash    string     `json:"git_hash,omitempty"`
	GitBranch  string     `json:"git_branch,omitempty"`
	GitRepo    string     `json:"git_repo,omitempty"`
}

// Stage represents a stage run (one row in stage_runs).
type Stage struct {
	Pipeline   string     `json:"pipeline"`
	RunNo      int        `json:"run_no"`
	Stage      string     `json:"stage"`
	StartTime  time.Time  `json:"start_time"`
	EndTime    *time.Time `json:"end_time,omitempty"`
	Status     string     `json:"status"`
}

// Job represents a job run (one row in job_runs).
type Job struct {
	Pipeline   string     `json:"pipeline"`
	RunNo      int        `json:"run_no"`
	Stage      string     `json:"stage"`
	Job        string     `json:"job"`
	StartTime  time.Time  `json:"start_time"`
	EndTime    *time.Time `json:"end_time,omitempty"`
	Status     string     `json:"status"`
	Failures   bool       `json:"failures"` // when true, job is allowed to fail and does not affect stage status
}

// CreateRunInput is the input for CreateRun (run_no is allocated by the store).
type CreateRunInput struct {
	Pipeline  string
	StartTime time.Time
	Status    string // typically StatusRunning
	GitHash   string
	GitBranch string
	GitRepo   string
}

// UpdateRunInput is the input for UpdateRun (only non-zero/non-nil fields are updated).
type UpdateRunInput struct {
	EndTime *time.Time
	Status  string
}

// CreateStageInput is the input for CreateStage.
type CreateStageInput struct {
	Pipeline   string
	RunNo      int
	Stage      string
	StartTime  time.Time
	Status     string // typically StatusRunning
}

// UpdateStageInput is the input for UpdateStage.
type UpdateStageInput struct {
	EndTime *time.Time
	Status  string
}

// CreateJobInput is the input for CreateJob.
type CreateJobInput struct {
	Pipeline   string
	RunNo      int
	Stage      string
	Job        string
	StartTime  time.Time
	Status     string // typically StatusRunning
	Failures   bool   // when true, job is allowed to fail; default false
}

// UpdateJobInput is the input for UpdateJob.
type UpdateJobInput struct {
	EndTime *time.Time
	Status  string
}
