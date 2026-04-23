package mq

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitClient is the low-level RabbitMQ transport used by higher-level
// publishers. Wire the AMQP connection/channel into this type when
// integrating the broker library.
type RabbitClient struct {
	cfg     Config
	conn    *amqp.Connection
	channel *amqp.Channel
	mu      sync.Mutex
}

const (
	reconnectDelay     = 1 * time.Second
	maxPublishAttempts = 3
)

func NewRabbitClientWithConn(cfg Config, conn *amqp.Connection) (*RabbitClient, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFatal, err)
	}
	if conn == nil {
		return nil, ErrConnectionClosed
	}
	client := &RabbitClient{cfg: cfg, conn: conn}
	if err := client.reopenChannel(); err != nil {
		return nil, err
	}
	if err := client.ensureQueue(cfg.JobQueue); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

// create queue if not exists
func (c *RabbitClient) ensureQueue(queue string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ensureQueueLocked(queue)
}

func (c *RabbitClient) ensureQueueLocked(queue string) error {
	if queue == "" {
		return fmt.Errorf("queue is required")
	}
	if c.channel == nil {
		return fmt.Errorf("rabbit channel is nil")
	}

	// when the error return value is not nil, the queue could not be declared with these parameters, and the channel will be closed.
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
		return fmt.Errorf("job consume handler is nil")
	}

	for {
		deliveries, err := c.startConsume(queue)
		if err != nil {
			log.Printf("[mq] consumer declared queue failed, queue=%s err=%v; reconnecting", queue, err)
			if err := c.reconnectAndWait(ctx, queue); err != nil {
				return err
			}
			continue
		}

		// select concurrency pattern evaluates all channels and blocks until one case is ready
		restartConsume := false
		for !restartConsume {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case delivery, ok := <-deliveries:
				if !ok {
					log.Printf("[mq] delivery channel closed queue=%s; reconnecting", queue)
					restartConsume = true
					break
				}

				if err := handler(ctx, delivery.Body); err != nil {
					// observability.RecordMQDeliveryOutcome(queue, "nack_requeue")
					_ = delivery.Nack(false, true)
					log.Printf("[mq] nack delivery from queue=%s err=%v", queue, err)
					continue
				}

				if err := delivery.Ack(false); err != nil {
					// observability.RecordMQDeliveryOutcome(queue, "ack_error")
					log.Printf("[mq] ack delivery failed queue=%s err=%v; reconnecting", queue, err)
					restartConsume = true
					break
				}
				// observability.RecordMQDeliveryOutcome(queue, "acked")
			}
		}

		if err := c.reconnectAndWait(ctx, queue); err != nil {
			return err
		}
	}
}

// reopens channel and reports err if connection lost
func (c *RabbitClient) reconnectAndWait(ctx context.Context, queue string) error {
	if recErr := c.reconnect(); recErr != nil {
		switch classifyError(recErr) {
		case ConnLost, Fatal:
			return fmt.Errorf("consume queue %q: %w", queue, recErr)
		case CtxDone:
			return recErr
		case Retryable:
			log.Printf("[mq] reconnect failed queue=%s err=%v", queue, recErr)
		}
	}
	if err := sleepWithContext(ctx, reconnectDelay); err != nil {
		return err
	}
	return nil
}

// implementation of Publish() for RawJobPublisher
func (c *RabbitClient) Publish(ctx context.Context, queue string, body []byte) error {
	if c == nil {
		return fmt.Errorf("rabbit client is nil")
	}

	var lastErr error
	for attempt := 1; attempt <= maxPublishAttempts; attempt++ {
		if err := c.publishOnce(ctx, queue, body); err == nil {
			return nil
		} else {
			lastErr = err
			log.Printf("[mq] publish failed attempt=%d/%d queue=%s err=%v", attempt, maxPublishAttempts, queue, err)
		}

		if attempt == maxPublishAttempts {
			break
		}
		if recErr := c.reconnect(); recErr != nil {
			log.Printf("[mq] reconnect failed queue=%s err=%v", queue, recErr)
			lastErr = fmt.Errorf("%w; reconnect: %v", lastErr, recErr)
		}
		if err := sleepWithContext(ctx, reconnectDelay); err != nil {
			return err
		}
	}
	return fmt.Errorf("publish message failed after %d attempts: %w", maxPublishAttempts, lastErr)
}

func (c *RabbitClient) publishOnce(ctx context.Context, queue string, body []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.ensureQueueLocked(queue); err != nil {
		return fmt.Errorf("declare queue %q: %w", queue, err)
	}

	err := c.channel.PublishWithContext(ctx, "", queue, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        body,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *RabbitClient) startConsume(queue string) (<-chan amqp.Delivery, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.ensureQueueLocked(queue); err != nil {
		return nil, fmt.Errorf("declare queue %q: %w", queue, err)
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
		return nil, fmt.Errorf("start consuming queue %q: %w", queue, err)
	}
	return deliveries, nil
}

func (c *RabbitClient) reconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.cfg.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrFatal, err)
	}

	if c.conn == nil || c.conn.IsClosed() {
		return ErrConnectionClosed
	}
	return c.reopenChannelLocked()
}

func (c *RabbitClient) reopenChannel() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.reopenChannelLocked()
}

func (c *RabbitClient) reopenChannelLocked() error {
	if c.conn == nil || c.conn.IsClosed() {
		return ErrConnectionClosed
	}
	if c.channel != nil {
		_ = c.channel.Close()
		c.channel = nil
	}
	ch, err := c.conn.Channel()
	if err != nil {
		return err
	}
	c.channel = ch
	return nil
}

func (c *RabbitClient) Close() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.channel != nil {
		_ = c.channel.Close()
		c.channel = nil
	}

	return nil
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// Check if mq is ready
func PingMQ(ctx context.Context, cfg Config) error {
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
