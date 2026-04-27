package validation

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const validPipelineYAML = `
pipeline:
  name: "Demo"
stages:
  - build
compile:
  - stage: build
  - image: golang:1.25
  - script:
    - go test ./...
`

const invalidPipelineYAML = `
pipeline:
  name: "Demo"
stages:
  - build
  - test
compile:
  - stage: build
  - needs: [missing-job]
  - image: golang:1.25
  - script:
    - go test ./...
`

func newValidationMux(t *testing.T) *http.ServeMux {
	t.Helper()

	h, err := NewHandler()
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	return mux
}

func doRequest(t *testing.T, mux *http.ServeMux, method, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

type errReadCloser struct{}

func (errReadCloser) Read(p []byte) (int, error) {
	return 0, errors.New("read failed")
}

func (errReadCloser) Close() error {
	return nil
}

func doRequestWithBody(t *testing.T, mux *http.ServeMux, method, path string, body io.ReadCloser) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	req.Body = body
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestNewHandler(t *testing.T) {
	h, err := NewHandler()
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	if h == nil {
		t.Fatal("expected handler to be initialized")
		return
	}
	if h.service == nil {
		t.Fatal("expected service to be initialized")
	}
}

func TestHandleHealth(t *testing.T) {
	mux := newValidationMux(t)

	rec := doRequest(t, mux, http.MethodGet, "/health", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /health status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), `"healthy"`) {
		t.Fatalf("unexpected response body: %q", rec.Body.String())
	}

	rec = doRequest(t, mux, http.MethodPost, "/health", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /health status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if !strings.Contains(rec.Body.String(), "Method Not Allowed") {
		t.Fatalf("unexpected error response: %q", rec.Body.String())
	}
}

func TestHandleReady(t *testing.T) {
	mux := newValidationMux(t)

	rec := doRequest(t, mux, http.MethodGet, "/ready", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /ready status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), `"ready"`) {
		t.Fatalf("unexpected response body: %q", rec.Body.String())
	}

	rec = doRequest(t, mux, http.MethodPost, "/ready", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /ready status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleValidate(t *testing.T) {
	mux := newValidationMux(t)

	rec := doRequest(t, mux, http.MethodGet, "/validate", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /validate status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}

	rec = doRequest(t, mux, http.MethodPost, "/validate", []byte("{"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid JSON status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "invalid JSON") {
		t.Fatalf("unexpected invalid JSON error: %q", rec.Body.String())
	}

	rec = doRequestWithBody(t, mux, http.MethodPost, "/validate", errReadCloser{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("read body error status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "failed to read request body") {
		t.Fatalf("unexpected read body error: %q", rec.Body.String())
	}

	invalidReq, err := json.Marshal(map[string]string{"yaml_content": invalidPipelineYAML})
	if err != nil {
		t.Fatalf("marshal request failed: %v", err)
	}
	rec = doRequest(t, mux, http.MethodPost, "/validate", invalidReq)
	if rec.Code != http.StatusOK {
		t.Fatalf("invalid pipeline status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), `"valid":false`) {
		t.Fatalf("expected invalid response, got: %q", rec.Body.String())
	}

	validReq, err := json.Marshal(map[string]string{"yaml_content": validPipelineYAML})
	if err != nil {
		t.Fatalf("marshal request failed: %v", err)
	}
	rec = doRequest(t, mux, http.MethodPost, "/validate", validReq)
	if rec.Code != http.StatusOK {
		t.Fatalf("valid pipeline status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), `"valid":true`) {
		t.Fatalf("expected valid response, got: %q", rec.Body.String())
	}
}

func TestHandleDryRun(t *testing.T) {
	mux := newValidationMux(t)

	rec := doRequest(t, mux, http.MethodGet, "/dryrun", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /dryrun status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}

	rec = doRequest(t, mux, http.MethodPost, "/dryrun", []byte("{"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid JSON status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "invalid JSON") {
		t.Fatalf("unexpected invalid JSON error: %q", rec.Body.String())
	}

	rec = doRequestWithBody(t, mux, http.MethodPost, "/dryrun", errReadCloser{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("read body error status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "failed to read request body") {
		t.Fatalf("unexpected read body error: %q", rec.Body.String())
	}

	invalidReq, err := json.Marshal(map[string]string{"yaml_content": invalidPipelineYAML})
	if err != nil {
		t.Fatalf("marshal request failed: %v", err)
	}
	rec = doRequest(t, mux, http.MethodPost, "/dryrun", invalidReq)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid pipeline status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), `"valid":false`) {
		t.Fatalf("expected invalid response, got: %q", rec.Body.String())
	}

	validReq, err := json.Marshal(map[string]string{"yaml_content": validPipelineYAML})
	if err != nil {
		t.Fatalf("marshal request failed: %v", err)
	}
	rec = doRequest(t, mux, http.MethodPost, "/dryrun", validReq)
	if rec.Code != http.StatusOK {
		t.Fatalf("valid pipeline status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"valid":true`) || !strings.Contains(body, "compile") {
		t.Fatalf("unexpected dryrun response: %q", body)
	}
}
