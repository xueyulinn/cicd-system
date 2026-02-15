package execution

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client handles communication with other services
type Client struct {
	validationURL string
	httpClient    *http.Client
}

func NewClient() *Client{
	return &Client{
		validationURL: "http://localhost:8001",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
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

	// Placeholder response until execution logic is implemented.
	return &RunResponse{
		Success: true,
		Message: "validation passed",
	}, nil
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
