package reporting

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/xueyulinn/cicd-system/internal/models"
	"github.com/xueyulinn/cicd-system/internal/store"
)

func TestNewService_RequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("REPORT_DB_URL", "")
	svc, err := NewService(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if svc != nil {
		t.Fatalf("svc=%#v, want nil", svc)
	}
}

func TestServiceClose_AndPingWhenStoreNil(t *testing.T) {
	svc := &Service{}
	svc.Close()

	if err := svc.Ping(context.Background()); err == nil {
		t.Fatal("expected error when store is nil")
	}
}

func TestReportErrorHelpers(t *testing.T) {
	bad := invalidReportQuery("bad")
	if !errors.Is(bad, errInvalidReportQuery) {
		t.Fatalf("invalidReportQuery err=%v", bad)
	}
	status, _, _ := classifyError(bad)
	if status != 400 {
		t.Fatal("invalidReportQuery status")
	}

	missing := reportNotFound("missing")
	if !errors.Is(missing, errReportNotFound) {
		t.Fatalf("reportNotFound err=%v", missing)
	}
	status, _, _ = classifyError(missing)
	if status != 404 {
		t.Fatal("reportNotFound status")
	}

	status, _, _ = classifyError(errors.New("boom"))
	if status != 500 {
		t.Fatal("unexpected status for internal error")
	}
}

func TestGetReport_ValidationCases(t *testing.T) {
	svc := &Service{}
	cases := []models.ReportQuery{
		{},
		{Pipeline: "p", Run: intPtr(0)},
		{Pipeline: "p", Stage: "build"},
		{Pipeline: "p", Run: intPtr(1), Job: "compile"},
	}
	for _, q := range cases {
		report, err := svc.GetReport(context.Background(), q)
		status, _, _ := classifyError(err)
		if err == nil || status != 400 {
			t.Fatalf("query=%+v report=%#v err=%v", q, report, err)
		}
	}
}

func TestMapRunsStagesJobs(t *testing.T) {
	now := time.Now().UTC()
	end := now.Add(time.Minute)

	runs := mapRuns([]store.Run{{
		Pipeline:  "p",
		RunNo:     2,
		Status:    store.StatusSuccess,
		TraceID:   "trace",
		GitRepo:   "repo",
		GitHash:   "abc",
		StartTime: now,
		EndTime:   &end,
	}})
	if len(runs) != 1 || runs[0].RunNo != 2 || runs[0].TraceID != "trace" {
		t.Fatalf("runs=%+v", runs)
	}

	stages := mapStages([]store.Stage{{
		Stage:     "build",
		Status:    store.StatusRunning,
		StartTime: now,
		EndTime:   &end,
	}})
	if len(stages) != 1 || stages[0].Name != "build" {
		t.Fatalf("stages=%+v", stages)
	}

	jobs := mapJobs([]store.Job{{
		Job:       "compile",
		Status:    store.StatusFailed,
		Failures:  true,
		StartTime: now,
		EndTime:   &end,
	}})
	if len(jobs) != 1 || jobs[0].Name != "compile" || !jobs[0].Failures {
		t.Fatalf("jobs=%+v", jobs)
	}
}

func intPtr(v int) *int { return &v }
