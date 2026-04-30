package reporting

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/xueyulinn/cicd-system/internal/models"
	"github.com/xueyulinn/cicd-system/internal/store"
)

type reportStore interface {
	Close()
	Ping(ctx context.Context) error
	GetRunsByPipeline(ctx context.Context, pipeline string) ([]store.Run, error)
	GetRun(ctx context.Context, pipeline string, runNo int) (*store.Run, error)
	GetStagesForRun(ctx context.Context, pipeline string, runNo int, stageFilter string) ([]store.Stage, error)
	GetJobsForRun(ctx context.Context, pipeline string, runNo int, stageFilter, jobFilter string) ([]store.Job, error)
}

// Service provides report queries backed by the report store.
type Service struct {
	store reportStore
}

// NewService creates a reporting service using DATABASE_URL or REPORT_DB_URL.
func NewService(ctx context.Context) (*Service, error) {
	connURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if connURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	st, err := store.New(ctx, connURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect report store: %w", err)
	}

	return &Service{store: st}, nil
}

// Close releases the underlying store connection.
func (s *Service) Close() {
	if s.store != nil {
		s.store.Close()
	}
}

// Ping reports whether the report store is ready to serve requests.
func (s *Service) Ping(ctx context.Context) error {
	if s.store == nil {
		return errors.New("store is not initialized")
	}
	return s.store.Ping(ctx)
}

// GetReport returns a pipeline-, run-, stage-, or job-scoped report view.
func (s *Service) GetReport(ctx context.Context, query models.ReportQuery) (*models.ReportResponse, error) {
	if strings.TrimSpace(query.Pipeline) == "" {
		return nil, invalidReportQuery("pipeline is required")
	}
	if query.Run != nil && *query.Run <= 0 {
		return nil, invalidReportQuery("run must be a positive integer")
	}
	if query.Stage != "" && query.Run == nil {
		return nil, invalidReportQuery("run is required when stage is provided")
	}
	if query.Job != "" && (query.Run == nil || query.Stage == "") {
		return nil, invalidReportQuery("run and stage are required when job is provided")
	}

	if query.Run == nil {
		runs, err := s.store.GetRunsByPipeline(ctx, query.Pipeline)
		if err != nil {
			return nil, fmt.Errorf("failed to read runs: %w", err)
		}
		if len(runs) == 0 {
			return nil, reportNotFound(fmt.Sprintf("pipeline %q not found", query.Pipeline))
		}

		resp := &models.ReportResponse{
			Pipeline: models.ReportPipeline{
				Name: query.Pipeline,
				Runs: mapRuns(runs),
			},
		}
		return resp, nil
	}

	runNo := *query.Run
	run, err := s.store.GetRun(ctx, query.Pipeline, runNo)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, reportNotFound(fmt.Sprintf("run %d for pipeline %q not found", runNo, query.Pipeline))
		}
		return nil, fmt.Errorf("failed to read run: %w", err)
	}

	resp := &models.ReportResponse{
		Pipeline: models.ReportPipeline{
			Name:    run.Pipeline,
			RunNo:   run.RunNo,
			Status:  run.Status,
			TraceID: run.TraceID,
			Start:   run.StartTime,
			End:     run.EndTime,
		},
	}

	if query.Stage == "" {
		stages, err := s.store.GetStagesForRun(ctx, query.Pipeline, runNo, "")
		if err != nil {
			return nil, fmt.Errorf("failed to read stages: %w", err)
		}
		resp.Pipeline.Stages = mapStages(stages)
		return resp, nil
	}

	stages, err := s.store.GetStagesForRun(ctx, query.Pipeline, runNo, query.Stage)
	if err != nil {
		return nil, fmt.Errorf("failed to read stage: %w", err)
	}
	if len(stages) == 0 {
		return nil, reportNotFound(fmt.Sprintf("stage %q not found for pipeline %q run %d", query.Stage, query.Pipeline, runNo))
	}

	stageReport := models.ReportStage{
		Name:   stages[0].Stage,
		Status: stages[0].Status,
		Start:  stages[0].StartTime,
		End:    stages[0].EndTime,
	}

	if query.Job == "" {
		jobs, err := s.store.GetJobsForRun(ctx, query.Pipeline, runNo, query.Stage, "")
		if err != nil {
			return nil, fmt.Errorf("failed to read jobs: %w", err)
		}
		stageReport.Jobs = mapJobs(jobs)
		resp.Pipeline.Stage = []models.ReportStage{stageReport}
		return resp, nil
	}

	jobs, err := s.store.GetJobsForRun(ctx, query.Pipeline, runNo, query.Stage, query.Job)
	if err != nil {
		return nil, fmt.Errorf("failed to read job: %w", err)
	}
	if len(jobs) == 0 {
		return nil, reportNotFound(fmt.Sprintf("job %q not found in stage %q for pipeline %q run %d", query.Job, query.Stage, query.Pipeline, runNo))
	}

	stageReport.Job = mapJobs(jobs)
	resp.Pipeline.Stage = []models.ReportStage{stageReport}
	return resp, nil
}

func mapRuns(runs []store.Run) []models.ReportRun {
	out := make([]models.ReportRun, 0, len(runs))
	for _, r := range runs {
		out = append(out, models.ReportRun{
			RunNo:     r.RunNo,
			Status:    r.Status,
			TraceID:   r.TraceID,
			GitRepo:   r.GitRepo,
			GitBranch: r.GitBranch,
			GitHash:   r.GitHash,
			Start:     r.StartTime,
			End:       r.EndTime,
		})
	}
	return out
}

func mapStages(stages []store.Stage) []models.ReportStage {
	out := make([]models.ReportStage, 0, len(stages))
	for _, st := range stages {
		out = append(out, models.ReportStage{
			Name:   st.Stage,
			Status: st.Status,
			Start:  st.StartTime,
			End:    st.EndTime,
		})
	}
	return out
}

func mapJobs(jobs []store.Job) []models.ReportJob {
	out := make([]models.ReportJob, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, models.ReportJob{
			Name:     j.Job,
			Status:   j.Status,
			Start:    j.StartTime,
			End:      j.EndTime,
			Failures: j.Failures,
		})
	}
	return out
}
