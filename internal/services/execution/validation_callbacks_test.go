package execution

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/CS7580-SEA-SP26/e-team/internal/api"
)

func TestValidatePipeline_HTTPErrorWithEmptyValidationErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(api.ValidateResponse{Valid: false})
	}))
	defer srv.Close()

	svc := &Service{httpValidation: srv.Client(), validationURL: srv.URL}
	resp, err := svc.validatePipeline("pipeline: {}")
	if err != nil {
		t.Fatalf("validatePipeline error: %v", err)
	}
	if resp.Valid {
		t.Fatal("expected Valid=false")
	}
	if len(resp.Errors) != 1 || !strings.Contains(resp.Errors[0], "status 400") {
		t.Fatalf("unexpected errors: %#v", resp.Errors)
	}
}

func TestValidatePipeline_HTTPErrorPreservesValidationErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(api.ValidateResponse{Valid: false, Errors: []string{"bad stage"}})
	}))
	defer srv.Close()

	svc := &Service{httpValidation: srv.Client(), validationURL: srv.URL}
	resp, err := svc.validatePipeline("pipeline: {}")
	if err != nil {
		t.Fatalf("validatePipeline error: %v", err)
	}
	if resp.Valid {
		t.Fatal("expected Valid=false")
	}
	if len(resp.Errors) != 1 || resp.Errors[0] != "bad stage" {
		t.Fatalf("unexpected errors: %#v", resp.Errors)
	}
}

func TestPrepareRun_EmptyYAML(t *testing.T) {
	svc := &Service{}
	prepared, runResp, err := svc.prepareRun(api.RunRequest{YAMLContent: "   "})
	if err != nil {
		t.Fatalf("prepareRun error: %v", err)
	}
	if prepared != nil {
		t.Fatalf("prepared=%#v, want nil", prepared)
	}
	if runResp == nil || len(runResp.Errors) == 0 {
		t.Fatalf("runResp=%#v, want validation error", runResp)
	}
}

func TestPrepareRun_ValidationInvalid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(api.ValidateResponse{Valid: false, Errors: []string{"invalid pipeline"}})
	}))
	defer srv.Close()

	svc := &Service{httpValidation: srv.Client(), validationURL: srv.URL}
	prepared, runResp, err := svc.prepareRun(api.RunRequest{YAMLContent: "pipeline: {}"})
	if err != nil {
		t.Fatalf("prepareRun error: %v", err)
	}
	if prepared != nil {
		t.Fatalf("prepared=%#v, want nil", prepared)
	}
	if runResp == nil || len(runResp.Errors) != 1 || runResp.Errors[0] != "invalid pipeline" {
		t.Fatalf("runResp=%#v, want invalid pipeline error", runResp)
	}
}

func TestPrepareRun_ParseFailureAfterValidationSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(api.ValidateResponse{Valid: true})
	}))
	defer srv.Close()

	svc := &Service{httpValidation: srv.Client(), validationURL: srv.URL}
	prepared, runResp, err := svc.prepareRun(api.RunRequest{YAMLContent: "{"})
	if err != nil {
		t.Fatalf("prepareRun error: %v", err)
	}
	if prepared != nil {
		t.Fatalf("prepared=%#v, want nil", prepared)
	}
	if runResp == nil || len(runResp.Errors) == 0 || !strings.Contains(runResp.Errors[0], "pipeline parse failed") {
		t.Fatalf("runResp=%#v, want parse error", runResp)
	}
}

func TestPrepareRun_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/validate" {
			t.Fatalf("unexpected call: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(api.ValidateResponse{Valid: true})
	}))
	defer srv.Close()

	yml := `pipeline:
  name: "Demo"
stages:
  - build
compile:
  - stage: build
  - image: golang:1.21
  - script:
    - "go test ./..."
`

	svc := &Service{httpValidation: srv.Client(), validationURL: srv.URL}
	prepared, runResp, err := svc.prepareRun(api.RunRequest{YAMLContent: yml})
	if err != nil {
		t.Fatalf("prepareRun error: %v", err)
	}
	if runResp != nil {
		t.Fatalf("runResp=%#v, want nil", runResp)
	}
	if prepared == nil || prepared.Pipeline == nil || prepared.ExecutionPlan == nil {
		t.Fatalf("prepared=%#v, want non-nil pipeline and execution plan", prepared)
	}
	if strings.TrimSpace(prepared.Pipeline.Name) == "" {
		t.Fatal("pipeline name should not be empty")
	}
	if len(prepared.ExecutionPlan.Stages) == 0 {
		t.Fatal("expected at least one stage in execution plan")
	}
}

func TestHandleJobStarted_ValidatesRequiredFields(t *testing.T) {
	svc := &Service{}
	err := svc.HandleJobStarted(context.Background(), api.JobStatusCallbackRequest{})
	if err == nil {
		t.Fatal("HandleJobStarted error=nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "pipeline, run_no, stage, and job are required") {
		t.Fatalf("error=%v", err)
	}
}

func TestHandleJobFinished_ValidatesRequiredFields(t *testing.T) {
	svc := &Service{}
	err := svc.HandleJobFinished(context.Background(), api.JobStatusCallbackRequest{})
	if err == nil {
		t.Fatal("HandleJobFinished error=nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "pipeline, run_no, stage, and job are required") {
		t.Fatalf("error=%v", err)
	}
}

func TestHandleJobFinished_RejectsInvalidStatus(t *testing.T) {
	svc := &Service{}
	err := svc.HandleJobFinished(context.Background(), api.JobStatusCallbackRequest{
		Pipeline: "p",
		RunNo:    1,
		Stage:    "build",
		Job:      "compile",
		Status:   "running",
	})
	if err == nil {
		t.Fatal("HandleJobFinished error=nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "invalid finished status") {
		t.Fatalf("error=%v", err)
	}
}
