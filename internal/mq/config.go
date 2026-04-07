package mq

import (
	"fmt"
	"os"
	"strings"
)

const (
	defaultRabbitURL = "amqp://guest:guest@localhost:5672/"
	defaultJobQueue  = "pipeline.jobs"
)

// Config contains RabbitMQ connection settings used by publishers/consumers.
type Config struct {
	URL      string
	JobQueue string
}

// LoadConfig reads MQ configuration from environment variables.
func LoadConfig() Config {
	url := strings.TrimSpace(os.Getenv("RABBITMQ_URL"))
	if url == "" {
		url = defaultRabbitURL
	}

	jobQueue := strings.TrimSpace(os.Getenv("RABBITMQ_JOB_QUEUE")) 
	if jobQueue == "" {
		jobQueue = defaultJobQueue
	}

	return Config{
		URL: url,
		JobQueue: jobQueue,
	}
}

// Validate checks whether required MQ configuration is present.
func(c Config) Validate() error {
	if strings.TrimSpace(c.URL) == "" {
		return fmt.Errorf("rabbitmq url is required")
	}
	if strings.TrimSpace(c.JobQueue) == "" {
		return fmt.Errorf("rabbitmq job queue is required")
	}
	return nil
}