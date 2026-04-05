package mq

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/CS7580-SEA-SP26/e-team/internal/messages"
	"github.com/CS7580-SEA-SP26/e-team/internal/models"
)

type publisherTestContextKey string

type fakeRawPublisher struct {
	publishFn func(context.Context, string, []byte) error
	closeErr  error
}

func (f *fakeRawPublisher) Publish(ctx context.Context, queue string, body []byte) error {
	if f.publishFn != nil {
		return f.publishFn(ctx, queue, body)
	}
	return nil
}

func (f *fakeRawPublisher) Close() error {
	return f.closeErr
}

func TestNewJobPublisher(t *testing.T) {
	cfg := Config{URL: "amqp://guest:guest@localhost:5672/", JobQueue: "jobs"}

	publisher, err := NewJobPublisher(&fakeRawPublisher{}, cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if publisher == nil {
		t.Fatalf("expected publisher instance")
	}
}

func TestNewJobPublisherValidation(t *testing.T) {
	_, err := NewJobPublisher(nil, Config{URL: "amqp://guest:guest@localhost:5672/", JobQueue: "jobs"})
	if err == nil || !strings.Contains(err.Error(), "mq client is required") {
		t.Fatalf("expected missing client error, got %v", err)
	}

	_, err = NewJobPublisher(&fakeRawPublisher{}, Config{})
	if err == nil || !strings.Contains(err.Error(), "rabbitmq url is required") {
		t.Fatalf("expected invalid config error, got %v", err)
	}
}

func TestJobPublisherPublishJob(t *testing.T) {
	cfg := Config{URL: "amqp://guest:guest@localhost:5672/", JobQueue: "jobs.queue"}
	msg := messages.JobExecutionMessage{
		RunNo:         22,
		Pipeline:      "release",
		Stage:         "test",
		Branch:        "main",
		Commit:        "abc123",
		WorkspacePath: "/tmp/work",
		Job: models.JobExecutionPlan{
			Name:   "unit-tests",
			Image:  "golang:1.25",
			Script: []string{"go test ./..."},
		},
	}

	var gotCtx context.Context
	var gotQueue string
	var gotBody []byte
	publisherClient := &fakeRawPublisher{
		publishFn: func(ctx context.Context, queue string, body []byte) error {
			gotCtx = ctx
			gotQueue = queue
			gotBody = append([]byte(nil), body...)
			return nil
		},
	}

	publisher, err := NewJobPublisher(publisherClient, cfg)
	if err != nil {
		t.Fatalf("NewJobPublisher error: %v", err)
	}

	ctx := context.WithValue(context.Background(), publisherTestContextKey("request_id"), "req-1")
	if err := publisher.PublishJob(ctx, msg); err != nil {
		t.Fatalf("PublishJob returned error: %v", err)
	}
	if gotCtx != ctx {
		t.Fatalf("expected publisher to pass context through")
	}
	if gotQueue != cfg.JobQueue {
		t.Fatalf("expected queue %q, got %q", cfg.JobQueue, gotQueue)
	}

	var decoded messages.JobExecutionMessage
	if err := json.Unmarshal(gotBody, &decoded); err != nil {
		t.Fatalf("unmarshal published body: %v", err)
	}
	if !reflect.DeepEqual(decoded, msg) {
		t.Fatalf("expected published message %+v, got %+v", msg, decoded)
	}
}

func TestJobPublisherPublishJobWrappedError(t *testing.T) {
	rawErr := errors.New("publish failed")
	publisherClient := &fakeRawPublisher{
		publishFn: func(context.Context, string, []byte) error {
			return rawErr
		},
	}
	publisher, err := NewJobPublisher(publisherClient, Config{URL: "amqp://guest:guest@localhost:5672/", JobQueue: "jobs"})
	if err != nil {
		t.Fatalf("NewJobPublisher error: %v", err)
	}

	err = publisher.PublishJob(context.Background(), messages.JobExecutionMessage{Pipeline: "demo", Stage: "build", Job: models.JobExecutionPlan{Name: "lint"}})
	if err == nil {
		t.Fatalf("expected wrapped publish error")
	}
	if !strings.Contains(err.Error(), "publish job execution message") {
		t.Fatalf("expected publish context in error, got %v", err)
	}
	if !errors.Is(err, rawErr) {
		t.Fatalf("expected wrapped raw error, got %v", err)
	}
}

func TestJobPublisherClose(t *testing.T) {
	rawErr := errors.New("close failed")
	publisher := &JobPublisher{client: &fakeRawPublisher{closeErr: rawErr}}

	err := publisher.Close()
	if !errors.Is(err, rawErr) {
		t.Fatalf("expected close error %v, got %v", rawErr, err)
	}

	var nilPublisher *JobPublisher
	if err := nilPublisher.Close(); err != nil {
		t.Fatalf("expected nil close error for nil publisher, got %v", err)
	}
}
