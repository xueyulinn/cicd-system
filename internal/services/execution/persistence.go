package execution

import (
	"context"
	"strings"
	"time"

	"github.com/xueyulinn/cicd-system/internal/store"
)

// startPipelineRun inserts a new pipeline run in queued state and returns run_no.
// The returned time matches the run start_time written to the store for new runs.
func (s *Service) startPipelineRun(ctx context.Context, pipeline string, runInfo runInfo, requestKey string) (store.CreateRunResult, time.Time, error) {
	now := time.Now().UTC()
	in := store.CreateRunInput{
		Pipeline:   pipeline,
		StartTime:  now,
		Status:     store.StatusQueued,
		GitBranch:  runInfo.Branch,
		GitHash:    runInfo.Commit,
		GitRepo:    runInfo.RepoURL,
		RequestKey: requestKey,
	}
	if strings.TrimSpace(in.GitRepo) == "" {
		in.GitRepo = runInfo.WorkspacePath
	}
	result, err := s.store.CreateRunOrGetActive(ctx, in)
	if err != nil {
		return store.CreateRunResult{}, time.Time{}, err
	}
	return result, now, nil
}

// finishPipelineRunWithMetrics updates the run in the store then records Prometheus pipeline metrics.
func (s *Service) finishPipelineRunWithMetrics(ctx context.Context, pipeline string, runNo int, status string, pipelineStart time.Time) error {
	if err := s.finishPipelineRun(ctx, pipeline, runNo, status); err != nil {
		return err
	}
	recordPipelineOutcome(pipeline, runNo, status, pipelineStart)
	return nil
}

// finishStageWithMetrics updates the stage in the store then records stage duration histograms.
func (s *Service) finishStageWithMetrics(ctx context.Context, pipeline string, runNo int, stage string, status string, stageStart time.Time) error {
	if err := s.finishStage(ctx, pipeline, runNo, stage, status); err != nil {
		return err
	}
	recordStageDuration(pipeline, runNo, stage, stageStart)
	return nil
}

// finishRun records terminal run status and end_time.
func (s *Service) finishPipelineRun(ctx context.Context, pipeline string, runNo int, status string) error {
	now := time.Now().UTC()
	update := store.UpdateRunInput{
		EndTime: &now,
		Status:  status,
	}
	return s.store.UpdateRun(ctx, pipeline, runNo, update)
}

// startStage inserts a stage row in queued state for the given run.
func (s *Service) startStage(ctx context.Context, pipeline string, runNo int, stage string) error {
	now := time.Now().UTC()
	in := store.CreateStageInput{
		Pipeline:  pipeline,
		RunNo:     runNo,
		StartTime: now,
		Stage:     stage,
		Status:    store.StatusQueued,
	}
	return s.store.CreateStage(ctx, in)
}

// finishStage records terminal stage status and end_time.
func (s *Service) finishStage(ctx context.Context, pipeline string, runNo int, stage string, status string) error {
	now := time.Now().UTC()
	update := store.UpdateStageInput{
		EndTime: &now,
		Status:  status,
	}
	return s.store.UpdateStage(ctx, pipeline, runNo, stage, update)
}

// startJob inserts a job row in queued state before worker execution.
// failures: when true, job is allowed to fail and does not affect stage status (Track B will set from plan).
func (s *Service) startJob(ctx context.Context, pipeline string, runNo int, stage string, job string, failures bool) error {
	now := time.Now().UTC()
	in := store.CreateJobInput{
		Pipeline:  pipeline,
		RunNo:     runNo,
		Stage:     stage,
		Job:       job,
		StartTime: now,
		Status:    store.StatusQueued,
		Failures:  failures,
	}
	return s.store.CreateJob(ctx, in)
}

// finishJob records terminal job status and end_time.
func (s *Service) finishJob(ctx context.Context, pipeline string, runNo int, stage string, job string, status string) error {
	now := time.Now().UTC()
	update := store.UpdateJobInput{
		EndTime: &now,
		Status:  status,
	}
	return s.store.UpdateJob(ctx, pipeline, runNo, stage, job, update)
}

// markJobRunning updates a queued job to running status.
func (s *Service) markJobRunning(ctx context.Context, pipeline string, runNo int, stage string, job string) error {
	update := store.UpdateJobInput{
		EndTime: nil,
		Status:  store.StatusRunning,
	}
	return s.store.UpdateJob(ctx, pipeline, runNo, stage, job, update)
}
