package worker

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/xueyulinn/cicd-system/internal/messages"
	"github.com/xueyulinn/cicd-system/internal/mq"
	amqp "github.com/rabbitmq/amqp091-go"
)

type fakeConsumer struct {
	closed bool
}

func (f *fakeConsumer) ConsumeJob(context.Context, func(context.Context, messages.JobExecutionMessage) error) error {
	return nil
}

func (f *fakeConsumer) Close() error {
	f.closed = true
	return nil
}

func TestLoadWorkerConcurrency_DefaultWhenUnset(t *testing.T) {
	t.Setenv("WORKER_CONCURRENCY", "")

	got := loadWorkerConcurrency()
	if got != defaultWorkerConcurrent {
		t.Fatalf("loadWorkerConcurrency() = %d, want %d", got, defaultWorkerConcurrent)
	}
}

func TestLoadWorkerConcurrency_ValidValue(t *testing.T) {
	t.Setenv("WORKER_CONCURRENCY", "3")

	got := loadWorkerConcurrency()
	if got != 3 {
		t.Fatalf("loadWorkerConcurrency() = %d, want 3", got)
	}
}

func TestLoadWorkerConcurrency_InvalidFallsBack(t *testing.T) {
	t.Setenv("WORKER_CONCURRENCY", "abc")

	got := loadWorkerConcurrency()
	if got != defaultWorkerConcurrent {
		t.Fatalf("loadWorkerConcurrency() = %d, want %d", got, defaultWorkerConcurrent)
	}
}

func TestCreateJobConsumers_CreatesRequestedCount(t *testing.T) {
	t.Setenv("WORKER_CONCURRENCY", "2")

	originalFactory := newJobConsumer
	defer func() { newJobConsumer = originalFactory }()

	created := 0
	newJobConsumer = func(cfg mq.Config, _ *amqp.Connection) (mq.Consumer, error) {
		created++
		return &fakeConsumer{}, nil
	}

	consumers, err := createJobConsumers(mq.Config{URL: "amqp://x", JobQueue: "q"}, nil)
	if err != nil {
		t.Fatalf("createJobConsumers returned error: %v", err)
	}
	if len(consumers) != 2 {
		t.Fatalf("len(consumers) = %d, want 2", len(consumers))
	}
	if created != 2 {
		t.Fatalf("created = %d, want 2", created)
	}
}

func TestCreateJobConsumers_ClosesAlreadyCreatedOnFailure(t *testing.T) {
	t.Setenv("WORKER_CONCURRENCY", "2")

	originalFactory := newJobConsumer
	defer func() { newJobConsumer = originalFactory }()

	first := &fakeConsumer{}
	call := 0
	newJobConsumer = func(cfg mq.Config, _ *amqp.Connection) (mq.Consumer, error) {
		call++
		if call == 1 {
			return first, nil
		}
		return nil, errors.New("boom")
	}

	consumers, err := createJobConsumers(mq.Config{URL: "amqp://x", JobQueue: "q"}, nil)
	if err == nil {
		t.Fatal("createJobConsumers error = nil, want non-nil")
	}
	if consumers != nil {
		t.Fatalf("consumers = %v, want nil", consumers)
	}
	if !first.closed {
		t.Fatal("expected first consumer to be closed on initialization failure")
	}
}

func TestCreateJobConsumers_InvalidEnvFallsBackToDefault(t *testing.T) {
	t.Setenv("WORKER_CONCURRENCY", "0")

	originalFactory := newJobConsumer
	defer func() { newJobConsumer = originalFactory }()

	created := 0
	newJobConsumer = func(cfg mq.Config, _ *amqp.Connection) (mq.Consumer, error) {
		created++
		return &fakeConsumer{}, nil
	}

	consumers, err := createJobConsumers(mq.Config{URL: "amqp://x", JobQueue: "q"}, nil)
	if err != nil {
		t.Fatalf("createJobConsumers returned error: %v", err)
	}
	if len(consumers) != defaultWorkerConcurrent {
		t.Fatalf("len(consumers) = %d, want %d", len(consumers), defaultWorkerConcurrent)
	}
	if created != defaultWorkerConcurrent {
		t.Fatalf("created = %d, want %d", created, defaultWorkerConcurrent)
	}
}

func TestMain(m *testing.M) {
	// Ensure this package test process does not inherit accidental concurrency overrides.
	_ = os.Unsetenv("WORKER_CONCURRENCY")
	os.Exit(m.Run())
}
