package messages

import "github.com/CS7580-SEA-SP26/e-team/internal/models"

// JobExecutionMessage is the MQ payload published by execution service and
// consumed by worker service for a single job run.
type JobExecutionMessage struct {
	RunNo         int                     `json:"run_no"`
	Pipeline      string                  `json:"pipeline"`
	Stage         string                  `json:"stage"`
	Branch        string                  `json:"branch,omitempty"`
	Commit        string                  `json:"commit,omitempty"`
	WorkspacePath string                  `json:"workspace_path,omitempty"`
	Job           models.JobExecutionPlan `json:"job"`
}