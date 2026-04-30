package gateway

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
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
		reportingClient:    observability.NewInstrumentedHTTPClient("reporting", 1*time.Minute),
	}
}

func doUpstreamRequest(client *http.Client, req *http.Request, serviceName string) (*http.Response, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, wrapUpstreamCallError(serviceName, err)
	}
	return resp, nil
}

func wrapUpstreamCallError(serviceName string, err error) error {
	if isTimeoutError(err) {
		return upstreamServiceTimeout(fmt.Sprintf("%s: %v", serviceName, err))
	}
	return fmt.Errorf("failed to call %s: %w", serviceName, err)
}

func isTimeoutError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func proxyDownstreamResponse(w http.ResponseWriter, resp *http.Response) error {
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.WriteHeader(resp.StatusCode)

	if _, err := io.Copy(w, resp.Body); err != nil {
		return fmt.Errorf("copy downstream response failed: %w", err)
	}
	return nil
}

// ForwardValidate proxies validation responses from downstream directly to upstream response writer.
func (c *Client) forwardValidate(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.validationURL+"/validate", r.Body)
	if err != nil {
		return fmt.Errorf("construct request failed: %w", err)
	}

	resp, err := doUpstreamRequest(c.validationClient, req, "validation service")
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	return proxyDownstreamResponse(w, resp)
}

// ForwardDryRun proxies dryrun responses from downstream directly to upstream response writer.
func (c *Client) forwardDryRun(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.validationURL+"/dryrun", r.Body)
	if err != nil {
		return fmt.Errorf("construct request failed: %w", err)
	}

	resp, err := doUpstreamRequest(c.validationClient, req, "dryrun service")
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	return proxyDownstreamResponse(w, resp)
}

// ForwardRun forwards run request to orchestrator service.
func (c *Client) forwardRun(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.orchestratorURL+"/run", r.Body)
	if err != nil {
		return fmt.Errorf("construct request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := doUpstreamRequest(c.orchestratorClient, req, "orchestrator service")
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	return proxyDownstreamResponse(w, resp)
}

// ForwardReport forwards report request to reporting service.
func (c *Client) forwardReport(ctx context.Context, w http.ResponseWriter, query models.ReportQuery) error {
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

	endpoint := c.reportURL + "/report"
	if encoded := params.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("construct request failed: %w", err)
	}

	resp, err := doUpstreamRequest(c.reportingClient, req, "reporting service")
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	return proxyDownstreamResponse(w, resp)
}
