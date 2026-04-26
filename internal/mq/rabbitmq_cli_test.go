package mq

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRabbitClientEnsureQueueValidation(t *testing.T) {
	var nilClient *RabbitClient
	err := nilClient.ensureQueue("jobs")
	if err == nil || !strings.Contains(err.Error(), "rabbit client is nil") {
		t.Fatalf("expected nil client error, got %v", err)
	}

	client := &RabbitClient{}
	err = client.ensureQueue("")
	if err == nil || !strings.Contains(err.Error(), "queue is required") {
		t.Fatalf("expected missing queue error, got %v", err)
	}
}

type fakePublishConfirmationWaiter struct {
	acked bool
	err   error
}

func (f fakePublishConfirmationWaiter) WaitContext(context.Context) (bool, error) {
	return f.acked, f.err
}

func TestWaitPublishConfirm_Acked(t *testing.T) {
	err := waitPublishConfirm(context.Background(), fakePublishConfirmationWaiter{acked: true})
	if err != nil {
		t.Fatalf("waitPublishConfirm returned error: %v", err)
	}
}

func TestWaitPublishConfirm_Nack(t *testing.T) {
	err := waitPublishConfirm(context.Background(), fakePublishConfirmationWaiter{acked: false})
	if err == nil || !strings.Contains(err.Error(), "nacked") {
		t.Fatalf("expected nack error, got %v", err)
	}
}

func TestWaitPublishConfirm_WaitError(t *testing.T) {
	err := waitPublishConfirm(context.Background(), fakePublishConfirmationWaiter{err: errors.New("timeout")})
	if err == nil || !strings.Contains(err.Error(), "wait publish confirmation") {
		t.Fatalf("expected wait error, got %v", err)
	}
}
