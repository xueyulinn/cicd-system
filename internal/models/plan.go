package models

// ExecutionPlan represents the stage/job execution order for a pipeline run.
type ExecutionPlan struct {
	Stages []StageExecutionPlan `json:"stages" yaml:"stages"`
}

// StageExecutionPlan represents a stage in execution order with its jobs.
type StageExecutionPlan struct {
	Name string             `json:"name" yaml:"name"`
	Jobs []JobExecutionPlan `json:"jobs" yaml:"jobs"`
}

// StagePlan captures the static dependency graph for a single stage.
// Execution services can use it to decide which jobs are initially ready
// and which downstream jobs are released after a dependency succeeds.
type StagePlan struct {
	Name       string                      `json:"name" yaml:"name"`
	Jobs       []JobExecutionPlan          `json:"jobs" yaml:"jobs"`
	Needs      map[string][]string         `json:"needs" yaml:"needs"`
	Dependents map[string][]string         `json:"dependents" yaml:"dependents"`
	InDegree   map[string]int              `json:"in_degree" yaml:"in_degree"`
	JobByName  map[string]JobExecutionPlan `json:"-" yaml:"-"`
}

// JobExecutionPlan represents one executable job in a stage plan or run plan.
type JobExecutionPlan struct {
	Name   string   `json:"name" yaml:"name"`
	Image  string   `json:"image,omitempty" yaml:"image,omitempty"`
	Script []string `json:"script,omitempty" yaml:"script,omitempty"`
}
