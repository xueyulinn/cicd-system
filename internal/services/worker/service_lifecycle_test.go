package worker

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/CS7580-SEA-SP26/e-team/internal/api"
	"github.com/CS7580-SEA-SP26/e-team/internal/messages"
	"github.com/CS7580-SEA-SP26/e-team/internal/models"
	"github.com/CS7580-SEA-SP26/e-team/internal/mq"
	"github.com/moby/moby/client"
)

func TestServiceClose_NilSafeAndClosesConsumers(t *testing.T) {
	var nilSvc *Service
	if err := nilSvc.Close(); err != nil {
		t.Fatalf("nil Close err=%v", err)
	}

	c1 := &fakeConsumer{}
	c2 := &fakeConsumer{}
	svc := &Service{jobConsumers: []mq.Consumer{c1, c2}}
	if err := svc.Close(); err != nil {
		t.Fatalf("Close err=%v", err)
	}
	if !c1.closed || !c2.closed {
		t.Fatalf("expected consumers closed, c1=%v c2=%v", c1.closed, c2.closed)
	}
}

func TestServiceReady_NilAndDockerUnavailable(t *testing.T) {
	var nilSvc *Service
	err := nilSvc.Ready(context.Background())
	if err == nil || !strings.Contains(err.Error(), "worker service is nil") {
		t.Fatalf("err=%v", err)
	}

	svc := &Service{docker: nil}
	err = svc.Ready(context.Background())
	if err == nil || !strings.Contains(err.Error(), "docker not ready") {
		t.Fatalf("err=%v", err)
	}
}

func TestHandleJobMessage_CallbackStartError(t *testing.T) {
	svc := &Service{
		docker:       &client.Client{},
		httpClient:   nil,
		executionURL: "http://example.invalid",
		jobTimeout:   time.Second,
	}

	err := svc.handleJobMessage(context.Background(), messages.JobExecutionMessage{
		Pipeline: "demo",
		RunNo:    1,
		Stage:    "build",
		Job:      models.JobExecutionPlan{Name: "compile"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "callback job started") {
		t.Fatalf("err=%v", err)
	}
}

func TestPostJobCallback_CreateRequestError(t *testing.T) {
	svc := &Service{
		httpClient:   &http.Client{Timeout: time.Second},
		executionURL: "://bad-url",
	}
	err := svc.postJobCallback(context.Background(), "/callbacks/job-started", api.JobStatusCallbackRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "create callback request") {
		t.Fatalf("err=%v", err)
	}
}
