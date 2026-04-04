package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/CS7580-SEA-SP26/e-team/internal/api"
	"github.com/CS7580-SEA-SP26/e-team/internal/common/parser"
	"github.com/CS7580-SEA-SP26/e-team/internal/common/planner"
	"github.com/CS7580-SEA-SP26/e-team/internal/config"
	"github.com/CS7580-SEA-SP26/e-team/internal/models"
	"github.com/CS7580-SEA-SP26/e-team/internal/observability"
	"github.com/CS7580-SEA-SP26/e-team/internal/store"
)

type Service struct {
	workerURL     string
	validationURL string
	httpClient    *http.Client
	store         *store.Store
}

type jobKey struct {
	stage string
	name  string
}

type jobConfig struct {
	failures bool
	needs    []string
}

func NewService(ctx context.Context) (*Service, error) {
	connURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if connURL == "" {
		connURL = strings.TrimSpace(os.Getenv("REPORT_DB_URL"))
	}
	if connURL == "" {
		return nil, fmt.Errorf("DATABASE_URL or REPORT_DB_URL is required")
	}

	st, err := store.New(ctx, connURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect report store: %w", err)
	}

	traced := observability.NewHTTPClient()
	traced.Timeout = 10 * time.Minute

	return &Service{
		workerURL:     config.GetEnvOrDefaultURL("WORKER_URL", config.DefaultWorkerURL),
		validationURL: config.GetEnvOrDefaultURL("VALIDATION_URL", config.DefaultValidationURL),
		httpClient:    traced,
		store:         st,
	}, nil
}

func (s *Service) Close() {
	if s.store != nil {
		s.store.Close()
	}
}

// Ready reports whether the execution service can serve requests.
// The service depends on the report store and the worker service.
func (s *Service) Ready(ctx context.Context) error {
	if s == nil {
		return fmt.Errorf("execution service is not initialized")
	}
	if s.store == nil {
		return fmt.Errorf("report store is not initialized")
	}
	if err := s.store.Ping(ctx); err != nil {
		return fmt.Errorf("report store is not ready: %w", err)
	}

	resp, err := s.httpClient.Get(s.workerURL + "/ready")
	if err != nil {
		return fmt.Errorf("worker service is not ready: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("worker service readiness returned status %d", resp.StatusCode)
		}
		return fmt.Errorf("worker service readiness returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return nil
}

// Run validates the pipeline before execution.
func (s *Service) Run(ctx context.Context, req api.RunRequest) (*api.RunResponse, error) {
	if strings.TrimSpace(req.YAMLContent) == "" {
		return &api.RunResponse{
			Success: false,
			Errors:  []string{"yaml_content is required"},
		}, nil
	}

	validationResp, err := s.validatePipeline(req.YAMLContent)
	if err != nil {
		return nil, fmt.Errorf("run pipeline: %w", err)
	}

	if !validationResp.Valid {
		return &api.RunResponse{
			Success: false,
			Errors:  validationResp.Errors,
		}, nil
	}

	p := parser.NewParserFromContent(req.YAMLContent)
	pipeline, _, err := p.Parse()
	if err != nil {
		return &api.RunResponse{
			Success: false,
			Errors:  []string{fmt.Sprintf("pipeline parse failed: %v", err)},
		}, nil
	}

	// --- Root span: full pipeline execution ---
	tracer := observability.Tracer("execution")
	ctx, rootSpan := tracer.Start(ctx, "pipeline.run",
		trace.WithAttributes(
			attribute.String("pipeline", pipeline.Name),
		),
	)
	defer rootSpan.End()
	pipelineStart := time.Now()

	runNo, err := s.startRun(ctx, pipeline.Name, req)
	if err != nil {
		rootSpan.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("create run record: %w", err)
	}

	rootSpan.SetAttributes(attribute.Int("run_no", runNo))
	log := observability.WithTraceContext(ctx,
		observability.WithPipelineContext(slog.Default(), pipeline.Name, runNo)).
		With("git_branch", req.Branch, "git_hash", req.Commit)
	log.Info("pipeline run started",
		"event", "pipeline-run",
		"status", store.StatusRunning,
		"source", "service",
	)

	executionPlan, err := planner.GenerateExecutionPlan(pipeline)
	if err != nil {
		s.recordPipelineFailure(ctx, rootSpan, log, pipeline.Name, runNo, pipelineStart)
		return &api.RunResponse{
			Success: false,
			Errors:  []string{fmt.Sprintf("generate execution plan failed: %v", err)},
		}, nil
	}

	jobConfigs := buildJobConfigs(pipeline)
	var logsByJob []string
	for _, stage := range executionPlan.Stages {
		stageResult, stageLogs, resp := s.runStage(ctx, tracer, log, pipeline.Name, runNo, stage, jobConfigs, req)
		logsByJob = append(logsByJob, stageLogs...)
		if resp != nil {
			s.recordPipelineFailure(ctx, rootSpan, log, pipeline.Name, runNo, pipelineStart)
			return resp, nil
		}
		if stageResult == store.StatusFailed {
			s.recordPipelineFailure(ctx, rootSpan, log, pipeline.Name, runNo, pipelineStart)
			return nil, fmt.Errorf("stage %q failed", stage.Name)
		}
	}

	if err := s.finishRun(ctx, pipeline.Name, runNo, store.StatusSuccess); err != nil {
		rootSpan.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("update run record failed: %w", err)
	}

	elapsed := time.Since(pipelineStart).Seconds()
	runNoLabel := strconv.Itoa(runNo)
	observability.PipelineRunsTotal.WithLabelValues(pipeline.Name, runNoLabel, store.StatusSuccess).Inc()
	observability.PipelineDurationSeconds.WithLabelValues(pipeline.Name, runNoLabel).Observe(elapsed)
	rootSpan.SetStatus(codes.Ok, "")
	log.Info("pipeline run completed",
		"event", "pipeline-run",
		"status", store.StatusSuccess,
		"source", "service",
		"duration_s", elapsed,
	)

	return &api.RunResponse{
		Success: true,
		Message: strings.Join(logsByJob, "\n\n"),
	}, nil
}

// runStage executes all jobs in a stage within a child span.
// Returns the stage status, accumulated job logs, and an optional early-return RunResponse (on hard failure).
func (s *Service) runStage(
	ctx context.Context,
	tracer trace.Tracer,
	parentLog *slog.Logger,
	pipelineName string,
	runNo int,
	stage models.StageExecutionPlan,
	jobConfigs map[jobKey]jobConfig,
	req api.RunRequest,
) (string, []string, *api.RunResponse) {
	ctx, stageSpan := tracer.Start(ctx, "stage.run",
		trace.WithAttributes(
			attribute.String("pipeline", pipelineName),
			attribute.String("stage", stage.Name),
			attribute.Int("run_no", runNo),
		),
	)
	defer stageSpan.End()
	stageStart := time.Now()

	log := parentLog.With("stage", stage.Name)

	if err := s.startStage(ctx, pipelineName, runNo, stage.Name); err != nil {
		stageSpan.SetStatus(codes.Error, err.Error())
		if finishErr := s.finishRun(ctx, pipelineName, runNo, store.StatusFailed); finishErr != nil {
			return store.StatusFailed, nil, nil
		}
		return store.StatusFailed, nil, nil
	}

	log.Info("stage started",
		"event", "stage-run",
		"status", store.StatusRunning,
		"source", "service",
	)
	var logsByJob []string

	for _, job := range stage.Jobs {
		key := jobKey{stage: stage.Name, name: job.Name}
		cfg := jobConfigs[key]
		allowFailure := cfg.failures

		if err := s.startJob(ctx, pipelineName, runNo, stage.Name, job.Name, allowFailure); err != nil {
			stageSpan.SetStatus(codes.Error, err.Error())
			_ = s.finishStage(ctx, pipelineName, runNo, stage.Name, store.StatusFailed)
			_ = s.finishRun(ctx, pipelineName, runNo, store.StatusFailed)
			return store.StatusFailed, logsByJob, nil
		}

		logs, jobErr := s.executeJob(ctx, job, req.WorkspacePath, req, pipelineName, runNo, stage.Name)
		if jobErr != nil {
			_ = s.finishJob(ctx, pipelineName, runNo, stage.Name, job.Name, store.StatusFailed)
			if allowFailure {
				log.Warn("job allowed failure",
					"event", "job-run",
					"status", store.StatusFailed,
					"source", "service",
					"job", job.Name,
					"error", jobErr,
				)
				logsByJob = append(logsByJob, fmt.Sprintf("[%s/%s]\nallowed failure: %v", stage.Name, job.Name, jobErr))
				continue
			}

			stageSpan.SetStatus(codes.Error, jobErr.Error())
			log.Error("job failed",
				"event", "job-run",
				"status", store.StatusFailed,
				"source", "service",
				"job", job.Name,
				"error", jobErr,
			)
			_ = s.finishStage(ctx, pipelineName, runNo, stage.Name, store.StatusFailed)
			_ = s.finishRun(ctx, pipelineName, runNo, store.StatusFailed)

			elapsed := time.Since(stageStart).Seconds()
				observability.StageDurationSeconds.WithLabelValues(pipelineName, strconv.Itoa(runNo), stage.Name).Observe(elapsed)

			return store.StatusFailed, logsByJob, &api.RunResponse{
				Success: false,
				Errors:  []string{fmt.Sprintf("job %q in stage %q failed: %v", job.Name, stage.Name, jobErr)},
			}
		}

		_ = s.finishJob(ctx, pipelineName, runNo, stage.Name, job.Name, store.StatusSuccess)
		logsByJob = append(logsByJob, fmt.Sprintf("[%s/%s]\n%s", stage.Name, job.Name, logs))
	}

	if err := s.finishStage(ctx, pipelineName, runNo, stage.Name, store.StatusSuccess); err != nil {
		stageSpan.SetStatus(codes.Error, err.Error())
		_ = s.finishRun(ctx, pipelineName, runNo, store.StatusFailed)
		return store.StatusFailed, logsByJob, nil
	}

	elapsed := time.Since(stageStart).Seconds()
	observability.StageDurationSeconds.WithLabelValues(pipelineName, strconv.Itoa(runNo), stage.Name).Observe(elapsed)
	stageSpan.SetStatus(codes.Ok, "")
	log.Info("stage completed",
		"event", "stage-run",
		"status", store.StatusSuccess,
		"source", "service",
		"duration_s", elapsed,
	)

	return store.StatusSuccess, logsByJob, nil
}

// recordPipelineFailure records failure metrics/span status and persists the failed run.
func (s *Service) recordPipelineFailure(ctx context.Context, span trace.Span, log *slog.Logger, pipeline string, runNo int, start time.Time) {
	elapsed := time.Since(start).Seconds()
	runNoLabel := strconv.Itoa(runNo)
	observability.PipelineRunsTotal.WithLabelValues(pipeline, runNoLabel, store.StatusFailed).Inc()
	observability.PipelineDurationSeconds.WithLabelValues(pipeline, runNoLabel).Observe(elapsed)
	span.SetStatus(codes.Error, "pipeline failed")
	log.Error("pipeline run failed",
		"event", "pipeline-run",
		"status", store.StatusFailed,
		"source", "service",
		"duration_s", elapsed,
	)
	_ = s.finishRun(ctx, pipeline, runNo, store.StatusFailed)
}

// startRun inserts a new pipeline run in running state and returns run_no.
func (s *Service) startRun(ctx context.Context, pipeline string, req api.RunRequest) (int, error) {
	now := time.Now().UTC()

	var traceID string
	if sc := trace.SpanFromContext(ctx).SpanContext(); sc.HasTraceID() {
		traceID = sc.TraceID().String()
	}

	in := store.CreateRunInput{
		Pipeline:  pipeline,
		StartTime: now,
		Status:    store.StatusRunning,
		GitBranch: req.Branch,
		GitHash:   req.Commit,
		GitRepo:   firstNonEmpty(req.RepoURL, req.WorkspacePath),
		TraceID:   traceID,
	}
	return s.store.CreateRun(ctx, in)
}

// finishRun records terminal run status and end_time.
func (s *Service) finishRun(ctx context.Context, pipeline string, runNo int, status string) error {
	now := time.Now().UTC()
	update := store.UpdateRunInput{
		EndTime: &now,
		Status:  status,
	}
	return s.store.UpdateRun(ctx, pipeline, runNo, update)
}

// startStage inserts a stage row in running state for the given run.
func (s *Service) startStage(ctx context.Context, pipeline string, runNo int, stage string) error {
	now := time.Now().UTC()
	in := store.CreateStageInput{
		Pipeline:  pipeline,
		RunNo:     runNo,
		StartTime: now,
		Stage:     stage,
		Status:    store.StatusRunning,
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

// startJob inserts a job row in running state before worker execution.
// failures: when true, job is allowed to fail and does not affect stage status (Track B will set from plan).
func (s *Service) startJob(ctx context.Context, pipeline string, runNo int, stage string, job string, failures bool) error {
	now := time.Now().UTC()
	in := store.CreateJobInput{
		Pipeline:  pipeline,
		RunNo:     runNo,
		Stage:     stage,
		Job:       job,
		StartTime: now,
		Status:    store.StatusRunning,
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

func buildJobConfigs(pipeline *models.Pipeline) map[jobKey]jobConfig {
	if pipeline == nil {
		return map[jobKey]jobConfig{}
	}

	jobConfigs := make(map[jobKey]jobConfig, len(pipeline.Jobs))
	for _, job := range pipeline.Jobs {
		jobConfigs[jobKey{stage: job.Stage, name: job.Name}] = jobConfig{
			failures: job.Failures,
			needs:    append([]string(nil), job.Needs...),
		}
	}

	return jobConfigs
}

// workerExecuteBody is the request body for worker /execute (job + optional workspace + pipeline context).
type workerExecuteBody struct {
	models.JobExecutionPlan
	RepoURL       string `json:"repo_url,omitempty"`
	Commit        string `json:"commit,omitempty"`
	WorkspacePath string `json:"workspace_path,omitempty"`
	Pipeline      string `json:"pipeline,omitempty"`
	RunNo         int    `json:"run_no,omitempty"`
	Stage         string `json:"stage,omitempty"`
}

func (s *Service) executeJob(ctx context.Context, job models.JobExecutionPlan, workspacePath string, req api.RunRequest, pipeline string, runNo int, stage string) (string, error) {
	body, err := json.Marshal(workerExecuteBody{
		JobExecutionPlan: job,
		RepoURL:          req.RepoURL,
		Commit:           req.Commit,
		WorkspacePath:    workspacePath,
		Pipeline:         pipeline,
		RunNo:            runNo,
		Stage:            stage,
	})
	if err != nil {
		return "", fmt.Errorf("marshal worker request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.workerURL+"/execute", bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("create worker request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("call worker service: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)

	if err != nil {
		return "", fmt.Errorf("read worker response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var e workerErrorResponse
		if json.Unmarshal(respBody, &e) == nil && e.Error != "" {
			return "", fmt.Errorf("worker returned %d: %s", resp.StatusCode, e.Error)
		}
		return "", fmt.Errorf("worker returned %d: %s", resp.StatusCode, string(respBody))
	}

	var ok workerExecuteResponse
	if err := json.Unmarshal(respBody, &ok); err != nil {
		return "", fmt.Errorf("unmarshal worker response: %w", err)
	}
	return ok.Logs, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// validatePipeline calls validation service and returns validation result.
func (s *Service) validatePipeline(yamlContent string) (*api.ValidateResponse, error) {
	validateReq := map[string]string{
		"yaml_content": yamlContent,
	}

	jsonBody, err := json.Marshal(validateReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal validation request: %w", err)
	}

	resp, err := s.httpClient.Post(s.validationURL+"/validate", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to call validation service: %w", err)
	}
	defer func() {
		_ = resp.Body.Close() // Ignore close error as we're done with the body
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read validation response: %w", err)
	}

	var validationResp api.ValidateResponse
	if err := json.Unmarshal(body, &validationResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal validation response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Keep server-provided validation details when available.
		if len(validationResp.Errors) == 0 {
			validationResp.Errors = []string{fmt.Sprintf("validation service returned status %d", resp.StatusCode)}
		}
		validationResp.Valid = false
	}

	return &validationResp, nil
}

type workerExecuteResponse struct {
	Logs string `json:"logs"`
}

type workerErrorResponse struct {
	Error string `json:"error"`
}

// callback used to update job status from queued to running
func (s *Service) HandleJobStarted(ctx context.Context, req api.JobStatusCallbackRequest) error {
	if strings.TrimSpace(req.Pipeline) == "" || strings.TrimSpace(req.Stage) == "" || strings.TrimSpace(req.Job) == "" || req.RunNo == 0 {
		return fmt.Errorf("pipeline, run_no, stage, and job are required")
	}
	return s.markJobRunning(ctx, req.Pipeline, req.RunNo, req.Stage, req.Job)
}

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
			if err := s.finishStage(ctx, req.Pipeline, req.RunNo, req.Stage, store.StatusFailed); err != nil {
				return err
			}
			if err := s.finishPipelineRun(ctx, req.Pipeline, req.RunNo, store.StatusFailed); err != nil {
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

	if err := s.finishStage(ctx, req.Pipeline, req.RunNo, req.Stage, store.StatusSuccess); err != nil {
		return err
	}

	nextStageName, ok := rt.nextStageName(req.Stage)

	if !ok {
		if err := s.finishPipelineRun(ctx, req.Pipeline, req.RunNo, store.StatusSuccess); err != nil {
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
		if err := s.finishStage(ctx, req.Pipeline, req.RunNo, nextStageName, store.StatusSuccess); err != nil {
			return err
		}
	}
	return nil
}

