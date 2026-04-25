package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/xueyulinn/cicd-system/internal/api"
	"github.com/xueyulinn/cicd-system/internal/config"
	"github.com/xueyulinn/cicd-system/internal/models"
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
			Timeout: 5 * time.Minute,
		},
	}
}

// Validate sends validation request to gateway
func (c *GatewayClient) Validate(req api.ValidateRequest) (*api.ValidateResponse, error) {
	var validationResp api.ValidateResponse
	if err := c.postJSON("/validate", req, &validationResp); err != nil {
		return nil, err
	}
	return &validationResp, nil
}

// DryRun sends dry run request to gateway
func (c *GatewayClient) DryRun(req api.ValidateRequest) (*api.DryRunResponse, error) {
	var dryRunResp api.DryRunResponse
	if err := c.postJSON("/dryrun", req, &dryRunResp); err != nil {
		return nil, err
	}
	return &dryRunResp, nil
}

// Run sends run request to gateway
func (c *GatewayClient) Run(req api.RunRequest) (*api.RunResponse, error) {
	var runResp api.RunResponse
	if err := c.postJSON("/run", req, &runResp); err != nil {
		return nil, err
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

	var reportResp models.ReportResponse
	if err := c.doJSON(req, &reportResp); err != nil {
		return nil, err
	}
	return &reportResp, nil
}

func (c *GatewayClient) postJSON(path string, reqBody any, out any) error {
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpClient.Post(c.baseURL+path, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to call gateway: %w", err)
	}

	return decodeJSONResponse(resp, out)
}

func (c *GatewayClient) doJSON(req *http.Request, out any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call gateway: %w", err)
	}

	return decodeJSONResponse(resp, out)
}

func decodeJSONResponse(resp *http.Response, out any) error {
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return parseGatewayError(resp.StatusCode, body)
	}

	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return nil
}

func parseGatewayError(statusCode int, body []byte) error {
	var errorResp struct {
		Error  string   `json:"error"`
		Errors []string `json:"errors"`
	}
	if parseErr := json.Unmarshal(body, &errorResp); parseErr == nil {
		if msg := strings.TrimSpace(errorResp.Error); msg != "" {
			return fmt.Errorf("%s", msg)
		}
		for _, msg := range errorResp.Errors {
			msg = strings.TrimSpace(msg)
			if msg != "" {
				return fmt.Errorf("%s", msg)
			}
		}
	}

	return fmt.Errorf("gateway returned status %d: %s", statusCode, string(body))
}
