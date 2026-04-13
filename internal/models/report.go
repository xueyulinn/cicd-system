package models

import "time"

// ReportQuery captures supported report filters.
type ReportQuery struct {
	Pipeline string `json:"pipeline"`
	Run      *int   `json:"run,omitempty"`
	Stage    string `json:"stage,omitempty"`
	Job      string `json:"job,omitempty"`
}

// ReportResponse is the API payload returned by reporting endpoints.
type ReportResponse struct {
	Pipeline ReportPipeline `json:"pipeline" yaml:"pipeline"`
}

// ReportPipeline contains pipeline-level and scoped run/stage/job report fields.
// Stages is used for full run-stage listings, while Stage preserves the existing
// single-stage response shape used by current clients.
type ReportPipeline struct {
	Name    string      `json:"name" yaml:"name"`
	Runs    []ReportRun `json:"runs,omitempty" yaml:"runs,omitempty"`
	RunNo   int         `json:"run-no,omitempty" yaml:"run-no,omitempty"`
	Status  string      `json:"status,omitempty" yaml:"status,omitempty"`
	TraceID string      `json:"trace-id,omitempty" yaml:"trace-id,omitempty"`
	Start   time.Time   `json:"start,omitempty" yaml:"start,omitempty"`
	End     *time.Time  `json:"end,omitempty" yaml:"end,omitempty"`
	Stages  []ReportStage `json:"stages,omitempty" yaml:"stages,omitempty"`
	Stage   []ReportStage `json:"stage,omitempty" yaml:"stage,omitempty"`
}

// ReportRun is a run summary for the "all runs for a pipeline" report.
type ReportRun struct {
	RunNo     int        `json:"run-no" yaml:"run-no"`
	Status    string     `json:"status" yaml:"status"`
	TraceID   string     `json:"trace-id,omitempty" yaml:"trace-id,omitempty"`
	GitRepo   string     `json:"git-repo,omitempty" yaml:"git-repo,omitempty"`
	GitBranch string     `json:"git-branch,omitempty" yaml:"git-branch,omitempty"`
	GitHash   string     `json:"git-hash,omitempty" yaml:"git-hash,omitempty"`
	Start     time.Time  `json:"start" yaml:"start"`
	End       *time.Time `json:"end,omitempty" yaml:"end,omitempty"`
}

// ReportStage is a stage report block.
// Jobs is used for full stage-job listings, while Job preserves the existing
// single-job response shape used by current clients.
type ReportStage struct {
	Name   string      `json:"name" yaml:"name"`
	Status string      `json:"status" yaml:"status"`
	Start  time.Time   `json:"start" yaml:"start"`
	End    *time.Time  `json:"end,omitempty" yaml:"end,omitempty"`
	Jobs   []ReportJob `json:"jobs,omitempty" yaml:"jobs,omitempty"`
	Job    []ReportJob `json:"job,omitempty" yaml:"job,omitempty"`
}

// ReportJob is a job report block. Failures is always present in output (true = allowed to fail).
type ReportJob struct {
	Name     string     `json:"name" yaml:"name"`
	Status   string     `json:"status" yaml:"status"`
	Start    time.Time  `json:"start" yaml:"start"`
	End      *time.Time `json:"end,omitempty" yaml:"end,omitempty"`
	Failures bool       `json:"failures" yaml:"failures"`
}
