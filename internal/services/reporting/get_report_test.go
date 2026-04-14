package reporting

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CS7580-SEA-SP26/e-team/internal/models"
	"github.com/CS7580-SEA-SP26/e-team/internal/store"
)

type mockReportStore struct {
	pingErr      error
	runs         []store.Run
	runsErr      error
	run          *store.Run
	runErr       error
	stages       []store.Stage
	stagesErr    error
	jobs         []store.Job
	jobsErr      error
	closeInvoked bool
}

func (m *mockReportStore) Close() { m.closeInvoked = true }
func (m *mockReportStore) Ping(ctx context.Context) error { return m.pingErr }
func (m *mockReportStore) GetRunsByPipeline(ctx context.Context, pipeline string) ([]store.Run, error) {
	return m.runs, m.runsErr
}
func (m *mockReportStore) GetRun(ctx context.Context, pipeline string, runNo int) (*store.Run, error) {
	return m.run, m.runErr
}
func (m *mockReportStore) GetStagesForRun(ctx context.Context, pipeline string, runNo int, stageFilter string) ([]store.Stage, error) {
	return m.stages, m.stagesErr
}
func (m *mockReportStore) GetJobsForRun(ctx context.Context, pipeline string, runNo int, stageFilter, jobFilter string) ([]store.Job, error) {
	return m.jobs, m.jobsErr
}

func TestServiceClose_WithStore(t *testing.T) {
	ms := &mockReportStore{}
	svc := &Service{store: ms}
	svc.Close()
	if !ms.closeInvoked {
		t.Fatal("expected Close to call store.Close")
	}
}

func TestServicePing_WithStoreErrorAndSuccess(t *testing.T) {
	ms := &mockReportStore{pingErr: errors.New("db down")}
	svc := &Service{store: ms}
	if err := svc.Ping(context.Background()); err == nil {
		t.Fatal("expected ping error")
	}

	ms.pingErr = nil
	if err := svc.Ping(context.Background()); err != nil {
		t.Fatalf("unexpected ping error: %v", err)
	}
}

func TestGetReport_RunlessBranches(t *testing.T) {
	ctx := context.Background()

	t.Run("runs read error", func(t *testing.T) {
		svc := &Service{store: &mockReportStore{runsErr: errors.New("boom")}}
		report, err := svc.GetReport(ctx, models.ReportQuery{Pipeline: "p"})
		if report != nil || err == nil || err.StatusCode != 500 {
			t.Fatalf("report=%#v err=%v", report, err)
		}
	})

	t.Run("pipeline not found", func(t *testing.T) {
		svc := &Service{store: &mockReportStore{runs: []store.Run{}}}
		report, err := svc.GetReport(ctx, models.ReportQuery{Pipeline: "p"})
		if report != nil || err == nil || err.StatusCode != 404 {
			t.Fatalf("report=%#v err=%v", report, err)
		}
	})

	t.Run("success maps runs", func(t *testing.T) {
		now := time.Now().UTC()
		end := now.Add(time.Minute)
		svc := &Service{store: &mockReportStore{runs: []store.Run{{Pipeline: "p", RunNo: 2, Status: store.StatusSuccess, StartTime: now, EndTime: &end}}}}
		report, err := svc.GetReport(ctx, models.ReportQuery{Pipeline: "p"})
		if err != nil || report == nil {
			t.Fatalf("report=%#v err=%v", report, err)
		}
		if report.Pipeline.Name != "p" || len(report.Pipeline.Runs) != 1 || report.Pipeline.Runs[0].RunNo != 2 {
			t.Fatalf("unexpected report: %#v", report)
		}
	})
}

func TestGetReport_RunScopedBranches(t *testing.T) {
	ctx := context.Background()
	runNo := 1
	now := time.Now().UTC()

	baseRun := &store.Run{Pipeline: "p", RunNo: runNo, Status: store.StatusRunning, StartTime: now}

	t.Run("run not found", func(t *testing.T) {
		svc := &Service{store: &mockReportStore{runErr: store.ErrNotFound}}
		report, err := svc.GetReport(ctx, models.ReportQuery{Pipeline: "p", Run: &runNo})
		if report != nil || err == nil || err.StatusCode != 404 {
			t.Fatalf("report=%#v err=%v", report, err)
		}
	})

	t.Run("run read error", func(t *testing.T) {
		svc := &Service{store: &mockReportStore{runErr: errors.New("db")}}
		report, err := svc.GetReport(ctx, models.ReportQuery{Pipeline: "p", Run: &runNo})
		if report != nil || err == nil || err.StatusCode != 500 {
			t.Fatalf("report=%#v err=%v", report, err)
		}
	})

	t.Run("stages read error", func(t *testing.T) {
		svc := &Service{store: &mockReportStore{run: baseRun, stagesErr: errors.New("db")}}
		report, err := svc.GetReport(ctx, models.ReportQuery{Pipeline: "p", Run: &runNo})
		if report != nil || err == nil || err.StatusCode != 500 {
			t.Fatalf("report=%#v err=%v", report, err)
		}
	})

	t.Run("run success with stages", func(t *testing.T) {
		svc := &Service{store: &mockReportStore{run: baseRun, stages: []store.Stage{{Stage: "build", Status: store.StatusSuccess, StartTime: now}}}}
		report, err := svc.GetReport(ctx, models.ReportQuery{Pipeline: "p", Run: &runNo})
		if err != nil || report == nil {
			t.Fatalf("report=%#v err=%v", report, err)
		}
		if len(report.Pipeline.Stages) != 1 || report.Pipeline.Stages[0].Name != "build" {
			t.Fatalf("unexpected report: %#v", report)
		}
	})
}

func TestGetReport_StageAndJobScopedBranches(t *testing.T) {
	ctx := context.Background()
	runNo := 1
	now := time.Now().UTC()

	baseRun := &store.Run{Pipeline: "p", RunNo: runNo, Status: store.StatusRunning, StartTime: now}
	baseStage := []store.Stage{{Stage: "build", Status: store.StatusRunning, StartTime: now}}

	t.Run("stage read error", func(t *testing.T) {
		svc := &Service{store: &mockReportStore{run: baseRun, stagesErr: errors.New("db")}}
		report, err := svc.GetReport(ctx, models.ReportQuery{Pipeline: "p", Run: &runNo, Stage: "build"})
		if report != nil || err == nil || err.StatusCode != 500 {
			t.Fatalf("report=%#v err=%v", report, err)
		}
	})

	t.Run("stage not found", func(t *testing.T) {
		svc := &Service{store: &mockReportStore{run: baseRun, stages: []store.Stage{}}}
		report, err := svc.GetReport(ctx, models.ReportQuery{Pipeline: "p", Run: &runNo, Stage: "build"})
		if report != nil || err == nil || err.StatusCode != 404 {
			t.Fatalf("report=%#v err=%v", report, err)
		}
	})

	t.Run("jobs read error", func(t *testing.T) {
		svc := &Service{store: &mockReportStore{run: baseRun, stages: baseStage, jobsErr: errors.New("db")}}
		report, err := svc.GetReport(ctx, models.ReportQuery{Pipeline: "p", Run: &runNo, Stage: "build"})
		if report != nil || err == nil || err.StatusCode != 500 {
			t.Fatalf("report=%#v err=%v", report, err)
		}
	})

	t.Run("stage success with jobs", func(t *testing.T) {
		svc := &Service{store: &mockReportStore{run: baseRun, stages: baseStage, jobs: []store.Job{{Job: "compile", Status: store.StatusSuccess, StartTime: now}}}}
		report, err := svc.GetReport(ctx, models.ReportQuery{Pipeline: "p", Run: &runNo, Stage: "build"})
		if err != nil || report == nil {
			t.Fatalf("report=%#v err=%v", report, err)
		}
		if len(report.Pipeline.Stage) != 1 || len(report.Pipeline.Stage[0].Jobs) != 1 {
			t.Fatalf("unexpected report: %#v", report)
		}
	})

	t.Run("job read error", func(t *testing.T) {
		svc := &Service{store: &mockReportStore{run: baseRun, stages: baseStage, jobsErr: errors.New("db")}}
		report, err := svc.GetReport(ctx, models.ReportQuery{Pipeline: "p", Run: &runNo, Stage: "build", Job: "compile"})
		if report != nil || err == nil || err.StatusCode != 500 {
			t.Fatalf("report=%#v err=%v", report, err)
		}
	})

	t.Run("job not found", func(t *testing.T) {
		svc := &Service{store: &mockReportStore{run: baseRun, stages: baseStage, jobs: []store.Job{}}}
		report, err := svc.GetReport(ctx, models.ReportQuery{Pipeline: "p", Run: &runNo, Stage: "build", Job: "compile"})
		if report != nil || err == nil || err.StatusCode != 404 {
			t.Fatalf("report=%#v err=%v", report, err)
		}
	})

	t.Run("job success", func(t *testing.T) {
		svc := &Service{store: &mockReportStore{run: baseRun, stages: baseStage, jobs: []store.Job{{Job: "compile", Status: store.StatusSuccess, StartTime: now, Failures: true}}}}
		report, err := svc.GetReport(ctx, models.ReportQuery{Pipeline: "p", Run: &runNo, Stage: "build", Job: "compile"})
		if err != nil || report == nil {
			t.Fatalf("report=%#v err=%v", report, err)
		}
		if len(report.Pipeline.Stage) != 1 || len(report.Pipeline.Stage[0].Job) != 1 || report.Pipeline.Stage[0].Job[0].Name != "compile" {
			t.Fatalf("unexpected report: %#v", report)
		}
	})
}
