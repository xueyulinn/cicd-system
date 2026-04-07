package execution

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/CS7580-SEA-SP26/e-team/internal/api"
	"github.com/CS7580-SEA-SP26/e-team/internal/common/parser"
	"github.com/CS7580-SEA-SP26/e-team/internal/common/planner"
	"github.com/CS7580-SEA-SP26/e-team/internal/config"
	"github.com/CS7580-SEA-SP26/e-team/internal/messages"
	"github.com/CS7580-SEA-SP26/e-team/internal/models"
	"github.com/CS7580-SEA-SP26/e-team/internal/mq"
	"github.com/CS7580-SEA-SP26/e-team/internal/observability"
	"github.com/CS7580-SEA-SP26/e-team/internal/store"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// struct called by execution handler
const executionClientName = "execution-service"

type Service struct {
	workerURL       string
	validationURL   string
	httpValidation  *http.Client
	httpWorker      *http.Client
	store           *store.Store
	jobPublisher    mq.Publisher
	runtimeMu       sync.Mutex
	runtimes        map[string]*pipelineRuntime // isolate pipeline runs on parallel
}

// job identifier
type jobKey struct {
	stage string
	name  string
}

// metadata for each job
type jobConfig struct {
	allowFailures bool
	needs         []string
}

// PreparedRun contains the validated pipeline and derived execution plan.
// Callers can persist or dispatch the returned plan without re-parsing YAML.
type PreparedRun struct {
	Pipeline      *models.Pipeline
	ExecutionPlan *models.ExecutionPlan
}

// context for this pipeline run
type runInfo struct {
	RepoURL       string
	Branch        string
	Commit        string
	WorkspacePath string
}

type stageState struct {
	plan           models.StagePlan
	remainingNeeds map[string]int
	queued         map[string]bool
	completed      map[string]bool
	jobConfigs     map[string]jobConfig
}

type initializedRun struct {
	runNo   int
	runtime *pipelineRuntime
	deduped bool
	status  string
}

type pipelineRuntime struct {
	pipeline    string
	runNo       int
	stageOrder  []string
	stageStates map[string]*stageState
	runInfo     runInfo
}

// service constructor
func NewService(ctx context.Context) (*Service, error) {
	// create DB client
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

	// create MQ client
	cfg := mq.LoadConfig()
	mqClient, err := mq.NewRabbitClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("fail to create RabbitMQ client: %w", err)
	}

	jobPublisher, err := mq.NewJobPublisher(mqClient, cfg)
	if err != nil {
		_ = mqClient.Close()
		st.Close()
		return nil, fmt.Errorf("fail to initialize job publisher: %w", err)
	}

	return &Service{
		workerURL:       config.GetEnvOrDefaultURL("WORKER_URL", config.DefaultWorkerURL),
		validationURL: config.GetEnvOrDefaultURL("VALIDATION_URL", config.DefaultValidationURL),
		// Validation calls are typically short; worker readiness and future HTTP paths may be long.
		httpValidation: observability.NewInstrumentedHTTPClient(executionClientName, "validation", 2*time.Minute),
		httpWorker:     observability.NewInstrumentedHTTPClient(executionClientName, "worker", 10*time.Minute),
		store:          st,
		jobPublisher:   jobPublisher,
		runtimes:       make(map[string]*pipelineRuntime),
	}, nil
}

func newRunInfo(req api.RunRequest) runInfo {
	return runInfo{
		RepoURL:       req.RepoURL,
		Branch:        req.Branch,
		Commit:        req.Commit,
		WorkspacePath: req.WorkspacePath,
	}
}

func buildRunRequestKey(req api.RunRequest, pipelineName string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		pipelineName,
		req.YAMLContent,
		strings.TrimSpace(req.Branch),
		strings.TrimSpace(req.Commit),
		strings.TrimSpace(req.RepoURL),
	}, "\n")))
	return fmt.Sprintf("%x", sum[:])
}

// close dependent resources: DB client and MQ client
func (s *Service) Close() {
	if s.jobPublisher != nil {
		_ = s.jobPublisher.Close()
	}
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

	resp, err := s.httpWorker.Get(s.workerURL + "/ready")
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

	timedoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := mq.PingMQ(timedoutCtx, mq.LoadConfig()); err != nil {
		return fmt.Errorf("mq is not ready: %w", err)
	}

	return nil
}

// PrepareRun validates the incoming YAML and returns static execution plan and pipeline dto.
func (s *Service) prepareRun(req api.RunRequest) (*PreparedRun, *api.RunResponse, error) {
	if strings.TrimSpace(req.YAMLContent) == "" {
		return nil, &api.RunResponse{
			Pipeline: "",
			Status:   "failed",
			Errors:   []string{"yaml_content is required"},
		}, nil
	}

	validationResp, err := s.validatePipeline(req.YAMLContent)
	if err != nil {
		return nil, nil, fmt.Errorf("run pipeline: %w", err)
	}

	if !validationResp.Valid {
		return nil, &api.RunResponse{
			Pipeline: "",
			Status:   "failed",
			Errors:   validationResp.Errors,
		}, nil
	}

	p := parser.NewParserFromContent(req.YAMLContent)
	pipeline, _, err := p.Parse()
	if err != nil {
		return nil, &api.RunResponse{
			Pipeline: "",
			Status:   "failed",
			Errors:   []string{fmt.Sprintf("pipeline parse failed: %v", err)},
		}, nil
	}

	// generate static executionPlan for current pipeline run
	executionPlan, err := planner.GenerateExecutionPlan(pipeline)
	if err != nil {
		return nil, &api.RunResponse{
			Pipeline: pipeline.Name,
			Status:   "failed",
			Errors:   []string{fmt.Sprintf("generate execution plan failed: %v", err)},
		}, nil
	}

	return &PreparedRun{
		Pipeline:      pipeline,
		ExecutionPlan: executionPlan,
	}, nil, nil
}

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

// publish job into mq using jobPublisher
func (s *Service) enqueueJob(ctx context.Context, msg messages.JobExecutionMessage) error {
	if s.jobPublisher == nil {
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

	if err := s.jobPublisher.PublishJob(publishCtx, msg); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	observability.RecordExecutionJobEnqueued(msg.Pipeline, msg.Stage)
	return nil
}

// constructor for stageState
func newStageState(stagePlan models.StagePlan) *stageState {
	remaining := make(map[string]int, len(stagePlan.InDegree))
	for name, degree := range stagePlan.InDegree {
		remaining[name] = degree
	}

	return &stageState{
		plan:           stagePlan,
		remainingNeeds: remaining,
		queued:         make(map[string]bool, len(stagePlan.JobByName)),
		completed:      make(map[string]bool, len(stagePlan.JobByName)),
		jobConfigs:     make(map[string]jobConfig, len(stagePlan.JobByName)),
	}
}

func (s *stageState) markQueued(jobName string) {
	s.queued[jobName] = true
}

func (s *stageState) markJobTerminal(jobName string) {
	s.completed[jobName] = true
}

// get ready jobs for a stage
func (s *stageState) getReadyJobs() []models.JobExecutionPlan {
	ready := make([]models.JobExecutionPlan, 0)
	for _, job := range s.plan.Jobs {
		if s.remainingNeeds[job.Name] == 0 && !s.queued[job.Name] && !s.completed[job.Name] {
			s.markQueued(job.Name)
			ready = append(ready, job)
		}
	}
	return ready
}

// finish job and return newly ready jobs
func (s *stageState) markJobSucceeded(jobName string) []models.JobExecutionPlan {
	if s.completed[jobName] {
		return nil
	}
	s.markJobTerminal(jobName)

	newlyReady := make([]models.JobExecutionPlan, 0)
	// decrement the indegrees for job's dependent
	for _, dependent := range s.plan.Dependents[jobName] {
		if s.remainingNeeds[dependent] > 0 {
			s.remainingNeeds[dependent]--
		}
		if s.remainingNeeds[dependent] == 0 && !s.queued[dependent] && !s.completed[dependent] {
			s.markQueued(dependent)
			newlyReady = append(newlyReady, s.plan.JobByName[dependent])
		}
	}
	return newlyReady
}

func (s *stageState) isStageComplete() bool {
	return len(s.completed) == len(s.plan.JobByName)
}

// publish msg to the queue
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

// serial execute stages
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
				if err := s.finishStage(ctx, pipelineName, runNo, stage.Name, store.StatusSuccess); err != nil {
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

// build pipeline identifier
func runtimeKey(pipeline string, runNo int) string {
	return fmt.Sprintf("%s:%d", pipeline, runNo)
}

func (s *Service) putRuntime(rt *pipelineRuntime) {
	if rt == nil {
		return
	}
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	s.runtimes[runtimeKey(rt.pipeline, rt.runNo)] = rt
}

func (s *Service) getPipelineRuntime(pipeline string, runNo int) *pipelineRuntime {
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	return s.runtimes[runtimeKey(pipeline, runNo)]
}

func (s *Service) deleteRuntime(pipeline string, runNo int) {
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	delete(s.runtimes, runtimeKey(pipeline, runNo))
}

func (rt *pipelineRuntime) nextStageName(current string) (string, bool) {
	for idx, name := range rt.stageOrder {
		if name == current && idx+1 < len(rt.stageOrder) {
			return rt.stageOrder[idx+1], true
		}
	}
	return "", false
}

// initialize pipeline runtime
func (s *Service) initializePipelineRun(ctx context.Context, prepared PreparedRun, jobConfigs map[jobKey]jobConfig, runInfo runInfo, requestKey string) (*initializedRun, error) {
	pipeline := prepared.Pipeline
	executionPlan := prepared.ExecutionPlan
	runResult, err := s.startPipelineRun(ctx, pipeline.Name, runInfo, requestKey)

	if err != nil {
		return nil, fmt.Errorf("create pipeline run record: %w", err)
	}
	if runResult.Deduped {
		return &initializedRun{
			runNo:   runResult.RunNo,
			deduped: true,
			status:  runResult.ExistingStatus,
		}, nil
	}
	runNo := runResult.RunNo

	stageStates := make(map[string]*stageState, len(executionPlan.Stages))
	stageOrder := make([]string, 0, len(executionPlan.Stages))

	// Persist stage/job rows up front in queued state. Only the first stage's
	// initial ready jobs are dispatched here; later releases happen on job-success events.
	for _, stage := range executionPlan.Stages {
		if err := s.startStage(ctx, pipeline.Name, runNo, stage.Name); err != nil {
			_ = s.finishPipelineRun(ctx, pipeline.Name, runNo, store.StatusFailed)
			return nil, fmt.Errorf("create stage record failed: %w", err)
		}

		// generates static execution plan for current stage
		stagePlan := planner.BuildStagePlan(stage.Name, pipeline)

		// maintain current running stage state
		stageStates[stage.Name] = newStageState(stagePlan)
		stageOrder = append(stageOrder, stage.Name)

		for _, job := range stage.Jobs {
			key := jobKey{stage: stage.Name, name: job.Name}
			cfg := jobConfigs[key]
			stageStates[stage.Name].jobConfigs[job.Name] = cfg
			allowFailure := cfg.allowFailures
			if err := s.startJob(ctx, pipeline.Name, runNo, stage.Name, job.Name, allowFailure); err != nil {
				_ = s.finishStage(ctx, pipeline.Name, runNo, stage.Name, store.StatusFailed)
				_ = s.finishPipelineRun(ctx, pipeline.Name, runNo, store.StatusFailed)
				return nil, fmt.Errorf("create job record failed: %w", err)
			}
		}
	}

	runtime := &pipelineRuntime{
		pipeline:    pipeline.Name,
		runNo:       runNo,
		stageOrder:  stageOrder,
		stageStates: stageStates,
		runInfo:     runInfo,
	}

	// no stages found
	if len(executionPlan.Stages) == 0 {
		if err := s.finishPipelineRun(ctx, pipeline.Name, runNo, store.StatusSuccess); err != nil {
			return nil, fmt.Errorf("finish empty pipeline run: %w", err)
		}
		return &initializedRun{
			runNo:   runNo,
			runtime: runtime,
		}, nil
	}

	return &initializedRun{
		runNo:   runNo,
		runtime: runtime,
	}, nil
}

func (s *Service) dispatchInitialReadyJobs(ctx context.Context, prepared PreparedRun, initialized *initializedRun) error {
	if initialized == nil || initialized.runtime == nil {
		return fmt.Errorf("initialized run is required")
	}

	pipeline := prepared.Pipeline
	executionPlan := prepared.ExecutionPlan
	if err := s.enqueueInitialReadyJobs(ctx, pipeline.Name, initialized.runNo, executionPlan.Stages, initialized.runtime.stageStates, initialized.runtime.runInfo); err != nil {
		_ = s.finishPipelineRun(ctx, pipeline.Name, initialized.runNo, store.StatusFailed)
		s.deleteRuntime(pipeline.Name, initialized.runNo)
		return err
	}

	return nil
}

// validate pipeline, dispatch jobs concurrently and retutrn right away
func (s *Service) Run(ctx context.Context, req api.RunRequest) (*api.RunResponse, error) {
	// validate pipeline file and generate execution plan
	prepared, runResp, err := s.prepareRun(req)
	if runResp != nil {
		return runResp, nil
	}
	if err != nil {
		return nil, err
	}

	// {branch, commit, repo}
	info := newRunInfo(req)
	jobConfigs := buildJobConfigs(prepared.Pipeline)
	requestKey := buildRunRequestKey(req, prepared.Pipeline.Name)

	initialized, err := s.initializePipelineRun(ctx, *prepared, jobConfigs, info, requestKey)
	if err != nil {
		return nil, err
	}
	if initialized.deduped {
		return &api.RunResponse{
			Pipeline: prepared.Pipeline.Name,
			RunNo:    initialized.runNo,
			Status:   initialized.status,
			Message:  fmt.Sprintf("Duplicate run request dropped; using in-flight run %d.", initialized.runNo),
		}, nil
	}
	s.putRuntime(initialized.runtime)

	// multi-thread run
	go func(prepared PreparedRun, initialized *initializedRun) {
		dispatchCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.dispatchInitialReadyJobs(dispatchCtx, prepared, initialized); err != nil {
			fmt.Fprintf(os.Stderr, "dispatch initial ready jobs failed for run %d: %v\n", initialized.runNo, err)
		}
	}(*prepared, initialized)

	return &api.RunResponse{
		Pipeline: prepared.Pipeline.Name,
		RunNo:    initialized.runNo,
		Status:   store.StatusQueued,
	}, nil
}

// startPipelineRun inserts a new pipeline run in queued state and returns run_no.
func (s *Service) startPipelineRun(ctx context.Context, pipeline string, runInfo runInfo, requestKey string) (store.CreateRunResult, error) {
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
	return s.store.CreateRunOrGetActive(ctx, in)
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

// buildJobConfigs returns a lookup table keyed by {stage, job name}.
// Each entry stores execution-related job metadata such as allow-failures and needs dependencies.
func buildJobConfigs(pipeline *models.Pipeline) map[jobKey]jobConfig {
	if pipeline == nil {
		return map[jobKey]jobConfig{}
	}

	jobConfigs := make(map[jobKey]jobConfig, len(pipeline.Jobs))
	for _, job := range pipeline.Jobs {
		jobConfigs[jobKey{stage: job.Stage, name: job.Name}] = jobConfig{
			allowFailures: job.Failures,
			needs:         append([]string(nil), job.Needs...),
		}
	}

	return jobConfigs
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

	resp, err := s.httpValidation.Post(s.validationURL+"/validate", "application/json", bytes.NewBuffer(jsonBody))
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

// update job status as running
func (s *Service) markJobRunning(ctx context.Context, pipeline string, runNo int, stage string, job string) error {
	update := store.UpdateJobInput{
		EndTime: nil,
		Status:  store.StatusRunning,
	}
	return s.store.UpdateJob(ctx, pipeline, runNo, stage, job, update)
}
