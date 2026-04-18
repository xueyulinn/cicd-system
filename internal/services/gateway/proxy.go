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

func postJSON[T any](client *http.Client, endpoint string, reqBody any, errorPrefix string) (*T, error) {
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := client.Post(endpoint, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to call %s: %w", errorPrefix, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var typedErr T
		if parseErr := json.Unmarshal(body, &typedErr); parseErr == nil {
			switch v := any(typedErr).(type) {
			case api.ValidateResponse:
				if len(v.Errors) > 0 {
					return nil, fmt.Errorf("%s returned status %d: %s", errorPrefix, resp.StatusCode, v.Errors[0])
				}
			case api.DryRunResponse:
				if len(v.Errors) > 0 {
					return nil, fmt.Errorf("%s returned status %d: %s", errorPrefix, resp.StatusCode, v.Errors[0])
				}
			case api.RunResponse:
				if len(v.Errors) > 0 {
					return nil, fmt.Errorf("%s returned status %d: %s", errorPrefix, resp.StatusCode, v.Errors[0])
				}
			}
		}
		return nil, fmt.Errorf("%s returned status %d: %s", errorPrefix, resp.StatusCode, string(body))
	}

	var out T
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &out, nil
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
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var out api.ValidateResponse
	if err := json.Unmarshal(body, &out); err != nil {
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("validation service returned status %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Validation errors are business outcomes, not proxy failures.
	// Some validation service deployments return 400 + structured error payload.
	if resp.StatusCode == http.StatusBadRequest {
		out.Valid = false
		return &out, nil
	}
	if resp.StatusCode != http.StatusOK {
		if len(out.Errors) > 0 {
			return nil, fmt.Errorf("validation service returned status %d: %s", resp.StatusCode, out.Errors[0])
		}
		return nil, fmt.Errorf("validation service returned status %d: %s", resp.StatusCode, string(body))
	}

	return &out, nil
}

// DryRunRequest forwards dry run request to validation service
func (c *Client) DryRunRequest(yamlContent string) (*api.DryRunResponse, error) {
	return postJSON[api.DryRunResponse](c.httpValidation, c.validationURL+"/dryrun", map[string]string{
		"yaml_content": yamlContent,
	}, "validation service")
}

// RunRequest forwards run request to execution service
func (c *Client) RunRequest(req api.RunRequest) (*api.RunResponse, error) {
	return postJSON[api.RunResponse](c.httpExecution, c.executionURL+"/run", req, "execution service")
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
