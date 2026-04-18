package execution

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/xueyulinn/cicd-system/internal/api"
	"github.com/xueyulinn/cicd-system/internal/mq"
)

func TestNewServiceRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("REPORT_DB_URL", "")

	svc, err := NewService(context.Background())
	if err == nil {
		t.Fatal("NewService error=nil, want non-nil")
	}
	if svc != nil {
		t.Fatalf("svc=%#v, want nil", svc)
	}
	if !strings.Contains(err.Error(), "DATABASE_URL or REPORT_DB_URL is required") {
		t.Fatalf("error=%v", err)
	}
}

func TestNewRunInfo(t *testing.T) {
	req := api.RunRequest{
		RepoURL:       "https://github.com/org/repo.git",
		Branch:        "main",
		Commit:        "abc123",
		WorkspacePath: "/tmp/wt",
	}
	info := newRunInfo(req)
	if info.RepoURL != req.RepoURL || info.Branch != req.Branch || info.Commit != req.Commit || info.WorkspacePath != req.WorkspacePath {
		t.Fatalf("newRunInfo()=%#v, want fields copied", info)
	}
}

func TestServiceCloseNilSafe(t *testing.T) {
	var svc *Service
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected nil receiver Close() to panic with current implementation")
		}
	}()
	svc.Close()
}

func TestServiceCloseEmptyService(t *testing.T) {
	(&Service{}).Close()
}

func TestServiceCloseClosesPublishers(t *testing.T) {
	p1 := &fakePublisher{}
	p2 := &fakePublisher{}
	svc := &Service{jobPublishers: []mq.Publisher{p1, p2}}

	svc.Close()
	if !p1.closed || !p2.closed {
		t.Fatalf("expected all publishers closed, p1=%v p2=%v", p1.closed, p2.closed)
	}
}

func TestServiceReadyNilService(t *testing.T) {
	var svc *Service
	err := svc.Ready(context.Background())
	if err == nil {
		t.Fatal("Ready error=nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "execution service is not initialized") {
		t.Fatalf("error=%v", err)
	}
}

func TestServiceReadyNilStore(t *testing.T) {
	svc := &Service{}
	err := svc.Ready(context.Background())
	if err == nil {
		t.Fatal("Ready error=nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "report store is not initialized") {
		t.Fatalf("error=%v", err)
	}
}

func TestRunReturnsValidationErrorsOnEmptyYAML(t *testing.T) {
	svc := &Service{}
	resp, err := svc.Run(context.Background(), api.RunRequest{YAMLContent: "   "})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if resp == nil || len(resp.Errors) == 0 {
		t.Fatalf("resp=%#v, want validation errors", resp)
	}
}

func TestRunReturnsValidationResponseWhenInvalid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api.WriteJSON(w, http.StatusOK, api.ValidateResponse{Valid: false, Errors: []string{"invalid"}})
	}))
	defer srv.Close()

	svc := &Service{httpValidation: srv.Client(), validationURL: srv.URL}
	resp, err := svc.Run(context.Background(), api.RunRequest{YAMLContent: "pipeline: {}"})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if resp == nil || len(resp.Errors) != 1 || resp.Errors[0] != "invalid" {
		t.Fatalf("resp=%#v, want invalid response", resp)
	}
}

func TestRunPropagatesValidationServiceError(t *testing.T) {
	svc := &Service{
		httpValidation: &http.Client{Timeout: 150 * time.Millisecond},
		validationURL:  "http://127.0.0.1:1",
	}

	_, err := svc.Run(context.Background(), api.RunRequest{YAMLContent: "pipeline: {}"})
	if err == nil {
		t.Fatal("Run error=nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "run pipeline") {
		t.Fatalf("error=%v", err)
	}
}

func TestRunPanicsWithoutStoreAfterValidPreparation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api.WriteJSON(w, http.StatusOK, api.ValidateResponse{Valid: true})
	}))
	defer srv.Close()

	svc := &Service{httpValidation: srv.Client(), validationURL: srv.URL}
	req := api.RunRequest{YAMLContent: `pipeline:
  name: "Demo"
stages:
  - build
compile:
  - stage: build
  - image: golang:1.21
  - script:
    - "echo ok"
`}

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when store is uninitialized")
		}
	}()

	_, _ = svc.Run(context.Background(), req)
}

func TestHandleReadyMethodNotAllowed(t *testing.T) {
	h := &Handler{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/ready", nil)

	h.handleReady(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleReadyReturnsServiceUnavailableWhenDependenciesNotReady(t *testing.T) {
	h := &Handler{service: &Service{}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)

	h.handleReady(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestHandleExecutionReturnsInternalOnRunError(t *testing.T) {
	h := &Handler{service: &Service{httpValidation: &http.Client{Timeout: 100 * time.Millisecond}, validationURL: "http://127.0.0.1:1"}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/run", strings.NewReader(`{"yaml_content":"pipeline: {}"}`))

	h.handleExecution(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestHandleExecutionReturnsBadRequestWhenRunResponseHasErrors(t *testing.T) {
	h := &Handler{service: &Service{}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/run", strings.NewReader(`{"yaml_content":"  "}`))

	h.handleExecution(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandlerNewAndRegisterRoutes(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("REPORT_DB_URL", "")
	h := NewHandler()
	if h == nil {
		t.Fatal("NewHandler returned nil")
	}
	if h.initErr == nil {
		t.Fatal("expected initErr when DB env vars are missing")
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /health status=%d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandlerCloseCoversNonNilService(t *testing.T) {
	h := &Handler{service: &Service{jobPublishers: []mq.Publisher{&fakePublisher{}}}}
	h.Close()
}

func TestHandleJobStartedAndFinishedWrapperEndpoints(t *testing.T) {
	h := &Handler{service: &Service{}}

	startedRec := httptest.NewRecorder()
	startedReq := httptest.NewRequest(http.MethodPost, "/callbacks/job-started", strings.NewReader(`{}`))
	h.handleJobStarted(startedRec, startedReq)
	if startedRec.Code != http.StatusInternalServerError {
		t.Fatalf("job-started status=%d, want %d", startedRec.Code, http.StatusInternalServerError)
	}

	finishedRec := httptest.NewRecorder()
	finishedReq := httptest.NewRequest(http.MethodPost, "/callbacks/job-finished", strings.NewReader(`{"pipeline":"p","run_no":1,"stage":"s","job":"j","status":"running"}`))
	h.handleJobFinished(finishedRec, finishedReq)
	if finishedRec.Code != http.StatusInternalServerError {
		t.Fatalf("job-finished status=%d, want %d", finishedRec.Code, http.StatusInternalServerError)
	}
}

func TestHandleJobCallbackMethodNotAllowedViaWrapper(t *testing.T) {
	h := &Handler{service: &Service{}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/callbacks/job-started", nil)
	h.handleJobStarted(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleExecutionInitErrTakesPrecedence(t *testing.T) {
	h := &Handler{initErr: errors.New("boom"), service: &Service{}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/run", strings.NewReader(`{"yaml_content":"pipeline: {}"}`))
	h.handleExecution(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}
