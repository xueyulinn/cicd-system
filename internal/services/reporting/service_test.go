package reporting

import (
	"context"
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

func TestServiceErrorHelpers(t *testing.T) {
	e := &serviceError{StatusCode: 418, Message: "teapot"}
	if e.Error() != "teapot" {
		t.Fatalf("error=%q", e.Error())
	}
	if badRequest("bad").StatusCode != 400 {
		t.Fatal("badRequest status")
	}
	if notFound("missing").StatusCode != 404 {
		t.Fatal("notFound status")
	}
	ie := internalError("x=%d", 1)
	if ie.StatusCode != 500 || ie.Message != "x=1" {
		t.Fatalf("internalError=%#v", ie)
	}
}

func TestGetReport_ValidationBranches(t *testing.T) {
	svc := &Service{}
	cases := []models.ReportQuery{
		{},
		{Pipeline: "p", Run: intPtr(0)},
		{Pipeline: "p", Stage: "build"},
		{Pipeline: "p", Run: intPtr(1), Job: "compile"},
	}
	for _, q := range cases {
		report, err := svc.GetReport(context.Background(), q)
		if err == nil || err.StatusCode != 400 {
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
		GitBranch: "main",
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
