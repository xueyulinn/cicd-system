package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/xueyulinn/cicd-system/internal/api"
	"github.com/xueyulinn/cicd-system/internal/config"
	"github.com/xueyulinn/cicd-system/internal/models"
	"github.com/xueyulinn/cicd-system/internal/observability"
)

const gatewayClientName = "api-gateway"

// Client handles communication with downstream services
type Client struct {
	validationURL  string
	executionURL   string
	reportURL      string
	httpValidation *http.Client
	httpExecution  *http.Client
	httpReporting  *http.Client
}

// NewClient creates a new gateway client with trace-propagating HTTP transport and client-side latency metrics per upstream.
func NewClient() *Client {
	return &Client{
		validationURL:  config.GetEnvOrDefaultURL("VALIDATION_URL", config.DefaultValidationURL),
		executionURL:   config.GetEnvOrDefaultURL("EXECUTION_URL", config.DefaultExecutionURL),
		reportURL:      config.GetEnvOrDefaultURL("REPORTING_URL", config.DefaultReportingURL),
		httpValidation: observability.NewInstrumentedHTTPClient(gatewayClientName, "validation", 2*time.Minute),
		httpExecution:  observability.NewInstrumentedHTTPClient(gatewayClientName, "execution", 2*time.Minute),
		httpReporting:  observability.NewInstrumentedHTTPClient(gatewayClientName, "reporting", 2*time.Minute),
	}
}

// ValidateRequest forwards validation request to validation service and decodes the JSON response.
func (c *Client) ValidateRequest(ctx context.Context, yamlContent string) (*api.ValidateResponse, error) {
	reqBody, err := json.Marshal(map[string]string{"yaml_content": yamlContent})
	if err != nil {
		return nil, fmt.Errorf("marshal validation request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.validationURL+"/validate", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("construct request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpValidation.Do(req)
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

// ForwardValidate proxies validation responses from downstream directly to upstream response writer.
func (c *Client) ForwardValidate(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.validationURL+"/validate", r.Body)
	if err != nil {
		return fmt.Errorf("construct request failed: %w", err)
	}

	resp, err := c.httpValidation.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call validation service: %w", err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.WriteHeader(resp.StatusCode)

	if _, err := io.Copy(w, resp.Body); err != nil {
		return fmt.Errorf("copy downstream response failed: %w", err)
	}
	return nil
}

// DryRunRequest forwards dry run request to validation service and decodes the JSON response.
func (c *Client) DryRunRequest(ctx context.Context, yamlContent string) (*api.DryRunResponse, error) {
	reqBody, err := json.Marshal(map[string]string{"yaml_content": yamlContent})
	if err != nil {
		return nil, fmt.Errorf("marshal dryrun request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.validationURL+"/dryrun", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("construct request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpValidation.Do(req)
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

	var out api.DryRunResponse
	if err := json.Unmarshal(body, &out); err != nil {
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("validation service returned status %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if len(out.Errors) > 0 {
			return nil, fmt.Errorf("validation service returned status %d: %s", resp.StatusCode, out.Errors[0])
		}
		return nil, fmt.Errorf("validation service returned status %d: %s", resp.StatusCode, string(body))
	}

	return &out, nil
}

// ForwardDryRun proxies dryrun responses from downstream directly to upstream response writer.
func (c *Client) ForwardDryRun(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.validationURL+"/dryrun", r.Body)
	if err != nil {
		return fmt.Errorf("construct request failed: %w", err)
	}

	resp, err := c.httpValidation.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call dryrun service: %w", err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.WriteHeader(resp.StatusCode)

	if _, err := io.Copy(w, resp.Body); err != nil {
		return fmt.Errorf("copy downstream response failed: %w", err)
	}
	return nil
}

// RunRequest forwards run request to execution service.
func (c *Client) RunRequest(ctx context.Context, req api.RunRequest) (*api.RunResponse, error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.executionURL+"/run", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("construct request failed: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpExecution.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call execution service: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var out api.RunResponse
	if err := json.Unmarshal(body, &out); err != nil {
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("execution service returned status %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if len(out.Errors) > 0 {
			return nil, fmt.Errorf("execution service returned status %d: %s", resp.StatusCode, out.Errors[0])
		}
		return nil, fmt.Errorf("execution service returned status %d: %s", resp.StatusCode, string(body))
	}

	return &out, nil
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
