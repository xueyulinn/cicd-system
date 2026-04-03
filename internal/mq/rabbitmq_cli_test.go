package mq

import (
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
