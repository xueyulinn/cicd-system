package worker

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/moby/moby/client"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/xueyulinn/cicd-system/internal/config"
	"github.com/xueyulinn/cicd-system/internal/messages"
	"github.com/xueyulinn/cicd-system/internal/mq"
	"github.com/xueyulinn/cicd-system/internal/observability"
	"github.com/xueyulinn/cicd-system/internal/store"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	defaultJobTimeout = 10 * time.Minute

	workerTracerScope                 = "internal/services/worker"
	orchestratorHTTPDownstreamTag     = "orchestrator"
	defaultFinishedCallbackRetryDelay = time.Second
	maxFinishedCallbackRetryDelay     = 30 * time.Second
)

var finishedCallbackRetryDelay = defaultFinishedCallbackRetryDelay

// Service runs the worker consumers and dependency checks.
type Service struct {
	docker          *client.Client
	jobTimeout      time.Duration
	jobConsumers    []mq.Consumer
	mqConn          *amqp.Connection
	orchestratorURL string
	httpClient      *http.Client
	mqConfig        mq.Config
	tracer          trace.Tracer
}

// NewService constructs a worker service with lazy dependency initialization.
func NewService(jobTimeout time.Duration) *Service {
	if jobTimeout == 0 {
		jobTimeout = defaultJobTimeout
	}

	cfg := mq.LoadConfig()

	return &Service{
		docker:          nil,
		jobTimeout:      jobTimeout,
		jobConsumers:    make([]mq.Consumer, 0),
		mqConn:          nil,
		orchestratorURL: config.GetEnvOrDefaultURL("ORCHESTRATOR_URL", config.DefaultOrchestratorURL),
		mqConfig:        cfg,
		httpClient:      observability.NewInstrumentedHTTPClient(orchestratorHTTPDownstreamTag, 15*time.Second),
		tracer:          observability.Tracer(workerTracerScope),
	}
}

func (s *Service) serviceTracer() trace.Tracer {
	if s != nil && s.tracer != nil {
		return s.tracer
	}
	return observability.Tracer(workerTracerScope)
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
	s.jobConsumers = nil
	if s.mqConn != nil {
		_ = s.mqConn.Close()
		s.mqConn = nil
	}
	if s.docker != nil {
		_ = s.docker.Close()
		s.docker = nil
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
	ctx, span := s.serviceTracer().Start(ctx, "consume.message",
		trace.WithAttributes(
			attribute.String("pipeline", msg.PipelineName),
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

	job := msg.Job
	jobName := job.Name
	if jobName == "" {
		jobName = "unnamed"
	}

	if callbackErr := s.callbackJobStarted(ctx, msg); callbackErr != nil {
		return fmt.Errorf("callback job started: %w", callbackErr)
	}

	if s.docker == nil {
		return fmt.Errorf("docker client not available")
	}

	timeout := s.jobTimeout
	jobCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	logs, execErr := s.ExecuteJob(jobCtx, s.docker, &job, msg.RepoURL, msg.Commit, msg.WorkspaceObjectName)

	if execErr != nil {
		slog.Error("execute job failed",
			"pipeline", msg.PipelineName,
			"run_no", msg.RunNo,
			"stage", msg.Stage,
			"job", jobName,
			"error", execErr,
			"logs", logs,
		)
		if callbackErr := s.reportJobFinishedUntilSuccess(ctx, msg, store.StatusFailed, "", execErr.Error()); callbackErr != nil {
			return fmt.Errorf("callback job finished (failed): %w", callbackErr)
		}
		// Execution-level failures are terminal for this job message once status
		// has been reported back; return nil so MQ ack does not requeue forever.
		return nil
	}

	if callbackErr := s.reportJobFinishedUntilSuccess(ctx, msg, store.StatusSuccess, logs, ""); callbackErr != nil {
		return fmt.Errorf("callback job finished: %w", callbackErr)
	}

	return nil
}

func (s *Service) reportJobFinishedUntilSuccess(ctx context.Context, msg messages.JobExecutionMessage, status string, logs string, errMsg string) error {
	retryDelay := finishedCallbackRetryDelay
	if retryDelay <= 0 {
		retryDelay = defaultFinishedCallbackRetryDelay
	}

	for {
		err := s.callbackJobFinished(ctx, msg, status, logs, errMsg)
		if err == nil {
			return nil
		}

		slog.Warn("callback job finished failed; retrying",
			"pipeline", msg.PipelineName,
			"run_no", msg.RunNo,
			"stage", msg.Stage,
			"job", msg.Job.Name,
			"status", status,
			"error", err,
		)

		if waitErr := waitForRetry(ctx, retryDelay); waitErr != nil {
			return fmt.Errorf("retry callback job finished: %w", err)
		}

		if retryDelay < maxFinishedCallbackRetryDelay {
			retryDelay *= 2
			if retryDelay > maxFinishedCallbackRetryDelay {
				retryDelay = maxFinishedCallbackRetryDelay
			}
		}
	}
}
