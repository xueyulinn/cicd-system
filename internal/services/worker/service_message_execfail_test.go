package worker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/xueyulinn/cicd-system/internal/messages"
	"github.com/xueyulinn/cicd-system/internal/models"
	"github.com/moby/moby/client"
)

func TestHandleJobMessage_ExecutionFailureThenCallbackFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/callbacks/job-started" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/callbacks/job-finished" {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	svc := &Service{
		docker:       &client.Client{},
		httpClient:   srv.Client(),
		executionURL: srv.URL,
		jobTimeout:   time.Second,
	}

	err := svc.handleJobMessage(context.Background(), messages.JobExecutionMessage{
		Pipeline: "demo",
		RunNo:    1,
		Stage:    "build",
		RepoURL:  "://bad-url",
		Commit:   "abc",
		Job:      models.JobExecutionPlan{Name: "compile"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "callback job finished (failed)") {
		t.Fatalf("err=%v", err)
	}
}

func TestHandleJobMessage_ExecutionFailureReportedSuccessfully(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/callbacks/job-started" || r.URL.Path == "/callbacks/job-finished" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	svc := &Service{
		docker:       &client.Client{},
		httpClient:   srv.Client(),
		executionURL: srv.URL,
		jobTimeout:   time.Second,
	}

	err := svc.handleJobMessage(context.Background(), messages.JobExecutionMessage{
		Pipeline: "demo",
		RunNo:    1,
		Stage:    "build",
		RepoURL:  "://bad-url",
		Commit:   "abc",
		Job:      models.JobExecutionPlan{Name: "compile"},
	})
	if err != nil {
		t.Fatalf("expected nil err for terminal execution failure reporting, got %v", err)
	}
}
