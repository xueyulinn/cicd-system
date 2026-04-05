package mq

import (
	"strings"
	"testing"
)

func TestLoadConfigDefaults(t *testing.T) {
	t.Setenv("RABBITMQ_URL", "")
	t.Setenv("RABBITMQ_JOB_QUEUE", "")

	cfg := LoadConfig()
	if cfg.URL != defaultRabbitURL {
		t.Fatalf("expected default URL %q, got %q", defaultRabbitURL, cfg.URL)
	}
	if cfg.JobQueue != defaultJobQueue {
		t.Fatalf("expected default queue %q, got %q", defaultJobQueue, cfg.JobQueue)
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	t.Setenv("RABBITMQ_URL", "  amqp://guest:guest@rabbit:5672/ ")
	t.Setenv("RABBITMQ_JOB_QUEUE", " jobs.custom ")

	cfg := LoadConfig()
	if cfg.URL != "amqp://guest:guest@rabbit:5672/" {
		t.Fatalf("expected trimmed URL from env, got %q", cfg.URL)
	}
	if cfg.JobQueue != "jobs.custom" {
		t.Fatalf("expected trimmed queue from env, got %q", cfg.JobQueue)
	}
}

func TestConfigValidate(t *testing.T) {
	if err := (Config{URL: "amqp://guest:guest@localhost:5672/", JobQueue: "jobs"}).Validate(); err != nil {
		t.Fatalf("expected valid config, got %v", err)
	}

	err := (Config{URL: " ", JobQueue: "jobs"}).Validate()
	if err == nil || !strings.Contains(err.Error(), "rabbitmq url is required") {
		t.Fatalf("expected URL required error, got %v", err)
	}

	err = (Config{URL: "amqp://guest:guest@localhost:5672/", JobQueue: "  "}).Validate()
	if err == nil || !strings.Contains(err.Error(), "rabbitmq job queue is required") {
		t.Fatalf("expected queue required error, got %v", err)
	}
}
