package mq

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

var prop = otel.GetTextMapPropagator()

type amqpHeaderCarrier struct {
	headers amqp.Table
}

func (c amqpHeaderCarrier) Get(key string) string {
	if v, ok := c.headers[key]; ok {
		switch t := v.(type) {
		case string:
			return t
		case []byte:
			return string(t)
		default:
			return fmt.Sprint(t)
		}
	}
	return ""
}

func (c amqpHeaderCarrier) Set(key, value string) {
	c.headers[key] = value
}

func (c amqpHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(c.headers))
	for k := range c.headers {
		keys = append(keys, k)
	}
	return keys
}

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
					slog.Warn("mq delivery channel closed; reconnecting",
						"queue", queue,
					)
					restartConsume = true
					break
				}

				deliveryCtx := prop.Extract(ctx, amqpHeaderCarrier{headers: delivery.Headers})
				deliveryCtx, span := tracer.Start(
					deliveryCtx,
					"consume.message",
					trace.WithSpanKind(trace.SpanKindConsumer),
				)

				if err := handler(deliveryCtx, delivery.Body); err != nil {
					span.End()
					// observability.RecordMQDeliveryOutcome(queue, "nack_requeue")
					_ = delivery.Nack(false, true)
					slog.Warn("mq nack delivery",
						"queue", queue,
						"error", err,
					)
					continue
				}

				if err := delivery.Ack(false); err != nil {
					span.End()
					// observability.RecordMQDeliveryOutcome(queue, "ack_error")
					slog.Warn("mq ack delivery failed; reconnecting",
						"queue", queue,
						"error", err,
					)
					restartConsume = true
					continue
				}
				span.End()
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
			slog.Warn("mq reconnect failed",
				"queue", queue,
				"error", recErr,
			)
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
			slog.Warn("mq publish failed",
				"attempt", attempt,
				"max_attempts", maxPublishAttempts,
				"queue", queue,
				"error", err,
			)
		}

		if attempt == maxPublishAttempts {
			break
		}
		if recErr := c.reconnect(); recErr != nil {
			slog.Warn("mq reconnect failed",
				"queue", queue,
				"error", recErr,
			)
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

	ctx, span := tracer.Start(ctx, "publish.message", trace.WithSpanKind(trace.SpanKindProducer))
	defer span.End()

	if err := c.ensureQueueLocked(queue); err != nil {
		return fmt.Errorf("declare queue %q: %w", queue, err)
	}

	headers := amqp.Table{}
	prop.Inject(ctx, amqpHeaderCarrier{headers: headers})

	err := c.channel.PublishWithContext(ctx, "", queue, false, false, amqp.Publishing{
		Headers:     headers,
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
