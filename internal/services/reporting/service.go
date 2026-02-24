package reporting

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/CS7580-SEA-SP26/e-team/internal/models"
	"github.com/CS7580-SEA-SP26/e-team/internal/store"
)

type Service struct {
	store *store.Store
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

	return &Service{store: st}, nil
}

func (s *Service) Close() {
	if s.store != nil {
		s.store.Close()
	}
}

type ServiceError struct {
	StatusCode int
	Message    string
}

func (e *ServiceError) Error() string {
	return e.Message
}

func (s *Service) GetReport(ctx context.Context, query models.ReportQuery) (*models.ReportResponse, *ServiceError) {
	if strings.TrimSpace(query.Pipeline) == "" {
		return nil, &ServiceError{StatusCode: 400, Message: "pipeline is required"}
	}
	if query.Run != nil && *query.Run <= 0 {
		return nil, &ServiceError{StatusCode: 400, Message: "run must be a positive integer"}
	}
	if query.Stage != "" && query.Run == nil {
		return nil, &ServiceError{StatusCode: 400, Message: "run is required when stage is provided"}
	}
	if query.Job != "" && (query.Run == nil || query.Stage == "") {
		return nil, &ServiceError{StatusCode: 400, Message: "run and stage are required when job is provided"}
	}

	if query.Run == nil {
		runs, err := s.store.GetRunsByPipeline(ctx, query.Pipeline)
		if err != nil {
			return nil, &ServiceError{StatusCode: 500, Message: fmt.Sprintf("failed to read runs: %v", err)}
		}
		if len(runs) == 0 {
			return nil, &ServiceError{StatusCode: 404, Message: fmt.Sprintf("pipeline %q not found", query.Pipeline)}
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
			return nil, &ServiceError{StatusCode: 404, Message: fmt.Sprintf("run %d for pipeline %q not found", runNo, query.Pipeline)}
		}
		return nil, &ServiceError{StatusCode: 500, Message: fmt.Sprintf("failed to read run: %v", err)}
	}

	resp := &models.ReportResponse{
		Pipeline: models.ReportPipeline{
			Name:   run.Pipeline,
			RunNo:  run.RunNo,
			Status: run.Status,
			Start:  run.StartTime,
			End:    run.EndTime,
		},
	}

	if query.Stage == "" {
		stages, err := s.store.GetStagesForRun(ctx, query.Pipeline, runNo, "")
		if err != nil {
			return nil, &ServiceError{StatusCode: 500, Message: fmt.Sprintf("failed to read stages: %v", err)}
		}
		resp.Pipeline.Stages = mapStages(stages)
		return resp, nil
	}

	stages, err := s.store.GetStagesForRun(ctx, query.Pipeline, runNo, query.Stage)
	if err != nil {
		return nil, &ServiceError{StatusCode: 500, Message: fmt.Sprintf("failed to read stage: %v", err)}
	}
	if len(stages) == 0 {
		return nil, &ServiceError{StatusCode: 404, Message: fmt.Sprintf("stage %q not found for pipeline %q run %d", query.Stage, query.Pipeline, runNo)}
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
			return nil, &ServiceError{StatusCode: 500, Message: fmt.Sprintf("failed to read jobs: %v", err)}
		}
		stageReport.Jobs = mapJobs(jobs)
		resp.Pipeline.Stage = []models.ReportStage{stageReport}
		return resp, nil
	}

	jobs, err := s.store.GetJobsForRun(ctx, query.Pipeline, runNo, query.Stage, query.Job)
	if err != nil {
		return nil, &ServiceError{StatusCode: 500, Message: fmt.Sprintf("failed to read job: %v", err)}
	}
	if len(jobs) == 0 {
		return nil, &ServiceError{StatusCode: 404, Message: fmt.Sprintf("job %q not found in stage %q for pipeline %q run %d", query.Job, query.Stage, query.Pipeline, runNo)}
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
			Name:   j.Job,
			Status: j.Status,
			Start:  j.StartTime,
			End:    j.EndTime,
		})
	}
	return out
}
