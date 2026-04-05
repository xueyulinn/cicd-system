package mq

import (
	"context"
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitClient is the low-level RabbitMQ transport used by higher-level
// publishers. Wire the AMQP connection/channel into this type when
// integrating the broker library.
type RabbitClient struct {
	cfg     Config
	conn    *amqp.Connection
	channel *amqp.Channel
}

func NewRabbitClient(cfg Config) (*RabbitClient, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	conn, err := amqp.Dial(cfg.URL)

	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	client := &RabbitClient{cfg: cfg, conn: conn, channel: ch}
	if err := client.ensureQueue(cfg.JobQueue); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

// create queue if not exists
func (c *RabbitClient) ensureQueue(queue string) error {
	if c == nil {
		return fmt.Errorf("rabbit client is nil")
	}
	if queue == "" {
		return fmt.Errorf("queue is required")
	}

	_, err := c.channel.QueueDeclare(
		queue,
		true,
		false,
		false,
		false,
		nil,
	)
	return err
}

// Consume starts consuming messages from the given queue and invokes handler for
// each delivery. Successful handling acks the message; failures nack it.
func (c *RabbitClient) Consume(ctx context.Context, queue string, handler func(context.Context, []byte) error) error {
	if c == nil {
		return fmt.Errorf("rabbit client is nil")
	}
	if handler == nil {
		return fmt.Errorf("consumer handler is required")
	}
	if err := c.ensureQueue(queue); err != nil {
		return fmt.Errorf("declare queue %q: %w", queue, err)
	}

	deliveries, err := c.channel.Consume(
		queue,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("start consuming queue %q: %w", queue, err)
	}

	for {
		// select concurrency pattern evaluates all channels and blocks until one case is ready
		select {
		case <-ctx.Done():
			return ctx.Err()
		case delivery, ok := <-deliveries:
			if !ok {
				return fmt.Errorf("delivery channel closed for queue %q", queue)
			}
			
			if err := handler(ctx, delivery.Body); err != nil {
				_ = delivery.Nack(false, true)
				log.Printf("[mq] nack delivery from queue=%s err=%v", queue, err)
				continue
			}
			if err := delivery.Ack(false); err != nil {
				return fmt.Errorf("ack delivery from queue %q: %w", queue, err)
			}
		}
	}
}

// implementation of Publish() for RawJobPublisher
func (c *RabbitClient) Publish(ctx context.Context, queue string, body []byte) error {
	if c == nil {
		return fmt.Errorf("rabbit client is nil")
	}
	if err := c.ensureQueue(queue); err != nil {
		return fmt.Errorf("declare queue %q: %w", queue, err)
	}

	err := c.channel.PublishWithContext(ctx, "", queue, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        body,
	})
	if err != nil {
		return fmt.Errorf("publish message failed: %w", err)
	}
	return nil
}

func (c *RabbitClient) Close() error {
	if c == nil {
		return nil
	}

	if c.channel != nil {
		_ = c.channel.Close()
	}

	if c.conn != nil {
		_ = c.conn.Close()
	}
	return nil
}

// Check if mq is ready
func PingMQ(ctx context.Context, cfg Config) error{
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("mq config is invalid: %w", err)
	}
	
	conn, err := amqp.Dial(cfg.URL)
	
	if err != nil {
		return fmt.Errorf("failed to connect with mq instance: %w", err)
	}

	defer func() { _ = conn.Close() }()

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("failed to open channel: %w", err)
	}

	defer func() { _ = ch.Close() }()

	 _, err = ch.QueueDeclarePassive(cfg.JobQueue, true, false, false, false, nil)

	if err != nil {
		return fmt.Errorf("queue %q not ready: %w", cfg.JobQueue, err)
	}
	return nil
}
