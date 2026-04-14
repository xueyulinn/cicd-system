package execution

import (
	"context"
	"errors"
	"testing"

	"github.com/CS7580-SEA-SP26/e-team/internal/messages"
	"github.com/CS7580-SEA-SP26/e-team/internal/mq"
	amqp "github.com/rabbitmq/amqp091-go"
)

type fakePublisher struct {
	closed bool
}

func (f *fakePublisher) PublishJob(_ context.Context, _ messages.JobExecutionMessage) error {
	return nil
}

func (f *fakePublisher) Close() error {
	f.closed = true
	return nil
}

func TestLoadPublisherConcurrency_DefaultWhenUnset(t *testing.T) {
	t.Setenv("PUBLISHER_CONCURRENCY", "")

	got := loadPublisherConcurrency()
	if got != defaultPublisherConcurrent {
		t.Fatalf("loadPublisherConcurrency() = %d, want %d", got, defaultPublisherConcurrent)
	}
}

func TestLoadPublisherConcurrency_ValidValue(t *testing.T) {
	t.Setenv("PUBLISHER_CONCURRENCY", "3")

	got := loadPublisherConcurrency()
	if got != 3 {
		t.Fatalf("loadPublisherConcurrency() = %d, want 3", got)
	}
}

func TestLoadPublisherConcurrency_InvalidFallsBack(t *testing.T) {
	t.Setenv("PUBLISHER_CONCURRENCY", "abc")

	got := loadPublisherConcurrency()
	if got != defaultPublisherConcurrent {
		t.Fatalf("loadPublisherConcurrency() = %d, want %d", got, defaultPublisherConcurrent)
	}
}

func TestCreateJobPublishers_CreatesRequestedCount(t *testing.T) {
	originalFactory := newJobPublisher
	defer func() { newJobPublisher = originalFactory }()

	created := 0
	newJobPublisher = func(cfg mq.Config, _ *amqp.Connection) (mq.Publisher, error) {
		created++
		return &fakePublisher{}, nil
	}

	publishers, err := createJobPublishers(mq.Config{URL: "amqp://x", JobQueue: "q"}, nil, 2)
	if err != nil {
		t.Fatalf("createJobPublishers returned error: %v", err)
	}
	if len(publishers) != 2 {
		t.Fatalf("len(publishers) = %d, want 2", len(publishers))
	}
	if created != 2 {
		t.Fatalf("created = %d, want 2", created)
	}
}

func TestCreateJobPublishers_ClosesAlreadyCreatedOnFailure(t *testing.T) {
	originalFactory := newJobPublisher
	defer func() { newJobPublisher = originalFactory }()

	first := &fakePublisher{}
	call := 0
	newJobPublisher = func(cfg mq.Config, _ *amqp.Connection) (mq.Publisher, error) {
		call++
		if call == 1 {
			return first, nil
		}
		return nil, errors.New("boom")
	}

	publishers, err := createJobPublishers(mq.Config{URL: "amqp://x", JobQueue: "q"}, nil, 2)
	if err == nil {
		t.Fatal("createJobPublishers error = nil, want non-nil")
	}
	if publishers != nil {
		t.Fatalf("publishers = %v, want nil", publishers)
	}
	if !first.closed {
		t.Fatal("expected first publisher to be closed on initialization failure")
	}
}

func TestCreateJobPublishers_InvalidCount(t *testing.T) {
	publishers, err := createJobPublishers(mq.Config{URL: "amqp://x", JobQueue: "q"}, nil, 0)
	if err == nil {
		t.Fatal("createJobPublishers error = nil, want non-nil")
	}
	if publishers != nil {
		t.Fatalf("publishers = %v, want nil", publishers)
	}
}
