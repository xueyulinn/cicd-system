package orchestrator

import (
	"context"
	"fmt"

	"github.com/xueyulinn/cicd-system/internal/messages"
	"github.com/xueyulinn/cicd-system/internal/models"
	"github.com/xueyulinn/cicd-system/internal/store"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// buildJobExecutionMessage constructs the MQ payload for a single job in a pipeline run.
func (s *Service) buildJobExecutionMessage(runNo int, pipeline, stage string, job models.JobExecutionPlan, runInfo runInfo) messages.JobExecutionMessage {
	return messages.JobExecutionMessage{
		RunNo:         runNo,
		PipelineName:  pipeline,
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

	ctx, span := s.serviceTracer().Start(ctx, "publish.job.message",
		trace.WithAttributes(
			attribute.String("pipeline_name", msg.PipelineName),
			attribute.Int("run_no", msg.RunNo),
			attribute.String("stage", msg.Stage),
			attribute.String("job", msg.Job.Name),
		))
	defer span.End()

	if err := publisher.PublishJob(ctx, msg); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

// enqueueReadyJobs publishes all ready jobs for the current stage.
func (s *Service) enqueueReadyJobs(ctx context.Context, pipelineName, stageName string, runNo int, jobs []models.JobExecutionPlan, runInfo runInfo) error {
	for _, job := range jobs {
		msg := s.buildJobExecutionMessage(runNo, pipelineName, stageName, job, runInfo)
		if err := s.enqueueJob(ctx, msg); err != nil {
			return err
		}
	}
	return nil
}

// enqueueFirstReadyStageJobs scans stages in order and enqueues ready jobs from
// the first stage that has ready work.
func (s *Service) enqueueFirstReadyStageJobs(ctx context.Context, pipelineName string, runNo int, stages []models.StageExecutionPlan, stageStates map[string]*stageState, runInfo runInfo) error {
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

		// serial-run stage: stops when ready-jobs are found for a stage
		return nil
	}

	return nil
}

// dispatchPipelineStartJobs dispatches the initial runnable jobs for a pipeline run.
func (s *Service) dispatchPipelineStartJobs(ctx context.Context, runtime *pipelineRuntime) error {
	ctx, span := s.serviceTracer().Start(ctx, "start.dispatch.jobs")
	defer span.End()

	if runtime == nil {
		return fmt.Errorf("pipeline runtime is required")
	}

	if err := s.enqueueFirstReadyStageJobs(ctx, runtime.pipeline, runtime.runNo, runtime.executionPlan.Stages, runtime.stageStates, runtime.runInfo); err != nil {
		_ = s.finishPipelineRunWithMetrics(ctx, runtime.pipeline, runtime.runNo, store.StatusFailed, runtime.pipelineStart)
		s.deleteRuntime(runtime.pipeline, runtime.runNo)
		return err
	}

	return nil
}
