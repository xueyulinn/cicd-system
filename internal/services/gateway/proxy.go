package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/CS7580-SEA-SP26/e-team/internal/api"
	"github.com/CS7580-SEA-SP26/e-team/internal/config"
	"github.com/CS7580-SEA-SP26/e-team/internal/models"
	"github.com/CS7580-SEA-SP26/e-team/internal/observability"
)

const gatewayClientName = "api-gateway"

// Client handles communication with downstream services
type Client struct {
	validationURL string
	executionURL  string
	reportURL     string
	httpValidation *http.Client
	httpExecution  *http.Client
	httpReporting  *http.Client
}

// NewClient creates a new gateway client with trace-propagating HTTP transport and client-side latency metrics per upstream.
func NewClient() *Client {
	timeout := 15 * time.Minute
	return &Client{
		validationURL: config.GetEnvOrDefaultURL("VALIDATION_URL", config.DefaultValidationURL),
		executionURL:  config.GetEnvOrDefaultURL("EXECUTION_URL", config.DefaultExecutionURL),
		reportURL:     config.GetEnvOrDefaultURL("REPORTING_URL", config.DefaultReportingURL),
		httpValidation: observability.NewInstrumentedHTTPClient(gatewayClientName, "validation", timeout),
		httpExecution:  observability.NewInstrumentedHTTPClient(gatewayClientName, "execution", timeout),
		httpReporting:  observability.NewInstrumentedHTTPClient(gatewayClientName, "reporting", timeout),
	}
}

// ValidateRequest forwards validation request to validation service
func (c *Client) ValidateRequest(yamlContent string) (*api.ValidateResponse, error) {
	reqBody := map[string]string{
		"yaml_content": yamlContent,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpValidation.Post(c.validationURL+"/validate", "application/json", bytes.NewBuffer(jsonBody))
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
		var errorResp api.ValidateResponse
		if parseErr := json.Unmarshal(body, &errorResp); parseErr == nil && len(errorResp.Errors) > 0 {
			return nil, fmt.Errorf("validation service returned status %d: %s", resp.StatusCode, errorResp.Errors[0])
		}
		// Fallback to raw body if parsing fails
		return nil, fmt.Errorf("validation service returned status %d: %s", resp.StatusCode, string(body))
	}

	var validationResp api.ValidateResponse
	if err := json.Unmarshal(body, &validationResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &validationResp, nil
}

// DryRunRequest forwards dry run request to validation service
func (c *Client) DryRunRequest(yamlContent string) (*api.DryRunResponse, error) {
	reqBody := map[string]string{
		"yaml_content": yamlContent,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpValidation.Post(c.validationURL+"/dryrun", "application/json", bytes.NewBuffer(jsonBody))
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
		var errorResp api.DryRunResponse
		if parseErr := json.Unmarshal(body, &errorResp); parseErr == nil && len(errorResp.Errors) > 0 {
			return nil, fmt.Errorf("validation service returned status %d: %s", resp.StatusCode, errorResp.Errors[0])
		}
		// Fallback to raw body if parsing fails
		return nil, fmt.Errorf("validation service returned status %d: %s", resp.StatusCode, string(body))
	}

	var dryRunResp api.DryRunResponse
	if err := json.Unmarshal(body, &dryRunResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &dryRunResp, nil
}

// RunRequest forwards run request to execution service
func (c *Client) RunRequest(req api.RunRequest) (*api.RunResponse, error) {
	jsonBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpExecution.Post(c.executionURL+"/run", "application/json", bytes.NewBuffer(jsonBody))
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
		var errorResp api.RunResponse
		if parseErr := json.Unmarshal(body, &errorResp); parseErr == nil && len(errorResp.Errors) > 0 {
			return nil, fmt.Errorf("execution service returned status %d: %s", resp.StatusCode, errorResp.Errors[0])
		}
		// Fallback to raw body if parsing fails
		return nil, fmt.Errorf("execution service returned status %d: %s", resp.StatusCode, string(body))
	}

	var runResp api.RunResponse
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

	resp, err := c.httpReporting.Get(c.reportURL + "/report?" + params.Encode())
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
