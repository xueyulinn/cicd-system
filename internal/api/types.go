// Package api defines shared HTTP request/response types for all services and the CLI.
// These types are the single source of truth for API contracts (validate, dryrun, run).
package api

import "github.com/xueyulinn/cicd-system/internal/models"

// StatusResponse is the common response shape for simple status endpoints
// such as /health and /ready.
type StatusResponse struct {
	Status string `json:"status"`
}

// ValidateRequest is the request body for POST /validate and POST /dryrun.
type ValidateRequest struct {
	YAMLContent string `json:"yaml_content"`
}

// ValidateResponse is the response for POST /validate.
type ValidateResponse struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

// DryRunResponse is the response for POST /dryrun.
type DryRunResponse struct {
	Valid         bool                  `json:"valid"`
	Errors        []string              `json:"errors,omitempty"`
	ExecutionPlan *models.ExecutionPlan `json:"execution_plan,omitempty"`
}

// RunRequest is the request body for POST /run.
type RunRequest struct {
	YAMLContent   string `json:"yaml_content"`
	Branch        string `json:"branch"`
	Commit        string `json:"commit"`
	RepoURL       string `json:"repo_url,omitempty"`
	WorkspacePath string `json:"workspace_path,omitempty"`
}

// RunResponse is the response for POST /run.
type RunResponse struct {
	Pipeline string   `json:"pipeline,omitempty"`
	RunNo    int      `json:"run_no,omitempty"`
	Status   string   `json:"status"`
	Errors   []string `json:"errors,omitempty"`
	Message  string   `json:"message,omitempty"`
}

// JobStatusCallbackRequest is sent by worker service to execution service to
// report job lifecycle transitions.
type JobStatusCallbackRequest struct {
	Pipeline string `json:"pipeline"`
	RunNo    int    `json:"run_no"`
	Stage    string `json:"stage"`
	Job      string `json:"job"`
	Status   string `json:"status"`
	Logs     string `json:"logs,omitempty"`
	Error    string `json:"error,omitempty"`
}
