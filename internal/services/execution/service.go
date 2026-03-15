package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/CS7580-SEA-SP26/e-team/internal/api"
	"github.com/CS7580-SEA-SP26/e-team/internal/common/parser"
	"github.com/CS7580-SEA-SP26/e-team/internal/common/planner"
	"github.com/CS7580-SEA-SP26/e-team/internal/config"
	"github.com/CS7580-SEA-SP26/e-team/internal/models"
	"github.com/CS7580-SEA-SP26/e-team/internal/store"
)

type Service struct {
	workerURL     string
	validationURL string
	httpClient    *http.Client
	store         *store.Store
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

	return &Service{
		workerURL:     config.GetEnvOrDefaultURL("WORKER_URL", config.DefaultWorkerURL),
		validationURL: config.GetEnvOrDefaultURL("VALIDATION_URL", config.DefaultValidationURL),
		httpClient: &http.Client{
			// Allow enough time for each job (pull image, build, test); worker uses 5m per job.
			Timeout: 10 * time.Minute,
		},
		store: st}, nil
}

func (s *Service) Close() {
	if s.store != nil {
		s.store.Close()
	}
}

// Run validates the pipeline before execution.
// Actual execution can be added after validation succeeds.
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

	// Parse pipeline from YAML content
	p := parser.NewParserFromContent(req.YAMLContent)
	pipeline, _, err := p.Parse()
	if err != nil {
		return &api.RunResponse{
			Success: false,
			Errors:  []string{fmt.Sprintf("pipeline parse failed: %v", err)},
		}, nil
	}

	// Persist run start metadata before any execution begins.
	runNo, err := s.startRun(ctx, pipeline.Name, req)
	if err != nil {
		return nil, fmt.Errorf("create run record: %w", err)
	}

	// Generate execution plan for the pipeline
	executionPlan, err := planner.GenerateExecutionPlan(pipeline)
	if err != nil {
		if finishErr := s.finishRun(ctx, pipeline.Name, runNo, store.StatusFailed); finishErr != nil {
			return nil, fmt.Errorf("update run record failed: %w", finishErr)
		}
		return &api.RunResponse{
			Success: false,
			Errors:  []string{fmt.Sprintf("generate execution plan failed: %v", err)},
		}, nil
	}

	// Forward jobs in execution order to worker service.
	var logsByJob []string
	for _, stage := range executionPlan.Stages {

		// Persist stage start before dispatching stage jobs.
		if err := s.startStage(ctx, pipeline.Name, runNo, stage.Name); err != nil {
			if finishErr := s.finishRun(ctx, pipeline.Name, runNo, store.StatusFailed); finishErr != nil {
				return nil, fmt.Errorf("update run record failed: %w", finishErr)
			}
			return nil, fmt.Errorf("create stage record failed: %w", err)
		}

		for _, job := range stage.Jobs {
			// Persist job start immediately before worker execution.
			// Failures: default false until Track A adds field to JobExecutionPlan.
			if err := s.startJob(ctx, pipeline.Name, runNo, stage.Name, job.Name, false); err != nil {
				if finishErr := s.finishStage(ctx, pipeline.Name, runNo, stage.Name, store.StatusFailed); finishErr != nil {
					return nil, fmt.Errorf("update stage record failed: %w", finishErr)
				}
				if finishErr := s.finishRun(ctx, pipeline.Name, runNo, store.StatusFailed); finishErr != nil {
					return nil, fmt.Errorf("update run record failed: %w", finishErr)
				}
				return nil, fmt.Errorf("create job record failed: %w", err)
			}

			logs, jobErr := s.executeJob(job, req.WorkspacePath)
			if jobErr != nil {
				// On failure, close job/stage/run as failed and stop processing.
				if err := s.finishJob(ctx, pipeline.Name, runNo, stage.Name, job.Name, store.StatusFailed); err != nil {
					return nil, fmt.Errorf("update job record failed: %w", err)
				}
				if err := s.finishStage(ctx, pipeline.Name, runNo, stage.Name, store.StatusFailed); err != nil {
					return nil, fmt.Errorf("update stage record failed: %w", err)
				}
				if finishErr := s.finishRun(ctx, pipeline.Name, runNo, store.StatusFailed); finishErr != nil {
					return nil, fmt.Errorf("update run record failed: %w", finishErr)
				}

				return &api.RunResponse{
					Success: false,
					Errors:  []string{fmt.Sprintf("job %q in stage %q failed: %v", job.Name, stage.Name, jobErr)},
				}, nil
			}

			if err := s.finishJob(ctx, pipeline.Name, runNo, stage.Name, job.Name, store.StatusSuccess); err != nil {
				return nil, fmt.Errorf("update job record failed: %w", err)
			}
			logsByJob = append(logsByJob, fmt.Sprintf("[%s/%s]\n%s", stage.Name, job.Name, logs))
		}

		// Mark stage success after all jobs in this stage succeed.
		if err := s.finishStage(ctx, pipeline.Name, runNo, stage.Name, store.StatusSuccess); err != nil {
			if finishErr := s.finishRun(ctx, pipeline.Name, runNo, store.StatusFailed); finishErr != nil {
				return nil, fmt.Errorf("update run record failed: %w", finishErr)
			}
			return nil, fmt.Errorf("update stage record failed: %w", err)
		}
	}

	if err := s.finishRun(ctx, pipeline.Name, runNo, store.StatusSuccess); err != nil {
		return nil, fmt.Errorf("update run record failed: %w", err)
	}

	// Execution finished for all jobs.
	return &api.RunResponse{
		Success: true,
		Message: strings.Join(logsByJob, "\n\n"),
	}, nil
}

// startRun inserts a new pipeline run in running state and returns run_no.
func (s *Service) startRun(ctx context.Context, pipeline string, req api.RunRequest) (int, error) {
	now := time.Now().UTC()
	in := store.CreateRunInput{
		Pipeline:  pipeline,
		StartTime: now,
		Status:    store.StatusRunning,
		GitBranch: req.Branch,
		GitHash:   req.Commit,
		GitRepo:   req.WorkspacePath,
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

// workerExecuteBody is the request body for worker /execute (job + optional workspace).
type workerExecuteBody struct {
	models.JobExecutionPlan
	WorkspacePath string `json:"workspace_path,omitempty"`
}

func (s *Service) executeJob(job models.JobExecutionPlan, workspacePath string) (string, error) {
	body, err := json.Marshal(workerExecuteBody{JobExecutionPlan: job, WorkspacePath: workspacePath})
	if err != nil {
		return "", fmt.Errorf("marshal worker request: %w", err)
	}

	resp, err := s.httpClient.Post(s.workerURL+"/execute", "application/json", bytes.NewBuffer(body))

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
