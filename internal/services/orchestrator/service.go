package orchestrator

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/xueyulinn/cicd-system/internal/api"
	"github.com/xueyulinn/cicd-system/internal/common/planner"
	"github.com/xueyulinn/cicd-system/internal/config"
	"github.com/xueyulinn/cicd-system/internal/models"
	"github.com/xueyulinn/cicd-system/internal/mq"
	"github.com/xueyulinn/cicd-system/internal/observability"
	"github.com/xueyulinn/cicd-system/internal/store"
	"go.opentelemetry.io/otel/trace"
)

const orchestratorTracerScope = "internal/services/orchestrator"

// Service coordinates pipeline execution, runtime state, and job dispatch.
type Service struct {
	validationURL    string
	validationClient *http.Client
	store            *store.Store
	mqConn           *amqp.Connection
	jobPublishers    []mq.Publisher
	publishIdx       uint64
	runtimeMu        sync.Mutex
	runtimes         map[string]*pipelineRuntime // isolate pipeline runs on parallel
	tracer           trace.Tracer
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

// PipelinePlan contains the validated pipeline and derived execution plan.
// Callers can persist or dispatch the returned plan without re-parsing YAML.
type PipelinePlan struct {
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
	startedAt      time.Time // wall clock when the stage row was created; used for metrics
}

type pipelineRuntime struct {
	pipeline      string
	runNo         int
	stageOrder    []string
	stageStates   map[string]*stageState
	runInfo       runInfo
	pipelineStart time.Time
	jobStartTimes map[jobKey]time.Time
	executionPlan models.ExecutionPlan
}

// NewService constructs an orchestrator service with DB, MQ, and HTTP dependencies initialized.
func NewService(ctx context.Context) (*Service, error) {
	// create DB client
	connURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if connURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	st, err := store.New(ctx, connURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect report store: %w", err)
	}

	// create MQ publishers
	cfg := mq.LoadConfig()
	mqConn, err := amqp.Dial(cfg.URL)
	if err != nil {
		st.Close()
		return nil, fmt.Errorf("fail to connect RabbitMQ: %w", err)
	}
	publisherConcurrency := loadPublisherConcurrency()
	jobPublishers, err := createJobPublishers(cfg, mqConn, publisherConcurrency)
	if err != nil {
		_ = mqConn.Close()
		st.Close()
		return nil, fmt.Errorf("fail to initialize job publishers: %w", err)
	}

	return &Service{
		validationURL:    config.GetEnvOrDefaultURL("VALIDATION_URL", config.DefaultValidationURL),
		validationClient: observability.NewInstrumentedHTTPClient("validation", 2*time.Minute),
		store:            st,
		mqConn:           mqConn,
		jobPublishers:    jobPublishers,
		runtimes:         make(map[string]*pipelineRuntime),
		tracer:           observability.Tracer(orchestratorTracerScope),
	}, nil
}

func (s *Service) serviceTracer() trace.Tracer {
	if s != nil && s.tracer != nil {
		return s.tracer
	}
	return observability.Tracer(orchestratorTracerScope)
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

// Close releases resources held by the Service, including MQ publisher and DB store.
func (s *Service) Close() {
	for _, publisher := range s.jobPublishers {
		if publisher != nil {
			_ = publisher.Close()
		}
	}
	if s.mqConn != nil {
		_ = s.mqConn.Close()
	}
	if s.store != nil {
		s.store.Close()
	}
}

func (s *Service) nextPublisher() mq.Publisher {
	if s == nil || len(s.jobPublishers) == 0 {
		return nil
	}
	idx := atomic.AddUint64(&s.publishIdx, 1) - 1
	return s.jobPublishers[idx%uint64(len(s.jobPublishers))]
}

// Ready reports whether the orchestrator service can serve requests.
// The service depends on the store client and mq client .
func (s *Service) Ready(ctx context.Context) error {
	if s == nil {
		return fmt.Errorf("orchestrator service is not initialized")
	}
	if s.store == nil {
		return fmt.Errorf("report store is not initialized")
	}
	if err := s.store.Ping(ctx); err != nil {
		return fmt.Errorf("report store is not ready: %w", err)
	}

	timedoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := mq.PingMQ(timedoutCtx, mq.LoadConfig()); err != nil {
		return fmt.Errorf("mq is not ready: %w", err)
	}

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

// noteJobStarted records when a job entered running state (worker callback); used for job duration metrics.
func (s *Service) noteJobStarted(pipeline string, runNo int, stageName, jobName string) {
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	rt := s.runtimes[runtimeKey(pipeline, runNo)]
	if rt == nil || rt.jobStartTimes == nil {
		return
	}
	rt.jobStartTimes[jobKey{stage: stageName, name: jobName}] = time.Now().UTC()
}

// popJobStartTime returns and removes the job running start time for duration metrics.
func (s *Service) popJobStartTime(pipeline string, runNo int, stageName, jobName string) time.Time {
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	rt := s.runtimes[runtimeKey(pipeline, runNo)]
	if rt == nil || rt.jobStartTimes == nil {
		return time.Time{}
	}
	key := jobKey{stage: stageName, name: jobName}
	t := rt.jobStartTimes[key]
	delete(rt.jobStartTimes, key)
	return t
}

func (rt *pipelineRuntime) nextStageName(current string) (string, bool) {
	for idx, name := range rt.stageOrder {
		if name == current && idx+1 < len(rt.stageOrder) {
			return rt.stageOrder[idx+1], true
		}
	}
	return "", false
}

// returns in-flight deduped pipeline run ortherwise initializes pipeline runtime
func (s *Service) initializePipelineRuntime(ctx context.Context, pipelinePlan PipelinePlan, jobConfigs map[jobKey]jobConfig, runInfo runInfo, requestKey string) (runtime *pipelineRuntime, deduped bool, status string, err error) {
	ctx, span := s.serviceTracer().Start(ctx, "initialize.pipeline.runtime")
	defer span.End()

	pipeline := pipelinePlan.Pipeline
	executionPlan := pipelinePlan.ExecutionPlan
	runResult, runStart, err := s.startPipelineRun(ctx, pipeline.Name, runInfo, requestKey)
	if err != nil {
		return nil, false, "", fmt.Errorf("create pipeline run record failed: %w", err)
	}

	if runResult.Deduped {
		// For deduped requests, return the existing run number so callers can respond deterministically.
		return &pipelineRuntime{runNo: runResult.RunNo}, true, runResult.ExistingStatus, nil
	}
	runNo := runResult.RunNo

	stageStates := make(map[string]*stageState, len(executionPlan.Stages))
	stageOrder := make([]string, 0, len(executionPlan.Stages))

	// Persist stage/job rows up front in queued state. Only the first stage's
	// initial ready jobs are dispatched here; later releases happen on job-success events.
	for _, stage := range executionPlan.Stages {
		if err := s.startStage(ctx, pipeline.Name, runNo, stage.Name); err != nil {
			_ = s.finishPipelineRunWithMetrics(ctx, pipeline.Name, runNo, store.StatusFailed, runStart)
			return nil, false, "", fmt.Errorf("create stage record failed: %w", err)
		}

		// generates static execution plan for current stage
		stagePlan := planner.BuildStagePlan(stage.Name, pipeline)

		// maintain current running stage state
		st := newStageState(stagePlan)
		st.startedAt = time.Now().UTC()
		stageStates[stage.Name] = st
		stageOrder = append(stageOrder, stage.Name)

		for _, job := range stage.Jobs {
			key := jobKey{stage: stage.Name, name: job.Name}
			cfg := jobConfigs[key]
			stageStates[stage.Name].jobConfigs[job.Name] = cfg
			allowFailure := cfg.allowFailures
			if err := s.startJob(ctx, pipeline.Name, runNo, stage.Name, job.Name, allowFailure); err != nil {
				_ = s.finishStageWithMetrics(ctx, pipeline.Name, runNo, stage.Name, store.StatusFailed, stageStates[stage.Name].startedAt)
				_ = s.finishPipelineRunWithMetrics(ctx, pipeline.Name, runNo, store.StatusFailed, runStart)
				return nil, false, "", fmt.Errorf("create job record failed: %w", err)
			}
		}
	}

	runtime = &pipelineRuntime{
		pipeline:      pipeline.Name,
		runNo:         runNo,
		stageOrder:    stageOrder,
		stageStates:   stageStates,
		runInfo:       runInfo,
		pipelineStart: runStart,
		jobStartTimes: make(map[jobKey]time.Time),
		executionPlan: *pipelinePlan.ExecutionPlan,
	}

	// no stages found
	if len(executionPlan.Stages) == 0 {
		if err := s.finishPipelineRunWithMetrics(ctx, pipeline.Name, runNo, store.StatusSuccess, runStart); err != nil {
			return nil, false, "", fmt.Errorf("finish empty pipeline run: %w", err)
		}
		return runtime, false, "", nil
	}

	return runtime, false, "", nil
}

// Run validates the pipeline, initializes run records/state, dispatches initial jobs, and returns immediately.
func (s *Service) Run(ctx context.Context, req api.RunRequest) (*api.RunResponse, error) {
	ctx, span := s.serviceTracer().Start(ctx, "process.pipeline")
	defer span.End()

	pipelinePlan, err := s.prepareRun(ctx, req)
	if err != nil {
		return nil, err
	}

	// {branch, commit, repo}
	info := newRunInfo(req)
	jobConfigs := buildJobConfigs(pipelinePlan.Pipeline)
	requestKey := buildRunRequestKey(req, pipelinePlan.Pipeline.Name)

	runtime, deduped, status, err := s.initializePipelineRuntime(ctx, *pipelinePlan, jobConfigs, info, requestKey)
	if err != nil {
		return nil, err
	}

	if deduped {
		slog.Info("pipeline deduped", "pipiline", pipelinePlan.Pipeline.Name, "runNo", runtime.runNo)
		return &api.RunResponse{
			Pipeline: pipelinePlan.Pipeline.Name,
			RunNo:    runtime.runNo,
			Status:   status,
			Message:  fmt.Sprintf("duplicate run request dropped; using in-flight run %d.", runtime.runNo),
		}, nil
	}

	s.putRuntime(runtime)

	go func(ctx context.Context, runtime *pipelineRuntime) {
		dispatchCtx := context.WithoutCancel(ctx)
		dispatchCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		if err := s.dispatchPipelineStartJobs(dispatchCtx, runtime); err != nil {
			slog.Error("dispatch initial ready jobs failed", "error", err)
			return
		}
	}(ctx, runtime)

	return &api.RunResponse{
		Pipeline: pipelinePlan.Pipeline.Name,
		RunNo:    runtime.runNo,
		Status:   store.StatusQueued,
	}, nil
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
