package worker

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/CS7580-SEA-SP26/e-team/internal/mq"
)

const defaultWorkerConcurrent = 1

var newJobConsumer = func(cfg mq.Config) (mq.Consumer, error) {
	mqClient, err := mq.NewRabbitClient(cfg)
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

// Start blocks and consumes jobs from RabbitMQ until ctx is cancelled or consuming fails.
func (s *Service) Start(ctx context.Context) error {
	if s == nil {
		return fmt.Errorf("worker service is nil")
	}
	if s.docker == nil {
		return fmt.Errorf("docker client not available")
	}
	if len(s.jobConsumers) == 0 {
		return fmt.Errorf("job consumer not available")
	}

	consumeCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, len(s.jobConsumers))
	done := make(chan struct{})
	var wg sync.WaitGroup
	for i, consumer := range s.jobConsumers {
		wg.Add(1)
		go func(idx int, c mq.Consumer) {
			defer wg.Done()
			if err := c.ConsumeJob(consumeCtx, s.handleJobMessage); err != nil && consumeCtx.Err() == nil {
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

func createJobConsumers(cfg mq.Config, count int) ([]mq.Consumer, error) {
	if count < 1 {
		return nil, fmt.Errorf("worker concurrency must be >= 1")
	}

	consumers := make([]mq.Consumer, 0, count)
	for i := 0; i < count; i++ {
		consumer, err := newJobConsumer(cfg)
		if err != nil {
			for _, c := range consumers {
				_ = c.Close()
			}
			return nil, fmt.Errorf("initialize job consumer %d/%d: %w", i+1, count, err)
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
