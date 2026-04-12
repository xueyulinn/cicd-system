package worker

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/CS7580-SEA-SP26/e-team/internal/config"
	"github.com/CS7580-SEA-SP26/e-team/internal/messages"
	"github.com/CS7580-SEA-SP26/e-team/internal/mq"
	"github.com/CS7580-SEA-SP26/e-team/internal/observability"
	"github.com/CS7580-SEA-SP26/e-team/internal/store"
	"github.com/moby/moby/client"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	defaultJobTimeout       = 5 * time.Minute
	defaultWorkerConcurrent = 1

	workerTracerName = "worker-service"
)

// Service runs the worker consumers and dependency checks.
type Service struct {
	docker       *client.Client
	jobTimeout   time.Duration
	jobConsumers []mq.Consumer
	executionURL string
	httpClient   *http.Client
	mqConfig     mq.Config
}

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

// NewService creates a worker service backed by Docker and RabbitMQ consumer groups.
func NewService(ctx context.Context, jobTimeout time.Duration) (*Service, error) {
	if jobTimeout == 0 {
		jobTimeout = defaultJobTimeout
	}

	docker, err := NewDockerClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}

	cfg := mq.LoadConfig()
	concurrency := loadWorkerConcurrency()
	jobConsumers, err := createJobConsumers(cfg, concurrency)
	if err != nil {
		_ = docker.Close()
		return nil, err
	}

	return &Service{
		docker:       docker,
		jobTimeout:   jobTimeout,
		jobConsumers: jobConsumers,
		executionURL: config.GetEnvOrDefaultURL("EXECUTION_URL", config.DefaultExecutionURL),
		mqConfig:     cfg,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}, nil
}

// Close releases all underlying consumers and Docker resources held by Service.
func (s *Service) Close() error {
	if s == nil {
		return nil
	}
	for _, consumer := range s.jobConsumers {
		if consumer != nil {
			_ = consumer.Close()
		}
	}
	if s.docker != nil {
		_ = s.docker.Close()
	}
	return nil
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

	// avoids blocking main goroutine
	go func() {
		wg.Wait()
		close(done)
	}()

	// main goroutine listens here
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

// Ready reports whether the worker can reach its required dependencies.
func (s *Service) Ready(ctx context.Context) error {
	if s == nil {
		return fmt.Errorf("worker service is nil")
	}
	if err := PingDocker(ctx, s.docker); err != nil {
		return fmt.Errorf("docker not ready: %w", err)
	}

	rabbitClient, err := mq.NewRabbitClient(s.mqConfig)
	if err != nil {
		return fmt.Errorf("rabbitmq not ready: %w", err)
	}
	defer func() { _ = rabbitClient.Close() }()

	return nil
}

func (s *Service) handleJobMessage(ctx context.Context, msg messages.JobExecutionMessage) (err error) {
	tracer := observability.Tracer(workerTracerName)
	ctx, span := tracer.Start(ctx, "mq.job.consume",
		trace.WithAttributes(
			attribute.String("pipeline", msg.Pipeline),
			attribute.Int("run_no", msg.RunNo),
			attribute.String("stage", msg.Stage),
			attribute.String("job", msg.Job.Name),
		))
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()

	if s.docker == nil {
		return fmt.Errorf("docker client not available")
	}

	timeout := s.jobTimeout
	jobCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	job := msg.Job
	jobName := job.Name
	if jobName == "" {
		jobName = "unnamed"
	}

	if err := s.callbackJobStarted(ctx, msg); err != nil {
		return fmt.Errorf("callback job started: %w", err)
	}

	start := time.Now()
	logs, execErr := ExecuteJob(jobCtx, s.docker, &job, msg.RepoURL, msg.Commit, msg.WorkspacePath)
	duration := time.Since(start)

	if execErr != nil {
		if callbackErr := s.callbackJobFinished(ctx, msg, store.StatusFailed, "", execErr.Error()); callbackErr != nil {
			log.Printf("[worker] callback failed for failed job pipeline=%s run=%d stage=%s job=%s err=%v", msg.Pipeline, msg.RunNo, msg.Stage, jobName, callbackErr)
			return fmt.Errorf("callback job finished (failed): %w", callbackErr)
		}
		log.Printf("[worker] pipeline=%s run=%d stage=%s job=%s duration=%v error=%v", msg.Pipeline, msg.RunNo, msg.Stage, jobName, duration, execErr)
		// Execution-level failures are terminal for this job message once status
		// has been reported back; return nil so MQ ack does not requeue forever.
		return nil
	}

	if err := s.callbackJobFinished(ctx, msg, store.StatusSuccess, logs, ""); err != nil {
		return fmt.Errorf("callback job finished: %w", err)
	}

	log.Printf("[worker] pipeline=%s run=%d stage=%s job=%s duration=%v ok logs=%q", msg.Pipeline, msg.RunNo, msg.Stage, jobName, duration, logs)
	return nil
}

// Run starts the worker consumer until ctx is cancelled.
func Run(ctx context.Context) error {
	srv, err := NewService(ctx, 0)
	if err != nil {
		return fmt.Errorf("create worker service: %w", err)
	}
	defer func() { _ = srv.Close() }()

	if err := srv.Start(ctx); err != nil && ctx.Err() == nil {
		return err
	}
	return nil
}

// returns worker pool
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
