package worker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/xueyulinn/cicd-system/internal/api"
	"github.com/xueyulinn/cicd-system/internal/messages"
	"github.com/xueyulinn/cicd-system/internal/models"
)

func TestPostJobCallback_SendsExpectedPayload(t *testing.T) {
	gotPath := ""
	var gotPayload api.JobStatusCallbackRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Method != http.MethodPost {
			t.Fatalf("method=%s, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
			t.Fatalf("Content-Type=%q, want application/json", ct)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	svc := &Service{orchestratorURL: srv.URL, httpClient: &http.Client{Timeout: time.Second}}
	msg := messages.JobExecutionMessage{RunNo: 9, PipelineName: "p", Stage: "build", Job: models.JobExecutionPlan{Name: "compile"}}

	if err := svc.callbackJobStarted(context.Background(), msg); err != nil {
		t.Fatalf("callbackJobStarted error: %v", err)
	}
	if gotPath != "/callbacks/job-started" {
		t.Fatalf("path=%q, want /callbacks/job-started", gotPath)
	}
	if gotPayload.Status != "started" || gotPayload.Job != "compile" {
		t.Fatalf("payload=%#v", gotPayload)
	}

	if err := svc.callbackJobFinished(context.Background(), msg, "failed", "logs", "boom"); err != nil {
		t.Fatalf("callbackJobFinished error: %v", err)
	}
	if gotPath != "/callbacks/job-finished" {
		t.Fatalf("path=%q, want /callbacks/job-finished", gotPath)
	}
	if gotPayload.Status != "failed" || gotPayload.Logs != "logs" || gotPayload.Error != "boom" {
		t.Fatalf("payload=%#v", gotPayload)
	}
}

func TestPostJobCallback_ReturnsErrorOnNonOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	svc := &Service{orchestratorURL: srv.URL, httpClient: srv.Client()}
	err := svc.postJobCallback(context.Background(), "/callbacks/job-finished", api.JobStatusCallbackRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "status 502") {
		t.Fatalf("err=%v", err)
	}
}

func TestPostJobCallback_ReturnsErrorWhenHTTPClientMissing(t *testing.T) {
	svc := &Service{orchestratorURL: "http://example.invalid", httpClient: nil}
	err := svc.postJobCallback(context.Background(), "/callbacks/job-started", api.JobStatusCallbackRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "http client is not initialized") {
		t.Fatalf("err=%v", err)
	}
}

func TestPostJobCallback_SendRequestError(t *testing.T) {
	svc := &Service{
		httpClient:      &http.Client{Timeout: 100 * time.Millisecond},
		orchestratorURL: "http://127.0.0.1:1",
	}
	err := svc.postJobCallback(context.Background(), "/callbacks/job-started", api.JobStatusCallbackRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "send callback request") {
		t.Fatalf("err=%v", err)
	}
}
