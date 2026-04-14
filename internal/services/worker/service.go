package worker

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/CS7580-SEA-SP26/e-team/internal/config"
	"github.com/CS7580-SEA-SP26/e-team/internal/messages"
	"github.com/CS7580-SEA-SP26/e-team/internal/mq"
	"github.com/CS7580-SEA-SP26/e-team/internal/observability"
	"github.com/CS7580-SEA-SP26/e-team/internal/store"
	"github.com/moby/moby/client"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	defaultJobTimeout = 5 * time.Minute

	workerTracerName = "worker-service"
)

// Service runs the worker consumers and dependency checks.
type Service struct {
	docker       *client.Client
	jobTimeout   time.Duration
	jobConsumers []mq.Consumer
	mqConn       *amqp.Connection
	executionURL string
	httpClient   *http.Client
	mqConfig     mq.Config
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
	mqConn, err := amqp.Dial(cfg.URL)
	if err != nil {
		_ = docker.Close()
		return nil, fmt.Errorf("connect rabbitmq: %w", err)
	}
	concurrency := loadWorkerConcurrency()
	jobConsumers, err := createJobConsumers(cfg, mqConn, concurrency)
	if err != nil {
		_ = mqConn.Close()
		_ = docker.Close()
		return nil, err
	}

	return &Service{
		docker:       docker,
		jobTimeout:   jobTimeout,
		jobConsumers: jobConsumers,
		mqConn:       mqConn,
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
	if s.mqConn != nil {
		_ = s.mqConn.Close()
	}
	if s.docker != nil {
		_ = s.docker.Close()
	}
	return nil
}

// Ready reports whether the worker can reach its required dependencies.
func (s *Service) Ready(ctx context.Context) error {
	if s == nil {
		return fmt.Errorf("worker service is nil")
	}
	if err := PingDocker(ctx, s.docker); err != nil {
		return fmt.Errorf("docker not ready: %w", err)
	}

	if err := mq.PingMQ(ctx, s.mqConfig); err != nil {
		return fmt.Errorf("rabbitmq not ready: %w", err)
	}

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
