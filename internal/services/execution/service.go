package execution

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/CS7580-SEA-SP26/e-team/internal/api"
	"github.com/CS7580-SEA-SP26/e-team/internal/common/planner"
	"github.com/CS7580-SEA-SP26/e-team/internal/config"
	"github.com/CS7580-SEA-SP26/e-team/internal/models"
	"github.com/CS7580-SEA-SP26/e-team/internal/mq"
	"github.com/CS7580-SEA-SP26/e-team/internal/observability"
	"github.com/CS7580-SEA-SP26/e-team/internal/store"
	amqp "github.com/rabbitmq/amqp091-go"
)

const executionClientName = "execution-service"

// Service coordinates pipeline execution, runtime state, and job dispatch.
type Service struct {
	workerURL      string
	validationURL  string
	httpValidation *http.Client
	httpWorker     *http.Client
	store          *store.Store
	mqConn         *amqp.Connection
	jobPublishers  []mq.Publisher
	publishIdx     uint64
	runtimeMu      sync.Mutex
	runtimes       map[string]*pipelineRuntime // isolate pipeline runs on parallel
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
	startedAt      time.Time // wall clock when the stage row was created; used for metrics
}

type initializedRun struct {
	runNo   int
	runtime *pipelineRuntime
	deduped bool
	status  string
}

type pipelineRuntime struct {
	pipeline        string
	runNo           int
	stageOrder      []string
	stageStates     map[string]*stageState
	runInfo         runInfo
	pipelineStart   time.Time
	jobStartTimes   map[jobKey]time.Time
}

// NewService constructs an execution Service with DB, MQ, and HTTP dependencies initialized.
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
		workerURL:     config.GetEnvOrDefaultURL("WORKER_URL", config.DefaultWorkerURL),
		validationURL: config.GetEnvOrDefaultURL("VALIDATION_URL", config.DefaultValidationURL),
		// Validation calls are typically short; worker readiness and future HTTP paths may be long.
		httpValidation: observability.NewInstrumentedHTTPClient(executionClientName, "validation", 2*time.Minute),
		httpWorker:     observability.NewInstrumentedHTTPClient(executionClientName, "worker", 10*time.Minute),
		store:          st,
		mqConn:         mqConn,
		jobPublishers:  jobPublishers,
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

// initialize pipeline runtime
func (s *Service) initializePipelineRun(ctx context.Context, prepared PreparedRun, jobConfigs map[jobKey]jobConfig, runInfo runInfo, requestKey string) (*initializedRun, error) {
	pipeline := prepared.Pipeline
	executionPlan := prepared.ExecutionPlan
	runResult, runStart, err := s.startPipelineRun(ctx, pipeline.Name, runInfo, requestKey)

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
			_ = s.finishPipelineRunWithMetrics(ctx, pipeline.Name, runNo, store.StatusFailed, runStart)
			return nil, fmt.Errorf("create stage record failed: %w", err)
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
				return nil, fmt.Errorf("create job record failed: %w", err)
			}
		}
	}

	runtime := &pipelineRuntime{
		pipeline:       pipeline.Name,
		runNo:          runNo,
		stageOrder:     stageOrder,
		stageStates:    stageStates,
		runInfo:        runInfo,
		pipelineStart:  runStart,
		jobStartTimes:  make(map[jobKey]time.Time),
	}

	// no stages found
	if len(executionPlan.Stages) == 0 {
		if err := s.finishPipelineRunWithMetrics(ctx, pipeline.Name, runNo, store.StatusSuccess, runStart); err != nil {
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

// Run validates the pipeline, initializes run records/state, dispatches initial jobs, and returns immediately.
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
