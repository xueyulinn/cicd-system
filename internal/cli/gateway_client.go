package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// GatewayClient handles communication with API gateway
type GatewayClient struct {
	baseURL    string
	httpClient  *http.Client
}

// NewGatewayClient creates a new gateway client
func NewGatewayClient() *GatewayClient {
	gatewayURL := os.Getenv("GATEWAY_URL")
	if gatewayURL == "" {
		gatewayURL = "http://localhost:8000"
	}

	return &GatewayClient{
		baseURL: gatewayURL,
		httpClient: &http.Client{
			Timeout: 15 * time.Minute, // Pipeline execution can take several minutes
		},
	}
}

// Validate sends validation request to gateway
func (c *GatewayClient) Validate(yamlContent string) (*ValidationResponse, error) {
	reqBody := map[string]string{
		"yaml_content": yamlContent,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpClient.Post(c.baseURL+"/validate", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to call gateway: %w", err)
	}
	defer func() {
		_ = resp.Body.Close() // Ignore close error as we're done with the body
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gateway returned status %d: %s", resp.StatusCode, string(body))
	}

	var validationResp ValidationResponse
	if err := json.Unmarshal(body, &validationResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &validationResp, nil
}

// DryRun sends dry run request to gateway
func (c *GatewayClient) DryRun(yamlContent string) (*DryRunResponse, error) {
	reqBody := map[string]string{
		"yaml_content": yamlContent,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpClient.Post(c.baseURL+"/dryrun", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to call gateway: %w", err)
	}
	defer func() {
		_ = resp.Body.Close() // Ignore close error as we're done with the body
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gateway returned status %d: %s", resp.StatusCode, string(body))
	}

	var dryRunResp DryRunResponse
	if err := json.Unmarshal(body, &dryRunResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &dryRunResp, nil
}

// Run sends run request to gateway
func (c *GatewayClient) Run(req RunRequest) (*RunResponse, error) {
	jsonBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpClient.Post(c.baseURL+"/run", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to call gateway: %w", err)
	}
	defer func() {
		_ = resp.Body.Close() // Ignore close error as we're done with the body
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gateway returned status %d: %s", resp.StatusCode, string(body))
	}

	var runResp RunResponse
	if err := json.Unmarshal(body, &runResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &runResp, nil
}

// ValidationResponse represents gateway validation response
type ValidationResponse struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

// DryRunResponse represents gateway dry run response
type DryRunResponse struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
	Output string   `json:"output,omitempty"`
}

// RunRequest represents gateway run request
type RunRequest struct {
	YAMLContent   string `json:"yaml_content"`
	Branch        string `json:"branch"`
	Commit        string `json:"commit"`
	WorkspacePath string `json:"workspace_path,omitempty"`
}

// RunResponse represents gateway run response
type RunResponse struct {
	Success bool     `json:"success"`
	Errors  []string `json:"errors,omitempty"`
	Message string   `json:"message,omitempty"`
}
