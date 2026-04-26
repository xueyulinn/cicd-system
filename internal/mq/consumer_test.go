package mq

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/xueyulinn/cicd-system/internal/messages"
	"github.com/xueyulinn/cicd-system/internal/models"
)

type consumerTestContextKey string

type fakeRawConsumer struct {
	consumeFn func(context.Context, string, func(context.Context, []byte) error) error
	closeErr  error
}

func (f *fakeRawConsumer) Consume(ctx context.Context, queue string, handler func(context.Context, []byte) error) error {
	if f.consumeFn != nil {
		return f.consumeFn(ctx, queue, handler)
	}
	return nil
}

func (f *fakeRawConsumer) Close() error {
	return f.closeErr
}

func TestNewJobConsumer(t *testing.T) {
	cfg := Config{URL: "amqp://guest:guest@localhost:5672/", JobQueue: "jobs"}

	consumer, err := NewJobConsumer(&fakeRawConsumer{}, cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if consumer == nil {
		t.Fatalf("expected consumer instance")
	}
}

func TestNewJobConsumerValidation(t *testing.T) {
	_, err := NewJobConsumer(nil, Config{URL: "amqp://guest:guest@localhost:5672/", JobQueue: "jobs"})
	if err == nil || !strings.Contains(err.Error(), "mq client is required") {
		t.Fatalf("expected missing client error, got %v", err)
	}

	_, err = NewJobConsumer(&fakeRawConsumer{}, Config{})
	if err == nil || !strings.Contains(err.Error(), "rabbitmq url is required") {
		t.Fatalf("expected invalid config error, got %v", err)
	}
}

func TestJobConsumerConsumeJob(t *testing.T) {
	cfg := Config{URL: "amqp://guest:guest@localhost:5672/", JobQueue: "jobs.queue"}
	expected := messages.JobExecutionMessage{
		RunNo:        7,
		PipelineName: "demo",
		Stage:        "build",
		Job: models.JobExecutionPlan{
			Name:   "lint",
			Image:  "golang:1.25",
			Script: []string{"go test ./..."},
		},
	}
	body, err := json.Marshal(expected)
	if err != nil {
		t.Fatalf("marshal expected message: %v", err)
	}

	handlerCtx := context.WithValue(context.Background(), consumerTestContextKey("trace"), "child")
	var gotQueue string
	var gotMsg messages.JobExecutionMessage
	called := false

	raw := &fakeRawConsumer{
		consumeFn: func(ctx context.Context, queue string, handler func(context.Context, []byte) error) error {
			gotQueue = queue
			return handler(handlerCtx, body)
		},
	}

	consumer, err := NewJobConsumer(raw, cfg)
	if err != nil {
		t.Fatalf("NewJobConsumer error: %v", err)
	}

	err = consumer.ConsumeJob(context.Background(), func(ctx context.Context, msg messages.JobExecutionMessage) error {
		called = true
		if ctx != handlerCtx {
			t.Fatalf("expected handler context from raw consumer")
		}
		gotMsg = msg
		return nil
	})
	if err != nil {
		t.Fatalf("ConsumeJob returned error: %v", err)
	}
	if gotQueue != cfg.JobQueue {
		t.Fatalf("expected queue %q, got %q", cfg.JobQueue, gotQueue)
	}
	if !called {
		t.Fatalf("expected job handler to be called")
	}
	if !reflect.DeepEqual(gotMsg, expected) {
		t.Fatalf("expected message %+v, got %+v", expected, gotMsg)
	}
}

func TestJobConsumerConsumeJobValidation(t *testing.T) {
	consumer := &JobConsumer{client: &fakeRawConsumer{}, queue: "jobs"}

	var nilCtx context.Context
	err := consumer.ConsumeJob(nilCtx, func(context.Context, messages.JobExecutionMessage) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "context is required") {
		t.Fatalf("expected missing context error, got %v", err)
	}

	err = consumer.ConsumeJob(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "job handler is required") {
		t.Fatalf("expected missing handler error, got %v", err)
	}

	var nilConsumer *JobConsumer
	err = nilConsumer.ConsumeJob(context.Background(), func(context.Context, messages.JobExecutionMessage) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "job consumer is not initialized") {
		t.Fatalf("expected uninitialized consumer error, got %v", err)
	}
}

func TestJobConsumerConsumeJobUnmarshalError(t *testing.T) {
	raw := &fakeRawConsumer{
		consumeFn: func(ctx context.Context, queue string, handler func(context.Context, []byte) error) error {
			return handler(ctx, []byte("not-json"))
		},
	}
	consumer, err := NewJobConsumer(raw, Config{URL: "amqp://guest:guest@localhost:5672/", JobQueue: "jobs"})
	if err != nil {
		t.Fatalf("NewJobConsumer error: %v", err)
	}

	err = consumer.ConsumeJob(context.Background(), func(context.Context, messages.JobExecutionMessage) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "unmarshal job execution message") {
		t.Fatalf("expected unmarshal error, got %v", err)
	}
}

func TestJobConsumerConsumeJobWrappedConsumeError(t *testing.T) {
	rawErr := errors.New("consume failed")
	raw := &fakeRawConsumer{
		consumeFn: func(context.Context, string, func(context.Context, []byte) error) error {
			return rawErr
		},
	}
	consumer, err := NewJobConsumer(raw, Config{URL: "amqp://guest:guest@localhost:5672/", JobQueue: "jobs"})
	if err != nil {
		t.Fatalf("NewJobConsumer error: %v", err)
	}

	err = consumer.ConsumeJob(context.Background(), func(context.Context, messages.JobExecutionMessage) error { return nil })
	if err == nil {
		t.Fatalf("expected wrapped consume error")
	}
	if !strings.Contains(err.Error(), "consume jobs from queue \"jobs\"") {
		t.Fatalf("expected queue context in error, got %v", err)
	}
	if !errors.Is(err, rawErr) {
		t.Fatalf("expected wrapped raw error, got %v", err)
	}
}

func TestJobConsumerClose(t *testing.T) {
	rawErr := errors.New("close failed")
	consumer := &JobConsumer{client: &fakeRawConsumer{closeErr: rawErr}}

	err := consumer.Close()
	if !errors.Is(err, rawErr) {
		t.Fatalf("expected close error %v, got %v", rawErr, err)
	}

	var nilConsumer *JobConsumer
	if err := nilConsumer.Close(); err != nil {
		t.Fatalf("expected nil close error for nil consumer, got %v", err)
	}
}
