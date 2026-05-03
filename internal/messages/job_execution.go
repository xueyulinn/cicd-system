package messages

import "github.com/xueyulinn/cicd-system/internal/models"

// JobExecutionMessage is the MQ payload published by execution service and
// consumed by worker service for a single job run.
type JobExecutionMessage struct {
	RunNo               int                     `json:"run_no"`
	PipelineName        string                  `json:"pipeline"`
	Stage               string                  `json:"stage"`
	Commit              string                  `json:"commit,omitempty"`
	RepoURL             string                  `json:"repo_url,omitempty"`
	WorkspaceObjectName string                  `json:"workspace_object,omitempty"`
	Job                 models.JobExecutionPlan `json:"job"`
}
