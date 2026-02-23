package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/CS7580-SEA-SP26/e-team/internal/models"
)

// Client handles communication with downstream services
type Client struct {
	validationURL string
	executionURL  string
	reportURL     string
	httpClient    *http.Client
}

// NewClient creates a new gateway client
func NewClient() *Client {
	return &Client{
		validationURL: getEnvOrDefault("VALIDATION_URL", "http://localhost:8001"),
		executionURL:  getEnvOrDefault("EXECUTION_URL", "http://localhost:8002"),
		reportURL:     getEnvOrDefault("REPORTING_URL", "http://localhost:8004"),
		httpClient: &http.Client{
			Timeout: 15 * time.Minute, // Pipeline execution can take several minutes
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

// ValidateRequest forwards validation request to validation service
func (c *Client) ValidateRequest(yamlContent string) (*ValidationResponse, error) {
	reqBody := map[string]string{
		"yaml_content": yamlContent,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
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
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Try to parse the error response first
		var errorResp ValidationResponse
		if parseErr := json.Unmarshal(body, &errorResp); parseErr == nil && len(errorResp.Errors) > 0 {
			return nil, fmt.Errorf("validation service returned status %d: %s", resp.StatusCode, errorResp.Errors[0])
		}
		// Fallback to raw body if parsing fails
		return nil, fmt.Errorf("validation service returned status %d: %s", resp.StatusCode, string(body))
	}

	var validationResp ValidationResponse
	if err := json.Unmarshal(body, &validationResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &validationResp, nil
}

// DryRunRequest forwards dry run request to validation service
func (c *Client) DryRunRequest(yamlContent string) (*DryRunResponse, error) {
	reqBody := map[string]string{
		"yaml_content": yamlContent,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpClient.Post(c.validationURL+"/dryrun", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to call validation service: %w", err)
	}
	defer func() {
		_ = resp.Body.Close() // Ignore close error as we're done with the body
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Try to parse the error response first
		var errorResp DryRunResponse
		if parseErr := json.Unmarshal(body, &errorResp); parseErr == nil && len(errorResp.Errors) > 0 {
			return nil, fmt.Errorf("validation service returned status %d: %s", resp.StatusCode, errorResp.Errors[0])
		}
		// Fallback to raw body if parsing fails
		return nil, fmt.Errorf("validation service returned status %d: %s", resp.StatusCode, string(body))
	}

	var dryRunResp DryRunResponse
	if err := json.Unmarshal(body, &dryRunResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &dryRunResp, nil
}

// RunRequest represents execution service request
type RunRequest struct {
	YAMLContent   string `json:"yaml_content"`
	Branch        string `json:"branch"`
	Commit        string `json:"commit"`
	WorkspacePath string `json:"workspace_path,omitempty"`
}

// RunResponse represents execution service response
type RunResponse struct {
	Success bool     `json:"success"`
	Errors  []string `json:"errors,omitempty"`
	Message string   `json:"message,omitempty"`
}

// RunRequest forwards run request to execution service
func (c *Client) RunRequest(req RunRequest) (*RunResponse, error) {
	jsonBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpClient.Post(c.executionURL+"/run", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to call execution service: %w", err)
	}
	defer func() {
		_ = resp.Body.Close() // Ignore close error as we're done with the body
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Try to parse error response first
		var errorResp RunResponse
		if parseErr := json.Unmarshal(body, &errorResp); parseErr == nil && len(errorResp.Errors) > 0 {
			return nil, fmt.Errorf("execution service returned status %d: %s", resp.StatusCode, errorResp.Errors[0])
		}
		// Fallback to raw body if parsing fails
		return nil, fmt.Errorf("execution service returned status %d: %s", resp.StatusCode, string(body))
	}

	var runResp RunResponse
	if err := json.Unmarshal(body, &runResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &runResp, nil
}

// ReportRequest forwards report request to reporting service.
func (c *Client) ReportRequest(query models.ReportQuery) (*models.ReportResponse, int, error) {
	params := url.Values{}
	params.Set("pipeline", query.Pipeline)
	if query.Run != nil {
		params.Set("run", strconv.Itoa(*query.Run))
	}
	if query.Stage != "" {
		params.Set("stage", query.Stage)
	}
	if query.Job != "" {
		params.Set("job", query.Job)
	}

	resp, err := c.httpClient.Get(c.reportURL + "/report?" + params.Encode())
	if err != nil {
		return nil, http.StatusBadGateway, fmt.Errorf("failed to call reporting service: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, http.StatusBadGateway, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResp map[string]string
		if parseErr := json.Unmarshal(body, &errorResp); parseErr == nil && errorResp["error"] != "" {
			return nil, resp.StatusCode, fmt.Errorf("%s", errorResp["error"])
		}
		return nil, resp.StatusCode, fmt.Errorf("reporting service returned status %d: %s", resp.StatusCode, string(body))
	}

	var out models.ReportResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, http.StatusBadGateway, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &out, http.StatusOK, nil
}

// ValidationResponse represents validation service response
type ValidationResponse struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

// DryRunResponse represents dry run service response
type DryRunResponse struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
	Output string   `json:"output,omitempty"`
}
