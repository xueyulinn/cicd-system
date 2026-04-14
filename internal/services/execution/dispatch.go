package execution

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/CS7580-SEA-SP26/e-team/internal/messages"
	"github.com/CS7580-SEA-SP26/e-team/internal/models"
	"github.com/CS7580-SEA-SP26/e-team/internal/observability"
	"github.com/CS7580-SEA-SP26/e-team/internal/store"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// buildJobExecutionMessage constructs the MQ payload for a single job in a pipeline run.
func (s *Service) buildJobExecutionMessage(runNo int, pipeline, stage string, job models.JobExecutionPlan, runInfo runInfo) messages.JobExecutionMessage {
	return messages.JobExecutionMessage{
		RunNo:         runNo,
		Pipeline:      pipeline,
		Stage:         stage,
		RepoURL:       runInfo.RepoURL,
		Branch:        runInfo.Branch,
		Commit:        runInfo.Commit,
		WorkspacePath: runInfo.WorkspacePath,
		Job:           job,
	}
}

// enqueueJob publishes a single job execution message to MQ.
func (s *Service) enqueueJob(ctx context.Context, msg messages.JobExecutionMessage) error {
	publisher := s.nextPublisher()
	if publisher == nil {
		return fmt.Errorf("job publisher is not initialized")
	}

	tracer := observability.Tracer(executionClientName)
	ctx, span := tracer.Start(ctx, "mq.job.publish",
		trace.WithAttributes(
			attribute.String("pipeline", msg.Pipeline),
			attribute.Int("run_no", msg.RunNo),
			attribute.String("stage", msg.Stage),
			attribute.String("job", msg.Job.Name),
		))
	defer span.End()

	publishCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	if err := publisher.PublishJob(publishCtx, msg); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	observability.RecordExecutionJobEnqueued(msg.Pipeline, msg.Stage)
	return nil
}

// enqueueReadyJobs publishes all ready jobs for the current stage.
func (s *Service) enqueueReadyJobs(ctx context.Context, pipelineName, stageName string, runNo int, jobs []models.JobExecutionPlan, runInfo runInfo) error {
	observability.RecordExecutionReadyBatchSize(len(jobs))
	if len(jobs) > 1 {
		slog.Default().Info("mq dispatch batch",
			"event", "mq-dispatch-batch",
			"pipeline", pipelineName,
			"stage", stageName,
			"run_no", runNo,
			"batch_size", len(jobs),
		)
	}
	for _, job := range jobs {
		msg := s.buildJobExecutionMessage(runNo, pipelineName, stageName, job, runInfo)
		if err := s.enqueueJob(ctx, msg); err != nil {
			return err
		}
	}
	return nil
}

// enqueueInitialReadyJobs dispatches ready jobs in the first runnable stage of a run.
func (s *Service) enqueueInitialReadyJobs(ctx context.Context, pipelineName string, runNo int, stages []models.StageExecutionPlan, stageStates map[string]*stageState, runInfo runInfo) error {
	for _, stage := range stages {
		state := stageStates[stage.Name]
		if state == nil {
			continue
		}

		readyJobs := state.getReadyJobs()
		if len(readyJobs) == 0 {
			// Empty stages can be completed immediately so dispatch can advance.
			if len(stage.Jobs) == 0 {
				if err := s.finishStageWithMetrics(ctx, pipelineName, runNo, stage.Name, store.StatusSuccess, state.startedAt); err != nil {
					return fmt.Errorf("finish empty stage %q: %w", stage.Name, err)
				}
				continue
			}

			// Non-empty stages with no ready jobs should not happen for a valid first runnable stage,
			// but we continue scanning to avoid hard-coding a single stage name.
			continue
		}

		if err := s.enqueueReadyJobs(ctx, pipelineName, stage.Name, runNo, readyJobs, runInfo); err != nil {
			return fmt.Errorf("enqueue ready jobs for stage %q: %w", stage.Name, err)
		}

		// stops when ready-jobs are found for a stage
		return nil
	}

	return nil
}

// dispatchInitialReadyJobs dispatches initial jobs and handles early dispatch failures.
func (s *Service) dispatchInitialReadyJobs(ctx context.Context, prepared PreparedRun, initialized *initializedRun) error {
	if initialized == nil || initialized.runtime == nil {
		return fmt.Errorf("initialized run is required")
	}

	pipeline := prepared.Pipeline
	executionPlan := prepared.ExecutionPlan
	if err := s.enqueueInitialReadyJobs(ctx, pipeline.Name, initialized.runNo, executionPlan.Stages, initialized.runtime.stageStates, initialized.runtime.runInfo); err != nil {
		_ = s.finishPipelineRunWithMetrics(ctx, pipeline.Name, initialized.runNo, store.StatusFailed, initialized.runtime.pipelineStart)
		s.deleteRuntime(pipeline.Name, initialized.runNo)
		return err
	}

	return nil
}
