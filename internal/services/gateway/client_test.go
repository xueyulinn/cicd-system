package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xueyulinn/cicd-system/internal/api"
	"github.com/xueyulinn/cicd-system/internal/models"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type errReadCloser struct{}

func (errReadCloser) Read([]byte) (int, error) {
	return 0, errors.New("read failed")
}

func (errReadCloser) Close() error {
	return nil
}

func postJSON[T any](client *http.Client, endpoint string, reqBody any, errorPrefix string) (*T, error) {
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := client.Post(endpoint, "application/json", bytes.NewReader(jsonBody))
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

func TestPostJSONMarshalError(t *testing.T) {
	client := &http.Client{}
	_, err := postJSON[api.ValidateResponse](client, "http://example.com/validate", map[string]any{
		"bad": make(chan int),
	}, "validation service")
	if err == nil || !strings.Contains(err.Error(), "failed to marshal request") {
		t.Fatalf("expected marshal error, got %v", err)
	}
}

func TestPostJSONCallError(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("network down")
		}),
	}

	_, err := postJSON[api.ValidateResponse](client, "http://example.com/validate", map[string]string{"yaml_content": "x"}, "validation service")
	if err == nil || !strings.Contains(err.Error(), "failed to call validation service") {
		t.Fatalf("expected call error, got %v", err)
	}
}

func TestPostJSONReadError(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       errReadCloser{},
				Header:     make(http.Header),
			}, nil
		}),
	}

	_, err := postJSON[api.ValidateResponse](client, "http://example.com/validate", map[string]string{"yaml_content": "x"}, "validation service")
	if err == nil || !strings.Contains(err.Error(), "failed to read response") {
		t.Fatalf("expected read error, got %v", err)
	}
}

func TestPostJSONNonOKTypedError(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Body:       io.NopCloser(strings.NewReader(`{"valid":false,"errors":["bad yaml"]}`)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	_, err := postJSON[api.ValidateResponse](client, "http://example.com/validate", map[string]string{"yaml_content": "x"}, "validation service")
	if err == nil || !strings.Contains(err.Error(), "bad yaml") {
		t.Fatalf("expected typed error, got %v", err)
	}
}

func TestPostJSONNonOKRawError(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Body:       io.NopCloser(strings.NewReader(`oops`)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	_, err := postJSON[api.ValidateResponse](client, "http://example.com/validate", map[string]string{"yaml_content": "x"}, "validation service")
	if err == nil || !strings.Contains(err.Error(), "returned status 502") {
		t.Fatalf("expected status error, got %v", err)
	}
}

func TestPostJSONUnmarshalError(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`not-json`)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	_, err := postJSON[api.ValidateResponse](client, "http://example.com/validate", map[string]string{"yaml_content": "x"}, "validation service")
	if err == nil || !strings.Contains(err.Error(), "failed to unmarshal response") {
		t.Fatalf("expected unmarshal error, got %v", err)
	}
}

func TestPostJSONSuccess(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodPost {
				t.Fatalf("method = %s, want POST", req.Method)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"valid":true}`)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	resp, err := postJSON[api.ValidateResponse](client, "http://example.com/validate", map[string]string{"yaml_content": "x"}, "validation service")
	if err != nil {
		t.Fatalf("postJSON error = %v", err)
	}
	if !resp.Valid {
		t.Fatal("expected valid response")
	}
}

func TestClientValidateDryRunRunRequests(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/validate":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"valid":true}`))
		case "/dryrun":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"valid":true,"execution_plan":{"stages":[]}}`))
		case "/run":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"queued"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := &Client{
		validationURL:  srv.URL,
		executionURL:   srv.URL,
		httpValidation: srv.Client(),
		httpExecution:  srv.Client(),
	}

	v, err := c.ValidateRequest(context.Background(), "pipeline: {}")
	if err != nil || !v.Valid {
		t.Fatalf("ValidateRequest err=%v resp=%+v", err, v)
	}
	d, err := c.DryRunRequest(context.Background(), "pipeline: {}")
	if err != nil || !d.Valid || d.ExecutionPlan == nil {
		t.Fatalf("DryRunRequest err=%v resp=%+v", err, d)
	}
	r, err := c.RunRequest(context.Background(), api.RunRequest{YAMLContent: "pipeline: {}", Branch: "main", Commit: "abc"})
	if err != nil || r.Status != "queued" {
		t.Fatalf("RunRequest err=%v resp=%+v", err, r)
	}
}

func TestReportRequestScenarios(t *testing.T) {
	t.Run("success with params", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/report" {
				t.Fatalf("path = %s", r.URL.Path)
			}
			q := r.URL.Query()
			if q.Get("pipeline") != "p" || q.Get("run") != "3" || q.Get("stage") != "build" || q.Get("job") != "compile" {
				t.Fatalf("unexpected query: %v", q)
			}
			_, _ = w.Write([]byte(`{"pipeline":{"runs":[{"run-no":3,"status":"success"}]}}`))
		}))
		defer srv.Close()

		run := 3
		c := &Client{
			reportURL:     srv.URL,
			httpReporting: srv.Client(),
		}
		resp, status, err := c.ReportRequest(models.ReportQuery{
			Pipeline: "p",
			Run:      &run,
			Stage:    "build",
			Job:      "compile",
		})
		if err != nil || status != http.StatusOK || len(resp.Pipeline.Runs) != 1 {
			t.Fatalf("ReportRequest err=%v status=%d resp=%+v", err, status, resp)
		}
	})

	t.Run("http call error", func(t *testing.T) {
		c := &Client{
			reportURL: "http://example.com",
			httpReporting: &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return nil, errors.New("dial error")
				}),
			},
		}
		_, status, err := c.ReportRequest(models.ReportQuery{Pipeline: "p"})
		if err == nil || status != http.StatusBadGateway {
			t.Fatalf("expected bad gateway error, got status=%d err=%v", status, err)
		}
	})

	t.Run("non-ok json error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"bad request"}`))
		}))
		defer srv.Close()

		c := &Client{reportURL: srv.URL, httpReporting: srv.Client()}
		_, status, err := c.ReportRequest(models.ReportQuery{Pipeline: "p"})
		if err == nil || status != http.StatusBadRequest || !strings.Contains(err.Error(), "bad request") {
			t.Fatalf("expected parsed error, got status=%d err=%v", status, err)
		}
	})

	t.Run("non-ok plain error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`down`))
		}))
		defer srv.Close()

		c := &Client{reportURL: srv.URL, httpReporting: srv.Client()}
		_, status, err := c.ReportRequest(models.ReportQuery{Pipeline: "p"})
		if err == nil || status != http.StatusServiceUnavailable || !strings.Contains(err.Error(), "returned status 503") {
			t.Fatalf("expected status text error, got status=%d err=%v", status, err)
		}
	})

	t.Run("read response error", func(t *testing.T) {
		c := &Client{
			reportURL: "http://example.com",
			httpReporting: &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       errReadCloser{},
						Header:     make(http.Header),
					}, nil
				}),
			},
		}
		_, status, err := c.ReportRequest(models.ReportQuery{Pipeline: "p"})
		if err == nil || status != http.StatusBadGateway || !strings.Contains(err.Error(), "failed to read response") {
			t.Fatalf("expected read error, got status=%d err=%v", status, err)
		}
	})

	t.Run("unmarshal error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`not-json`))
		}))
		defer srv.Close()

		c := &Client{reportURL: srv.URL, httpReporting: srv.Client()}
		_, status, err := c.ReportRequest(models.ReportQuery{Pipeline: "p"})
		if err == nil || status != http.StatusBadGateway || !strings.Contains(err.Error(), "failed to unmarshal response") {
			t.Fatalf("expected unmarshal error, got status=%d err=%v", status, err)
		}
	})
}
