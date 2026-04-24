package worker

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/moby/moby/client"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/xueyulinn/cicd-system/internal/config"
	"github.com/xueyulinn/cicd-system/internal/messages"
	"github.com/xueyulinn/cicd-system/internal/mq"
	"github.com/xueyulinn/cicd-system/internal/store"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	defaultJobTimeout = 10 * time.Minute

	workerTracerName = "worker-service"
)

// Service runs the worker consumers and dependency checks.
type Service struct {
	docker          *client.Client
	jobTimeout      time.Duration
	jobConsumers    []mq.Consumer
	mqConn          *amqp.Connection
	orchestratorURL string
	httpClient      *http.Client
	mqConfig        mq.Config
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
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
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
	tracer := otel.Tracer(workerTracerName)
	ctx, span := tracer.Start(ctx, "mq.job.consume",
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

	logs, execErr := ExecuteJob(jobCtx, s.docker, &job, msg.RepoURL, msg.Commit, msg.WorkspacePath)

	if execErr != nil {
		if callbackErr := s.callbackJobFinished(ctx, msg, store.StatusFailed, "", execErr.Error()); callbackErr != nil {
			log.Printf("[worker] callback failed for failed job pipeline=%s run=%d stage=%s job=%s err=%v", msg.PipelineName, msg.RunNo, msg.Stage, jobName, callbackErr)
			return fmt.Errorf("callback job finished (failed): %w", callbackErr)
		}
		// Execution-level failures are terminal for this job message once status
		// has been reported back; return nil so MQ ack does not requeue forever.
		return nil
	}

	if err := s.callbackJobFinished(ctx, msg, store.StatusSuccess, logs, ""); err != nil {
		return fmt.Errorf("callback job finished: %w", err)
	}

	return nil
}
