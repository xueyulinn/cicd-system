package cli

import (
	"strings"
	"testing"
)

func TestBuildReportQuery_PipelineRequired(t *testing.T) {
	resetReportFlags()
	reportRun = 1
	_, err := buildReportQuery([]string{""})
	if err == nil || !strings.Contains(err.Error(), "pipeline name is required") {
		t.Fatalf("err=%v", err)
	}
}

func TestBuildReportQuery_NegativeRunRejected(t *testing.T) {
	resetReportFlags()
	reportRun = -1
	_, err := buildReportQuery([]string{"default"})
	if err == nil || !strings.Contains(err.Error(), "run must be a positive integer") {
		t.Fatalf("err=%v", err)
	}
}

func TestBuildReportQuery_RunOmittedProducesNilPointer(t *testing.T) {
	resetReportFlags()
	q, err := buildReportQuery([]string{"default"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q.Run != nil {
		t.Fatalf("expected nil run when --run is omitted, got %+v", *q.Run)
	}
}

func TestBuildReportQuery_JobRequiresStage(t *testing.T) {
	resetReportFlags()
	reportRun = 1
	reportJob = "compile"
	_, err := buildReportQuery([]string{"default"})
	if err == nil || !strings.Contains(err.Error(), "run and stage are required when job is provided") {
		t.Fatalf("err=%v", err)
	}
}

func TestBuildReportQuery_ValidWithJob(t *testing.T) {
	resetReportFlags()
	reportRun = 1
	reportStage = "build"
	reportJob = "compile"

	q, err := buildReportQuery([]string{"default"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q.Pipeline != "default" {
		t.Fatalf("expected pipeline=default, got %q", q.Pipeline)
	}
	if q.Run == nil || *q.Run != 1 {
		t.Fatalf("expected run=1, got %+v", q.Run)
	}
	if q.Stage != "build" || q.Job != "compile" {
		t.Fatalf("unexpected query: %+v", q)
	}
}

func resetReportFlags() {
	reportPipeline = ""
	reportRun = 0
	reportStage = ""
	reportJob = ""
}
