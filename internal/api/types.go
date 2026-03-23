// Package api defines shared HTTP request/response types for all services and the CLI.
// These types are the single source of truth for API contracts (validate, dryrun, run).
package api

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
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
	Output string   `json:"output,omitempty"`
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
	Success bool     `json:"success"`
	Errors  []string `json:"errors,omitempty"`
	Message string   `json:"message,omitempty"`
}
