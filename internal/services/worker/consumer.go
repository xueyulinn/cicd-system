package worker

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/xueyulinn/cicd-system/internal/messages"
	"github.com/xueyulinn/cicd-system/internal/mq"
)

const defaultWorkerConcurrent = 1

const dependencyRetryDelay = 2 * time.Second

var newJobConsumer = func(cfg mq.Config, conn *amqp.Connection) (mq.Consumer, error) {
	mqClient, err := mq.NewRabbitClientWithConn(cfg, conn)
	if err != nil {
		return nil, err
	}

	jobConsumer, err := mq.NewJobConsumer(mqClient, cfg)
	if err != nil {
		_ = mqClient.Close()
		return nil, err
	}
	return jobConsumer, nil
}

var newDockerClient = NewDockerClient
var dialRabbitMQ = amqp.Dial

// Start blocks and consumes jobs from RabbitMQ until ctx is cancelled or consuming fails.
func (s *Service) Start(ctx context.Context) error {
	if s == nil {
		return fmt.Errorf("worker service is nil")
	}
	if err := s.ensureDependencies(ctx); err != nil {
		return err
	}
	return s.consumeJobs(ctx)
}

func (s *Service) ensureDependencies(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		if s.docker == nil {
			dockerClient, err := newDockerClient(ctx)
			if err != nil {
				log.Printf("[worker] docker not ready: %v", err)
				if err := waitForRetry(ctx, dependencyRetryDelay); err != nil {
					return err
				}
				continue
			}
			s.docker = dockerClient
		}

		if len(s.jobConsumers) == 0 {
			if s.mqConn != nil {
				_ = s.mqConn.Close()
				s.mqConn = nil
			}

			conn, err := dialRabbitMQ(s.mqConfig.URL)
			if err != nil {
				log.Printf("[worker] rabbitmq not ready: %v", err)
				if err := waitForRetry(ctx, dependencyRetryDelay); err != nil {
					return err
				}
				continue
			}

			jobConsumers, err := createJobConsumers(s.mqConfig, conn)
			if err != nil {
				_ = conn.Close()
				log.Printf("[worker] initialize job consumers failed: %v", err)
				if err := waitForRetry(ctx, dependencyRetryDelay); err != nil {
					return err
				}
				continue
			}

			s.mqConn = conn
			s.jobConsumers = jobConsumers
		}

		if s.docker != nil && len(s.jobConsumers) > 0 {
			return nil
		}
	}
}

func consumeWorkers(ctx context.Context, consumers []mq.Consumer, handler func(context.Context, messages.JobExecutionMessage) error) error {
	consumeCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, len(consumers))
	done := make(chan struct{})
	var wg sync.WaitGroup
	for i, consumer := range consumers {
		wg.Add(1)
		go func(idx int, c mq.Consumer) {
			defer wg.Done()
			if err := c.ConsumeJob(consumeCtx, handler); err != nil && consumeCtx.Err() == nil {
				errCh <- fmt.Errorf("job consumer %d failed: %w", idx+1, err)
			}
		}(i, consumer)
	}

	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case err := <-errCh:
		cancel()
		<-done
		return err
	case <-ctx.Done():
		cancel()
		<-done
		return ctx.Err()
	case <-done:
		return nil
	}
}

func (s *Service) consumeJobs(ctx context.Context) error {
	if s.docker == nil {
		return fmt.Errorf("docker client not available")
	}
	if len(s.jobConsumers) == 0 {
		return fmt.Errorf("job consumer not available")
	}

	return consumeWorkers(ctx, s.jobConsumers, s.handleJobMessage)
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func createJobConsumers(cfg mq.Config, conn *amqp.Connection) ([]mq.Consumer, error) {
	consumerNumber := loadWorkerConcurrency()
	if consumerNumber < 1 {
		return nil, fmt.Errorf("worker concurrency must be >= 1")
	}

	consumers := make([]mq.Consumer, 0, consumerNumber)
	for i := 0; i < consumerNumber; i++ {
		consumer, err := newJobConsumer(cfg, conn)
		if err != nil {
			for _, c := range consumers {
				_ = c.Close()
			}
			return nil, fmt.Errorf("initialize worker %d/%d: %w", i+1, consumerNumber, err)
		}
		consumers = append(consumers, consumer)
	}
	return consumers, nil
}

func loadWorkerConcurrency() int {
	raw := strings.TrimSpace(os.Getenv("WORKER_CONCURRENCY"))
	if raw == "" {
		return defaultWorkerConcurrent
	}

	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 {
		log.Printf("[worker] invalid WORKER_CONCURRENCY=%q, fallback=%d", raw, defaultWorkerConcurrent)
		return defaultWorkerConcurrent
	}
	return v
}
