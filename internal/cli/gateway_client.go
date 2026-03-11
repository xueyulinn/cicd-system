package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/CS7580-SEA-SP26/e-team/internal/api"
	"github.com/CS7580-SEA-SP26/e-team/internal/config"
	"github.com/CS7580-SEA-SP26/e-team/internal/models"
)

// GatewayClient handles communication with API gateway
type GatewayClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewGatewayClient creates a new gateway client
func NewGatewayClient() *GatewayClient {
	gatewayURL := os.Getenv("GATEWAY_URL")
	if gatewayURL == "" {
		gatewayURL = config.DefaultGatewayURL
	}

	return &GatewayClient{
		baseURL: gatewayURL,
		httpClient: &http.Client{
			Timeout: 15 * time.Minute, // Pipeline execution can take several minutes
		},
	}
}

// Validate sends validation request to gateway
func (c *GatewayClient) Validate(yamlContent string) (*api.ValidateResponse, error) {
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
		var errorResp map[string]string
		if parseErr := json.Unmarshal(body, &errorResp); parseErr == nil && errorResp["error"] != "" {
			return nil, fmt.Errorf("%s", errorResp["error"])
		}
		return nil, fmt.Errorf("gateway returned status %d: %s", resp.StatusCode, string(body))
	}

	var validationResp api.ValidateResponse
	if err := json.Unmarshal(body, &validationResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &validationResp, nil
}

// DryRun sends dry run request to gateway
func (c *GatewayClient) DryRun(yamlContent string) (*api.DryRunResponse, error) {
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
		var errorResp map[string]string
		if parseErr := json.Unmarshal(body, &errorResp); parseErr == nil && errorResp["error"] != "" {
			return nil, fmt.Errorf("%s", errorResp["error"])
		}
		return nil, fmt.Errorf("gateway returned status %d: %s", resp.StatusCode, string(body))
	}

	var dryRunResp api.DryRunResponse
	if err := json.Unmarshal(body, &dryRunResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &dryRunResp, nil
}

// Run sends run request to gateway
func (c *GatewayClient) Run(req api.RunRequest) (*api.RunResponse, error) {
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
		var errorResp map[string]string
		if parseErr := json.Unmarshal(body, &errorResp); parseErr == nil && errorResp["error"] != "" {
			return nil, fmt.Errorf("%s", errorResp["error"])
		}
		return nil, fmt.Errorf("gateway returned status %d: %s", resp.StatusCode, string(body))
	}

	var runResp api.RunResponse
	if err := json.Unmarshal(body, &runResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &runResp, nil
}

// Report sends report request to gateway.
func (c *GatewayClient) Report(query models.ReportQuery) (*models.ReportResponse, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/report", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	params := req.URL.Query()
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
	req.URL.RawQuery = params.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call gateway: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResp map[string]string
		if parseErr := json.Unmarshal(body, &errorResp); parseErr == nil && errorResp["error"] != "" {
			return nil, fmt.Errorf("%s", errorResp["error"])
		}
		return nil, fmt.Errorf("gateway returned status %d: %s", resp.StatusCode, string(body))
	}

	var reportResp models.ReportResponse
	if err := json.Unmarshal(body, &reportResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &reportResp, nil
}
