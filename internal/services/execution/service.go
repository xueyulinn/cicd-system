package execution

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/CS7580-SEA-SP26/e-team/internal/common/parser"
	"github.com/CS7580-SEA-SP26/e-team/internal/common/planner"
	"github.com/CS7580-SEA-SP26/e-team/internal/models"
)

// Client handles communication with other services
type Client struct {
	workerURL string
	validationURL string
	httpClient    *http.Client
}

func NewClient() *Client {
	return &Client{
		workerURL:     getEnvOrDefault("WORKER_URL", "http://localhost:8003"),
		validationURL: getEnvOrDefault("VALIDATION_URL", "http://localhost:8001"),
		httpClient: &http.Client{
			// Allow enough time for each job (pull image, build, test); worker uses 5m per job.
			Timeout: 10 * time.Minute,
		},
	}
}

func getEnvOrDefault(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return strings.TrimRight(v, "/")
}

// Run validates the pipeline before execution.
// Actual execution can be added after validation succeeds.
func (c *Client) Run(req RunRequest) (*RunResponse, error) {
	if strings.TrimSpace(req.YAMLContent) == "" {
		return &RunResponse{
			Success: false,
			Errors:  []string{"yaml_content is required"},
		}, nil
	}

	validationResp, err := c.validatePipeline(req.YAMLContent)
	if err != nil {
		return nil, err
	}

	if !validationResp.Valid {
		return &RunResponse{
			Success: false,
			Errors:  validationResp.Errors,
		}, nil
	}

	// Parse pipeline from YAML content
	p := parser.NewParserFromContent(req.YAMLContent)
	pipeline, _, err := p.Parse()
	if err != nil {
		return &RunResponse{
			Success: false,
			Errors:  []string{fmt.Sprintf("pipeline parse failed: %v", err)},
		}, nil
	}

	// Generate execution plan for the pipeline
	executionPlan, err := planner.GenerateExecutionPlan(pipeline)
	if err != nil {
		return &RunResponse{
			Success: false,
			Errors:  []string{fmt.Sprintf("generate execution plan failed: %v", err)},
		}, nil
	}

	// Forward jobs in execution order to worker service.
	var logsByJob []string
	for _, stage := range executionPlan.Stages {
		for _, job := range stage.Jobs {
			logs, err := c.executeJob(job, req.WorkspacePath)
			if err != nil {
				return &RunResponse{
					Success: false,
					Errors:  []string{fmt.Sprintf("job %q in stage %q failed: %v", job.Name, stage.Name, err)},
				}, nil
			}

			logsByJob = append(logsByJob, fmt.Sprintf("[%s/%s]\n%s", stage.Name, job.Name, logs))
		}
	}

	// Execution finished for all jobs.
	return &RunResponse{
		Success: true,
		Message: strings.Join(logsByJob, "\n\n"),
	}, nil
}

// workerExecuteBody is the request body for worker /execute (job + optional workspace).
type workerExecuteBody struct {
	models.JobExecutionPlan
	WorkspacePath string `json:"workspace_path,omitempty"`
}

func (c *Client) executeJob(job models.JobExecutionPlan, workspacePath string) (string, error) {
	body, err := json.Marshal(workerExecuteBody{JobExecutionPlan: job, WorkspacePath: workspacePath})
	if err != nil {
		return "", fmt.Errorf("marshal worker request: %w", err)
	}

	resp, err := c.httpClient.Post(c.workerURL+"/execute", "application/json", bytes.NewBuffer(body))
	
	if err != nil {
		return "", fmt.Errorf("call worker service: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)

	if err != nil {
        return "", fmt.Errorf("read worker response: %w", err)
    }

	if resp.StatusCode != http.StatusOK {
		var e workerErrorResponse
        if json.Unmarshal(respBody, &e) == nil && e.Error != "" {
            return "", fmt.Errorf("worker returned %d: %s", resp.StatusCode, e.Error)
        }
        return "", fmt.Errorf("worker returned %d: %s", resp.StatusCode, string(respBody))
	}

	var ok workerExecuteResponse
    if err := json.Unmarshal(respBody, &ok); err != nil {
        return "", fmt.Errorf("unmarshal worker response: %w", err)
    }
    return ok.Logs, nil
}

// validatePipeline calls validation service and returns validation result.
func (c *Client) validatePipeline(yamlContent string) (*ValidationResponse, error) {
	validateReq := map[string]string{
		"yaml_content": yamlContent,
	}

	jsonBody, err := json.Marshal(validateReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal validation request: %w", err)
	}

	resp, err := c.httpClient.Post(c.validationURL+"/validate", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to call validation service: %w", err)
	}
	defer func() {
		_ = resp.Body.Close() // Ignore close error as we're done with the body
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read validation response: %w", err)
	}

	var validationResp ValidationResponse
	if err := json.Unmarshal(body, &validationResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal validation response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Keep server-provided validation details when available.
		if len(validationResp.Errors) == 0 {
			validationResp.Errors = []string{fmt.Sprintf("validation service returned status %d", resp.StatusCode)}
		}
		validationResp.Valid = false
	}

	return &validationResp, nil
}

// ValidationResponse represents validation service response.
type ValidationResponse struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

// RunResponse represents run response.
type RunResponse struct {
	Success bool     `json:"success"`
	Errors  []string `json:"errors,omitempty"`
	Message string   `json:"message,omitempty"`
}

type workerExecuteResponse struct {
    Logs string `json:"logs"`
}

type workerErrorResponse struct {
    Error string `json:"error"`
}
