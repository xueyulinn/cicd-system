package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CS7580-SEA-SP26/e-team/internal/api"
)

func TestExecutionHandlerHealth(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	h.handleHealth(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /health status=%d, want %d", rec.Code, http.StatusOK)
	}
}

func TestExecutionHandlerHealthMethodNotAllowed(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	rec := httptest.NewRecorder()

	h.handleHealth(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /health status=%d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestExecutionHandlerReadyWhenInitFailed(t *testing.T) {
	h := &Handler{initErr: errors.New("init failed")}
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	h.handleReady(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET /ready status=%d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestExecutionHandlerRunMethodNotAllowed(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "/run", nil)
	rec := httptest.NewRecorder()

	h.handleExecution(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /run status=%d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestExecutionHandlerRunWhenInitFailed(t *testing.T) {
	h := &Handler{initErr: errors.New("init failed")}
	req := httptest.NewRequest(http.MethodPost, "/run", bytes.NewBufferString(`{"yaml_content":"x"}`))
	rec := httptest.NewRecorder()

	h.handleExecution(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("POST /run status=%d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestExecutionHandlerRunInvalidJSON(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPost, "/run", bytes.NewBufferString(`{`))
	rec := httptest.NewRecorder()

	h.handleExecution(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /run invalid JSON status=%d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestExecutionHandlerJobCallbackMethodNotAllowed(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "/callbacks/job-started", nil)
	rec := httptest.NewRecorder()

	h.handleJobCallback(rec, req, func(context.Context, api.JobStatusCallbackRequest) error { return nil })
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET callback status=%d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestExecutionHandlerJobCallbackWhenInitFailed(t *testing.T) {
	h := &Handler{initErr: errors.New("init failed")}
	req := httptest.NewRequest(http.MethodPost, "/callbacks/job-started", bytes.NewBufferString(`{"pipeline":"p"}`))
	rec := httptest.NewRecorder()

	h.handleJobCallback(rec, req, func(context.Context, api.JobStatusCallbackRequest) error { return nil })
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("POST callback status=%d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestExecutionHandlerJobCallbackInvalidJSON(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPost, "/callbacks/job-started", bytes.NewBufferString(`{`))
	rec := httptest.NewRecorder()

	h.handleJobCallback(rec, req, func(context.Context, api.JobStatusCallbackRequest) error { return nil })
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST callback invalid JSON status=%d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestExecutionHandlerJobCallbackFunctionError(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPost, "/callbacks/job-finished", bytes.NewBufferString(`{"pipeline":"p","run_no":1,"stage":"s","job":"j","status":"success"}`))
	rec := httptest.NewRecorder()

	h.handleJobCallback(rec, req, func(context.Context, api.JobStatusCallbackRequest) error {
		return errors.New("boom")
	})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("POST callback function error status=%d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestExecutionHandlerJobCallbackOK(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPost, "/callbacks/job-finished", bytes.NewBufferString(`{"pipeline":"p","run_no":1,"stage":"s","job":"j","status":"success"}`))
	rec := httptest.NewRecorder()

	h.handleJobCallback(rec, req, func(context.Context, api.JobStatusCallbackRequest) error {
		return nil
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST callback status=%d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode callback body: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status body=%q, want ok", body["status"])
	}
}
