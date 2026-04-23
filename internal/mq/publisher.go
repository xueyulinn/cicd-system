package mq

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/xueyulinn/cicd-system/internal/messages"
)

// Publisher publishes job messages for worker consumption.
type Publisher interface {
	PublishJob(ctx context.Context, msg messages.JobExecutionMessage) error
	Close() error
}

// RawPublisher is the low-level transport abstraction implemented by a
// RabbitMQ client. It publishes an already-serialized payload to a queue.
type RawPublisher interface {
	Publish(ctx context.Context, queue string, body []byte) error
	Close() error
}

// JobPublisher serializes job execution messages and sends them to the job queue.
type JobPublisher struct {
	client RawPublisher
	queue  string
}

func NewJobPublisher(client RawPublisher, cfg Config) (*JobPublisher, error) {
	if client == nil {
		return nil, fmt.Errorf("mq client is required")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &JobPublisher{
		client: client,
		queue:  cfg.JobQueue,
	}, nil
}

func (p *JobPublisher) PublishJob(ctx context.Context, msg messages.JobExecutionMessage) error {
	body, err := json.Marshal(msg)
	if err != nil {
		// observability.RecordMQJobPublished(p.queue, false)
		return fmt.Errorf("marshal job execution message: %w", err)
	}

	if err := p.client.Publish(ctx, p.queue, body); err != nil {
		// observability.RecordMQJobPublished(p.queue, false)
		return fmt.Errorf("publish job execution message: %w", err)
	}
	// observability.RecordMQJobPublished(p.queue, true)
	return nil
}

func (p *JobPublisher) Close() error {
	if p == nil || p.client == nil {
		return nil
	}
	return p.client.Close()
}
