package execution

import (
	"context"
	"fmt"
	"strings"

	"github.com/xueyulinn/cicd-system/internal/api"
	"github.com/xueyulinn/cicd-system/internal/store"
)

// HandleJobStarted updates job status from queued to running.
func (s *Service) HandleJobStarted(ctx context.Context, req api.JobStatusCallbackRequest) error {
	if strings.TrimSpace(req.Pipeline) == "" || strings.TrimSpace(req.Stage) == "" || strings.TrimSpace(req.Job) == "" || req.RunNo == 0 {
		return fmt.Errorf("pipeline, run_no, stage, and job are required")
	}
	if err := s.markJobRunning(ctx, req.Pipeline, req.RunNo, req.Stage, req.Job); err != nil {
		return err
	}
	s.noteJobStarted(req.Pipeline, req.RunNo, req.Stage, req.Job)
	return nil
}

// HandleJobFinished updates job/stage/pipeline status and dispatches newly-ready jobs.
func (s *Service) HandleJobFinished(ctx context.Context, req api.JobStatusCallbackRequest) error {
	if strings.TrimSpace(req.Pipeline) == "" || strings.TrimSpace(req.Stage) == "" || strings.TrimSpace(req.Job) == "" || req.RunNo == 0 {
		return fmt.Errorf("pipeline, run_no, stage, and job are required")
	}

	status := strings.TrimSpace(req.Status)
	switch status {
	case store.StatusSuccess, store.StatusFailed:
	default:
		return fmt.Errorf("invalid finished status %q", req.Status)
	}

	// update DB job status
	if err := s.finishJob(ctx, req.Pipeline, req.RunNo, req.Stage, req.Job, status); err != nil {
		return err
	}
	jobStart := s.popJobStartTime(req.Pipeline, req.RunNo, req.Stage, req.Job)
	recordJobOutcome(req.Pipeline, req.RunNo, req.Stage, req.Job, status, jobStart)

	rt := s.getPipelineRuntime(req.Pipeline, req.RunNo)
	if rt == nil {
		return nil
	}

	stage := rt.stageStates[req.Stage]
	if stage == nil {
		return nil
	}

	jobCfg := stage.jobConfigs[req.Job]
	allowFailure := jobCfg.allowFailures

	if status == store.StatusFailed {
		if allowFailure {
			newlyReady := stage.markJobSucceeded(req.Job)
			if err := s.enqueueReadyJobs(ctx, req.Pipeline, req.Stage, req.RunNo, newlyReady, rt.runInfo); err != nil {
				return err
			}
		} else {
			stage.markJobTerminal(req.Job)
			if err := s.finishStageWithMetrics(ctx, req.Pipeline, req.RunNo, req.Stage, store.StatusFailed, stage.startedAt); err != nil {
				return err
			}
			if err := s.finishPipelineRunWithMetrics(ctx, req.Pipeline, req.RunNo, store.StatusFailed, rt.pipelineStart); err != nil {
				return err
			}
			s.deleteRuntime(req.Pipeline, req.RunNo)
			return nil
		}
	} else {
		newlyReady := stage.markJobSucceeded(req.Job)
		if err := s.enqueueReadyJobs(ctx, req.Pipeline, req.Stage, req.RunNo, newlyReady, rt.runInfo); err != nil {
			return err
		}
	}

	// adjust the current stage
	if !stage.isStageComplete() {
		return nil
	}

	if err := s.finishStageWithMetrics(ctx, req.Pipeline, req.RunNo, req.Stage, store.StatusSuccess, stage.startedAt); err != nil {
		return err
	}

	nextStageName, ok := rt.nextStageName(req.Stage)

	if !ok {
		if err := s.finishPipelineRunWithMetrics(ctx, req.Pipeline, req.RunNo, store.StatusSuccess, rt.pipelineStart); err != nil {
			return err
		}
		s.deleteRuntime(req.Pipeline, req.RunNo)
		return nil
	}

	nextStage := rt.stageStates[nextStageName]
	if nextStage == nil {
		return nil
	}

	readyJobs := nextStage.getReadyJobs()
	if err := s.enqueueReadyJobs(ctx, req.Pipeline, nextStageName, req.RunNo, readyJobs, rt.runInfo); err != nil {
		return err
	}

	if nextStage.isStageComplete() {
		if err := s.finishStageWithMetrics(ctx, req.Pipeline, req.RunNo, nextStageName, store.StatusSuccess, nextStage.startedAt); err != nil {
			return err
		}
	}
	return nil
}
