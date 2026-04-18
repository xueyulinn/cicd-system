package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/xueyulinn/cicd-system/internal/api"
	"github.com/xueyulinn/cicd-system/internal/config"
	"github.com/xueyulinn/cicd-system/internal/models"
)

func TestNewGatewayClient_UsesDefaultAndEnv(t *testing.T) {
	t.Setenv("GATEWAY_URL", "")
	c := NewGatewayClient()
	if c.baseURL != config.DefaultGatewayURL {
		t.Fatalf("baseURL=%q, want %q", c.baseURL, config.DefaultGatewayURL)
	}

	t.Setenv("GATEWAY_URL", "http://example.test")
	c = NewGatewayClient()
	if c.baseURL != "http://example.test" {
		t.Fatalf("baseURL=%q, want env value", c.baseURL)
	}
}

func TestGatewayClientValidate_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/validate" || r.Method != http.MethodPost {
			t.Fatalf("unexpected route: %s %s", r.Method, r.URL.Path)
		}
		var req map[string]string
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode req: %v", err)
		}
		if req["yaml_content"] == "" {
			t.Fatal("yaml_content is empty")
		}
		_ = json.NewEncoder(w).Encode(api.ValidateResponse{Valid: true})
	}))
	defer srv.Close()

	c := &GatewayClient{baseURL: srv.URL, httpClient: srv.Client()}
	resp, err := c.Validate("pipeline: {}")
	if err != nil {
		t.Fatalf("Validate err: %v", err)
	}
	if resp == nil || !resp.Valid {
		t.Fatalf("resp=%#v, want valid=true", resp)
	}
}

func TestGatewayClientValidate_ErrorBodyAndStatusFallback(t *testing.T) {
	t.Run("json error body", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"bad request"}`))
		}))
		defer srv.Close()

		c := &GatewayClient{baseURL: srv.URL, httpClient: srv.Client()}
		_, err := c.Validate("x")
		if err == nil || err.Error() != "bad request" {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("status fallback", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("oops"))
		}))
		defer srv.Close()

		c := &GatewayClient{baseURL: srv.URL, httpClient: srv.Client()}
		_, err := c.Validate("x")
		if err == nil || !strings.Contains(err.Error(), "gateway returned status 500") {
			t.Fatalf("err=%v", err)
		}
	})
}

func TestGatewayClientDryRun_Run_Report(t *testing.T) {
	var seenReportQuery map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/dryrun":
			_ = json.NewEncoder(w).Encode(api.DryRunResponse{Valid: true, Output: "ok"})
		case "/run":
			_ = json.NewEncoder(w).Encode(api.RunResponse{Pipeline: "demo", RunNo: 7, Status: "queued"})
		case "/report":
			seenReportQuery = map[string]string{}
			for key, vals := range r.URL.Query() {
				if len(vals) > 0 {
					seenReportQuery[key] = vals[0]
				}
			}
			_ = json.NewEncoder(w).Encode(models.ReportResponse{Pipeline: models.ReportPipeline{Name: "demo"}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := &GatewayClient{baseURL: srv.URL, httpClient: srv.Client()}

	dry, err := c.DryRun("pipeline: {}")
	if err != nil || dry == nil || !dry.Valid || dry.Output != "ok" {
		t.Fatalf("dry=%#v err=%v", dry, err)
	}

	runResp, err := c.Run(api.RunRequest{YAMLContent: "pipeline: {}", Branch: "main", Commit: "abc"})
	if err != nil || runResp == nil || runResp.RunNo != 7 {
		t.Fatalf("run=%#v err=%v", runResp, err)
	}

	r := 3
	report, err := c.Report(models.ReportQuery{Pipeline: "demo", Run: &r, Stage: "build", Job: "compile"})
	if err != nil || report == nil || report.Pipeline.Name != "demo" {
		t.Fatalf("report=%#v err=%v", report, err)
	}
	if seenReportQuery["pipeline"] != "demo" || seenReportQuery["run"] != strconv.Itoa(r) || seenReportQuery["stage"] != "build" || seenReportQuery["job"] != "compile" {
		t.Fatalf("unexpected query params: %#v", seenReportQuery)
	}
}

func TestGatewayClient_Non200ForDryRunRunReport(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func(c *GatewayClient) error
	}{
		{name: "dryrun", call: func(c *GatewayClient) error { _, err := c.DryRun("x"); return err }},
		{name: "run", call: func(c *GatewayClient) error { _, err := c.Run(api.RunRequest{}); return err }},
		{name: "report", call: func(c *GatewayClient) error { _, err := c.Report(models.ReportQuery{Pipeline: "p"}); return err }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadGateway)
				_, _ = w.Write([]byte(`{"error":"gateway down"}`))
			}))
			defer srv.Close()
			c := &GatewayClient{baseURL: srv.URL, httpClient: srv.Client()}
			err := tc.call(c)
			if err == nil || err.Error() != "gateway down" {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestGatewayClient_BadJSONResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{"))
	}))
	defer srv.Close()

	c := &GatewayClient{baseURL: srv.URL, httpClient: srv.Client()}
	if _, err := c.Validate("x"); err == nil {
		t.Fatal("expected unmarshal error")
	}
	if _, err := c.DryRun("x"); err == nil {
		t.Fatal("expected unmarshal error")
	}
	if _, err := c.Run(api.RunRequest{}); err == nil {
		t.Fatal("expected unmarshal error")
	}
	if _, err := c.Report(models.ReportQuery{Pipeline: "p"}); err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestGatewayClient_NetworkError(t *testing.T) {
	c := &GatewayClient{baseURL: "http://127.0.0.1:1", httpClient: http.DefaultClient}
	if _, err := c.Validate("x"); err == nil {
		t.Fatal("expected network error")
	}
	if _, err := c.DryRun("x"); err == nil {
		t.Fatal("expected network error")
	}
	if _, err := c.Run(api.RunRequest{}); err == nil {
		t.Fatal("expected network error")
	}
	if _, err := c.Report(models.ReportQuery{Pipeline: "p"}); err == nil {
		t.Fatal("expected network error")
	}
}

func TestGatewayClient_UsesEnvInConstructionForReportPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/report" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(models.ReportResponse{})
	}))
	defer srv.Close()

	t.Setenv("GATEWAY_URL", srv.URL)
	c := NewGatewayClient()
	c.httpClient = srv.Client()
	_, err := c.Report(models.ReportQuery{Pipeline: "demo"})
	if err != nil {
		t.Fatalf("Report err=%v", err)
	}

	_ = os.Unsetenv("GATEWAY_URL")
}
