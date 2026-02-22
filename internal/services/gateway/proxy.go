package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client handles communication with downstream services
type Client struct {
	validationURL string
	executionURL  string
	httpClient    *http.Client
}

// NewClient creates a new gateway client
func NewClient() *Client {
	return &Client{
		validationURL: "http://localhost:8001",
		executionURL:  "http://localhost:8002",
		httpClient: &http.Client{
			Timeout: 15 * time.Minute, // Pipeline execution can take several minutes
		},
	}
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
