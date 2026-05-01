package orchestrator

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/xueyulinn/cicd-system/internal/messages"
	"github.com/xueyulinn/cicd-system/internal/mq"
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

func TestCreateJobPublishers_InvalidCount(t *testing.T) {
	publishers, err := createJobPublishers(mq.Config{URL: "amqp://x", JobQueue: "q"}, nil, 0)
	if err == nil {
		t.Fatal("createJobPublishers error = nil, want non-nil")
	}
	if publishers != nil {
		t.Fatalf("publishers = %v, want nil", publishers)
	}
}

func TestPublisherManagerCloseClosesIdlePublisherSets(t *testing.T) {
	currentPub := &fakePublisher{}
	stalePub := &fakePublisher{}
	manager := &publisherManager{
		current: &publisherSet{
			gen:  1,
			pubs: []mq.Publisher{currentPub},
		},
		stale: map[int]*publisherSet{
			2: {
				gen:     2,
				pubs:    []mq.Publisher{stalePub},
				retired: atomic.Bool{},
			},
		},
	}
	manager.stale[2].retired.Store(true)

	if err := manager.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if manager.current != nil {
		t.Fatal("expected current publisher set to be cleared")
	}
	if !currentPub.closed {
		t.Fatal("expected idle current publisher set to be closed")
	}
	if !stalePub.closed {
		t.Fatal("expected idle stale publisher set to be closed")
	}
	if len(manager.stale) != 0 {
		t.Fatalf("expected stale sets to be drained, got %d", len(manager.stale))
	}
}

func TestPublisherManagerCloseDefersBusyPublisherSetUntilRelease(t *testing.T) {
	currentPub := &fakePublisher{}
	currentSet := &publisherSet{
		gen:  1,
		pubs: []mq.Publisher{currentPub},
	}
	currentSet.refs.Add(1)

	manager := &publisherManager{
		current: currentSet,
		stale:   make(map[int]*publisherSet),
	}

	if err := manager.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if manager.current != nil {
		t.Fatal("expected current publisher set to be cleared")
	}
	if !currentSet.retired.Load() {
		t.Fatal("expected current publisher set to be retired")
	}
	if currentPub.closed {
		t.Fatal("expected busy publisher set to stay open until release")
	}
	if got := manager.stale[currentSet.gen]; got != currentSet {
		t.Fatal("expected busy publisher set to remain tracked as stale")
	}

	manager.releaseSet(currentSet)

	if !currentPub.closed {
		t.Fatal("expected busy publisher set to close after final release")
	}
	if _, ok := manager.stale[currentSet.gen]; ok {
		t.Fatal("expected stale publisher set to be removed after close")
	}
}
