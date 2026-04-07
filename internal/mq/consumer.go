package mq

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/CS7580-SEA-SP26/e-team/internal/messages"
)

// Service level abstraction
type Consumer interface {
	ConsumeJob(ctx context.Context, handler func(context.Context, messages.JobExecutionMessage) error) error
	Close() error
}

// RawPublisher is the low-level transport abstraction implemented by a
// RabbitMQ client. It publishes an already-serialized payload to a queue.
type RawConsumer interface {
	Consume(ctx context.Context, queue string, handler func(context.Context, []byte) error) error
	Close() error
}

// JobConsumer implements Consumer and delegates queue consumption to the underlying MQ client.
type JobConsumer struct {
	client RawConsumer
	queue  string
}

func NewJobConsumer(client RawConsumer, cfg Config) (*JobConsumer, error) {
	if client == nil {
		return nil, fmt.Errorf("mq client is required")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &JobConsumer{
		client: client,
		queue:  cfg.JobQueue,
	}, nil
}

func (c *JobConsumer) ConsumeJob(ctx context.Context, handler func(context.Context, messages.JobExecutionMessage) error) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("job consumer is not initialized")
	}
	if ctx == nil {
		return fmt.Errorf("context is required")
	}
	if handler == nil {
		return fmt.Errorf("job handler is required")
	}

	if err := c.client.Consume(ctx, c.queue, func(handlerCtx context.Context, body []byte) error {
		var msg messages.JobExecutionMessage
		if err := json.Unmarshal(body, &msg); err != nil {
			return fmt.Errorf("unmarshal job execution message: %w", err)
		}
		return handler(handlerCtx, msg)
	}); err != nil {
		return fmt.Errorf("consume jobs from queue %q: %w", c.queue, err)
	}
	return nil
}

func (c *JobConsumer) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}
