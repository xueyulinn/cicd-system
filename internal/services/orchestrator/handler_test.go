package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xueyulinn/cicd-system/internal/api"
)

type errReadCloser struct{}

func (errReadCloser) Read([]byte) (int, error) {
	return 0, errors.New("read failed")
}

func (errReadCloser) Close() error {
	return nil
}

func TestHandlerHealth(t *testing.T) {
	h := &Handler{}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.handleHealth(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /health status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), `"healthy"`) {
		t.Fatalf("unexpected response body: %q", rec.Body.String())
	}
}

func TestHandlerHealthMethodNotAllowed(t *testing.T) {
	h := &Handler{}

	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	rec := httptest.NewRecorder()
	h.handleHealth(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /health status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if !strings.Contains(rec.Body.String(), "Method Not Allowed") {
		t.Fatalf("unexpected error response: %q", rec.Body.String())
	}
}

func TestHandlerReadyMethodNotAllowed(t *testing.T) {
	h := &Handler{}

	req := httptest.NewRequest(http.MethodPost, "/ready", nil)
	rec := httptest.NewRecorder()
	h.handleReady(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /ready status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandlerReadyReturnsServiceUnavailableWhenServiceIsNil(t *testing.T) {
	h := &Handler{}

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()
	h.handleReady(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET /ready status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if !strings.Contains(rec.Body.String(), "orchestrator service is not initialized") {
		t.Fatalf("unexpected error response: %q", rec.Body.String())
	}
}

func TestHandlerRunMethodNotAllowed(t *testing.T) {
	h := &Handler{}

	req := httptest.NewRequest(http.MethodGet, "/run", nil)
	rec := httptest.NewRecorder()
	h.handleExecution(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /run status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandlerRunReadBodyError(t *testing.T) {
	h := &Handler{}

	req := httptest.NewRequest(http.MethodPost, "/run", nil)
	req.Body = errReadCloser{}
	rec := httptest.NewRecorder()
	h.handleExecution(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /run read body status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "failed to read request body") {
		t.Fatalf("unexpected error response: %q", rec.Body.String())
	}
}

func TestHandlerRunInvalidJSON(t *testing.T) {
	h := &Handler{}

	req := httptest.NewRequest(http.MethodPost, "/run", bytes.NewBufferString(`{`))
	rec := httptest.NewRecorder()
	h.handleExecution(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /run invalid JSON status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "invalid JSON") {
		t.Fatalf("unexpected error response: %q", rec.Body.String())
	}
}

func TestHandlerRunReturnsInternalServerErrorWhenServiceReturnsError(t *testing.T) {
	h := &Handler{}

	req := httptest.NewRequest(http.MethodPost, "/run", bytes.NewBufferString(`{"yaml_content":""}`))
	rec := httptest.NewRecorder()
	h.handleExecution(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("POST /run status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rec.Body.String(), "pipeline content can not be empty") {
		t.Fatalf("unexpected error response: %q", rec.Body.String())
	}
}

func TestHandlerJobCallbackMethodNotAllowed(t *testing.T) {
	h := &Handler{}

	req := httptest.NewRequest(http.MethodGet, "/callbacks/job-started", nil)
	rec := httptest.NewRecorder()
	h.handleJobCallback(rec, req, func(context.Context, api.JobStatusCallbackRequest) error { return nil })

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET callback status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandlerJobCallbackReadBodyError(t *testing.T) {
	h := &Handler{}

	req := httptest.NewRequest(http.MethodPost, "/callbacks/job-started", nil)
	req.Body = errReadCloser{}
	rec := httptest.NewRecorder()
	h.handleJobCallback(rec, req, func(context.Context, api.JobStatusCallbackRequest) error { return nil })

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST callback read body status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "failed to read request body") {
		t.Fatalf("unexpected error response: %q", rec.Body.String())
	}
}

func TestHandlerJobCallbackInvalidJSON(t *testing.T) {
	h := &Handler{}

	req := httptest.NewRequest(http.MethodPost, "/callbacks/job-started", bytes.NewBufferString(`{`))
	rec := httptest.NewRecorder()
	h.handleJobCallback(rec, req, func(context.Context, api.JobStatusCallbackRequest) error { return nil })

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST callback invalid JSON status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "invalid JSON") {
		t.Fatalf("unexpected error response: %q", rec.Body.String())
	}
}

func TestHandlerJobCallbackFunctionError(t *testing.T) {
	h := &Handler{}

	req := httptest.NewRequest(http.MethodPost, "/callbacks/job-finished", bytes.NewBufferString(`{"pipeline":"p","run_no":1,"stage":"s","job":"j","status":"success"}`))
	rec := httptest.NewRecorder()
	h.handleJobCallback(rec, req, func(context.Context, api.JobStatusCallbackRequest) error {
		return errors.New("boom")
	})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("POST callback function error status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rec.Body.String(), "boom") {
		t.Fatalf("unexpected error response: %q", rec.Body.String())
	}
}

func TestHandlerJobCallbackOK(t *testing.T) {
	h := &Handler{}

	req := httptest.NewRequest(http.MethodPost, "/callbacks/job-finished", bytes.NewBufferString(`{"pipeline":"p","run_no":1,"stage":"s","job":"j","status":"success"}`))
	rec := httptest.NewRecorder()
	h.handleJobCallback(rec, req, func(context.Context, api.JobStatusCallbackRequest) error {
		return nil
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("POST callback status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode callback body: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status body = %q, want ok", body["status"])
	}
}
