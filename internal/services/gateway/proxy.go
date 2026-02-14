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
	httpClient    *http.Client
}

// NewClient creates a new gateway client
func NewClient() *Client {
	return &Client{
		validationURL: "http://localhost:8001",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
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
	defer resp.Body.Close()

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
	defer resp.Body.Close()

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
