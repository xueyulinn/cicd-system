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

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return false }

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

func TestClientForwardValidateDryRunRun(t *testing.T) {
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
		validationURL:      srv.URL,
		orchestratorURL:    srv.URL,
		validationClient:   srv.Client(),
		orchestratorClient: srv.Client(),
	}

	validateReq := httptest.NewRequest(http.MethodPost, "/validate", strings.NewReader(`{"yaml_content":"pipeline: {}"}`))
	validateRec := httptest.NewRecorder()
	if err := c.forwardValidate(context.Background(), validateRec, validateReq); err != nil {
		t.Fatalf("forwardValidate err=%v", err)
	}
	if validateRec.Code != http.StatusOK {
		t.Fatalf("forwardValidate code=%d want=%d", validateRec.Code, http.StatusOK)
	}
	var v api.ValidateResponse
	if err := json.Unmarshal(validateRec.Body.Bytes(), &v); err != nil {
		t.Fatalf("forwardValidate unmarshal err=%v body=%q", err, validateRec.Body.String())
	}
	if !v.Valid {
		t.Fatalf("forwardValidate resp=%+v", v)
	}

	dryReq := httptest.NewRequest(http.MethodPost, "/dryrun", strings.NewReader(`{"yaml_content":"pipeline: {}"}`))
	dryRec := httptest.NewRecorder()
	if err := c.forwardDryRun(context.Background(), dryRec, dryReq); err != nil {
		t.Fatalf("forwardDryRun err=%v", err)
	}
	if dryRec.Code != http.StatusOK {
		t.Fatalf("forwardDryRun code=%d want=%d", dryRec.Code, http.StatusOK)
	}
	var d api.DryRunResponse
	if err := json.Unmarshal(dryRec.Body.Bytes(), &d); err != nil {
		t.Fatalf("forwardDryRun unmarshal err=%v body=%q", err, dryRec.Body.String())
	}
	if !d.Valid || d.ExecutionPlan == nil {
		t.Fatalf("forwardDryRun resp=%+v", d)
	}

	runReq := httptest.NewRequest(http.MethodPost, "/run", strings.NewReader(`{"yaml_content":"pipeline: {}","branch":"main","commit":"abc"}`))
	runRec := httptest.NewRecorder()
	if err := c.forwardRun(context.Background(), runRec, runReq); err != nil {
		t.Fatalf("forwardRun err=%v", err)
	}
	if runRec.Code != http.StatusOK {
		t.Fatalf("forwardRun code=%d want=%d", runRec.Code, http.StatusOK)
	}
	var r api.RunResponse
	if err := json.Unmarshal(runRec.Body.Bytes(), &r); err != nil {
		t.Fatalf("forwardRun unmarshal err=%v body=%q", err, runRec.Body.String())
	}
	if r.Status != "queued" {
		t.Fatalf("forwardRun resp=%+v", r)
	}
}

func TestClientForwardValidateCallError(t *testing.T) {
	t.Run("non-timeout call error", func(t *testing.T) {
		validateReq := httptest.NewRequest(http.MethodPost, "/validate", strings.NewReader(`{"yaml_content":"pipeline: {}"}`))
		validateRec := httptest.NewRecorder()
		c := &Client{
			validationURL: "http://example.com",
			validationClient: &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return nil, errors.New("network down")
				}),
			},
		}

		err := c.forwardValidate(context.Background(), validateRec, validateReq)
		if err == nil || !strings.Contains(err.Error(), "failed to call validation service") {
			t.Fatalf("expected call error, got err=%v", err)
		}
		if errors.Is(err, errUpstreamTimeout) {
			t.Fatalf("non-timeout error misclassified as timeout: %v", err)
		}
	})

	t.Run("timeout call error", func(t *testing.T) {
		validateReq := httptest.NewRequest(http.MethodPost, "/validate", strings.NewReader(`{"yaml_content":"pipeline: {}"}`))
		validateRec := httptest.NewRecorder()
		c := &Client{
			validationURL: "http://example.com",
			validationClient: &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return nil, timeoutErr{}
				}),
			},
		}

		err := c.forwardValidate(context.Background(), validateRec, validateReq)
		if err == nil || !errors.Is(err, errUpstreamTimeout) {
			t.Fatalf("expected timeout classification, got err=%v", err)
		}
	})
}

func TestClientForwardDryRunCallError(t *testing.T) {
	t.Run("non-timeout call error", func(t *testing.T) {
		dryReq := httptest.NewRequest(http.MethodPost, "/dryrun", strings.NewReader(`{"yaml_content":"pipeline: {}"}`))
		dryRec := httptest.NewRecorder()
		c := &Client{
			validationURL: "http://example.com",
			validationClient: &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return nil, errors.New("network down")
				}),
			},
		}

		err := c.forwardDryRun(context.Background(), dryRec, dryReq)
		if err == nil || !strings.Contains(err.Error(), "failed to call dryrun service") {
			t.Fatalf("expected call error, got err=%v", err)
		}
		if errors.Is(err, errUpstreamTimeout) {
			t.Fatalf("non-timeout error misclassified as timeout: %v", err)
		}
	})

	t.Run("timeout call error", func(t *testing.T) {
		dryReq := httptest.NewRequest(http.MethodPost, "/dryrun", strings.NewReader(`{"yaml_content":"pipeline: {}"}`))
		dryRec := httptest.NewRecorder()
		c := &Client{
			validationURL: "http://example.com",
			validationClient: &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return nil, timeoutErr{}
				}),
			},
		}

		err := c.forwardDryRun(context.Background(), dryRec, dryReq)
		if err == nil || !errors.Is(err, errUpstreamTimeout) {
			t.Fatalf("expected timeout classification, got err=%v", err)
		}
	})
}

func TestClientForwardRunCallError(t *testing.T) {
	t.Run("non-timeout call error", func(t *testing.T) {
		runReq := httptest.NewRequest(http.MethodPost, "/run", strings.NewReader(`{"yaml_content":"pipeline: {}","branch":"main","commit":"abc"}`))
		runRec := httptest.NewRecorder()
		c := &Client{
			orchestratorURL: "http://example.com",
			orchestratorClient: &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return nil, errors.New("network down")
				}),
			},
		}

		err := c.forwardRun(context.Background(), runRec, runReq)
		if err == nil || !strings.Contains(err.Error(), "failed to call orchestrator service") {
			t.Fatalf("expected call error, got err=%v", err)
		}
		if errors.Is(err, errUpstreamTimeout) {
			t.Fatalf("non-timeout error misclassified as timeout: %v", err)
		}
	})

	t.Run("timeout call error", func(t *testing.T) {
		runReq := httptest.NewRequest(http.MethodPost, "/run", strings.NewReader(`{"yaml_content":"pipeline: {}","branch":"main","commit":"abc"}`))
		runRec := httptest.NewRecorder()
		c := &Client{
			orchestratorURL: "http://example.com",
			orchestratorClient: &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return nil, timeoutErr{}
				}),
			},
		}

		err := c.forwardRun(context.Background(), runRec, runReq)
		if err == nil || !errors.Is(err, errUpstreamTimeout) {
			t.Fatalf("expected timeout classification, got err=%v", err)
		}
	})
}

func TestClientForwardReport(t *testing.T) {
	t.Run("success with params", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/report" {
				t.Fatalf("path = %s", r.URL.Path)
			}
			q := r.URL.Query()
			if q.Get("pipeline") != "p" || q.Get("run") != "3" || q.Get("stage") != "build" || q.Get("job") != "compile" {
				t.Fatalf("unexpected query: %v", q)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"pipeline":{"name":"p"}}`))
		}))
		defer srv.Close()

		run := 3
		c := &Client{
			reportURL:       srv.URL,
			reportingClient: srv.Client(),
		}
		rec := httptest.NewRecorder()
		err := c.forwardReport(context.Background(), rec, models.ReportQuery{
			Pipeline: "p",
			Run:      &run,
			Stage:    "build",
			Job:      "compile",
		})
		if err != nil {
			t.Fatalf("forwardReport err=%v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("forwardReport code=%d want=%d", rec.Code, http.StatusOK)
		}
		if got := strings.TrimSpace(rec.Body.String()); got != `{"pipeline":{"name":"p"}}` {
			t.Fatalf("forwardReport body=%q", got)
		}
	})

	t.Run("http call error", func(t *testing.T) {
		c := &Client{
			reportURL: "http://example.com",
			reportingClient: &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return nil, errors.New("dial error")
				}),
			},
		}
		rec := httptest.NewRecorder()
		err := c.forwardReport(context.Background(), rec, models.ReportQuery{Pipeline: "p"})
		if err == nil || !strings.Contains(err.Error(), "failed to call reporting service") {
			t.Fatalf("expected call error, got err=%v", err)
		}
	})

	t.Run("timeout call error", func(t *testing.T) {
		c := &Client{
			reportURL: "http://example.com",
			reportingClient: &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return nil, timeoutErr{}
				}),
			},
		}
		rec := httptest.NewRecorder()
		err := c.forwardReport(context.Background(), rec, models.ReportQuery{Pipeline: "p"})
		if err == nil || !errors.Is(err, errUpstreamTimeout) {
			t.Fatalf("expected timeout classification, got err=%v", err)
		}
	})

	t.Run("downstream non-ok is proxied", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"code":"report_not_found","message":"missing"}`))
		}))
		defer srv.Close()

		c := &Client{reportURL: srv.URL, reportingClient: srv.Client()}
		rec := httptest.NewRecorder()
		if err := c.forwardReport(context.Background(), rec, models.ReportQuery{Pipeline: "p"}); err != nil {
			t.Fatalf("forwardReport err=%v", err)
		}
		if rec.Code != http.StatusNotFound {
			t.Fatalf("forwardReport code=%d want=%d", rec.Code, http.StatusNotFound)
		}
		if got := strings.TrimSpace(rec.Body.String()); got != `{"code":"report_not_found","message":"missing"}` {
			t.Fatalf("forwardReport body=%q", got)
		}
	})

	t.Run("copy response error", func(t *testing.T) {
		c := &Client{
			reportURL: "http://example.com",
			reportingClient: &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       errReadCloser{},
						Header:     make(http.Header),
					}, nil
				}),
			},
		}
		rec := httptest.NewRecorder()
		err := c.forwardReport(context.Background(), rec, models.ReportQuery{Pipeline: "p"})
		if err == nil || !strings.Contains(err.Error(), "copy downstream response failed") {
			t.Fatalf("expected copy error, got err=%v", err)
		}
	})
}
