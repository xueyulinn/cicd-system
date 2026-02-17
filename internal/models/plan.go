package models

// ExecutionPlan represents the execution order of stages and jobs (reusable across CLI and services).
type ExecutionPlan struct {
	Stages []StageExecutionPlan `json:"stages" yaml:"stages"`
}

// StageExecutionPlan represents a stage in execution order with its jobs.
type StageExecutionPlan struct {
	Name string             `json:"name" yaml:"name"`
	Jobs []JobExecutionPlan `json:"jobs" yaml:"jobs"`
}

// JobExecutionPlan represents a job in execution order with image and script.
type JobExecutionPlan struct {
	Name   string   `json:"name" yaml:"name"`
	Image  string   `json:"image,omitempty" yaml:"image,omitempty"`
	Script []string `json:"script,omitempty" yaml:"script,omitempty"`
}
