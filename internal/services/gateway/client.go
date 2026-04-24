package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/xueyulinn/cicd-system/internal/config"
	"github.com/xueyulinn/cicd-system/internal/models"
	"github.com/xueyulinn/cicd-system/internal/observability"
)

// Client handles communication with downstream services
type Client struct {
	validationURL      string
	orchestratorURL    string
	reportURL          string
	validationClient   *http.Client
	orchestratorClient *http.Client
	reportingClient    *http.Client
}

// NewClient creates a new gateway client with trace-propagating HTTP transport and client-side latency metrics per upstream.
func NewClient() *Client {
	return &Client{
		validationURL:      config.GetEnvOrDefaultURL("VALIDATION_URL", config.DefaultValidationURL),
		orchestratorURL:    config.GetEnvOrDefaultURL("ORCHESTRATOR_URL", config.DefaultOrchestratorURL),
		reportURL:          config.GetEnvOrDefaultURL("REPORTING_URL", config.DefaultReportingURL),
		validationClient:   observability.NewInstrumentedHTTPClient("validation", 2*time.Minute),
		orchestratorClient: observability.NewInstrumentedHTTPClient("orchestrator", 2*time.Minute),
		reportingClient:    observability.NewInstrumentedHTTPClient("reporting", 2*time.Minute),
	}
}

// ForwardValidate proxies validation responses from downstream directly to upstream response writer.
func (c *Client) forwardValidate(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.validationURL+"/validate", r.Body)
	if err != nil {
		return fmt.Errorf("construct request failed: %w", err)
	}

	resp, err := c.validationClient.Do(req)
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

// ForwardDryRun proxies dryrun responses from downstream directly to upstream response writer.
func (c *Client) forwardDryRun(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.validationURL+"/dryrun", r.Body)
	if err != nil {
		return fmt.Errorf("construct request failed: %w", err)
	}

	resp, err := c.validationClient.Do(req)
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

// RunRequest forwards run request to orchestrator service.
func (c *Client) forwardRun(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.orchestratorURL+"/run", r.Body)
	if err != nil {
		return fmt.Errorf("construct request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := c.orchestratorClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call orchestrator service: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.WriteHeader(resp.StatusCode)

	if _, err := io.Copy(w, resp.Body); err != nil {
		return fmt.Errorf("copy downstream response failed: %w", err)
	}
	return nil
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

	resp, err := c.reportingClient.Get(c.reportURL + "/report?" + params.Encode())
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
