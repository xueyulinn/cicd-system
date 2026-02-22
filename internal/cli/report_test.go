package cli

import "testing"

func TestBuildReportQuery_PipelineRequired(t *testing.T) {
	resetReportFlags()
	_, err := buildReportQuery()
	if err == nil {
		t.Fatal("expected pipeline required error")
	}
}

func TestBuildReportQuery_StageRequiresRun(t *testing.T) {
	resetReportFlags()
	reportPipeline = "default"
	reportStage = "build"
	_, err := buildReportQuery()
	if err == nil {
		t.Fatal("expected stage requires run error")
	}
}

func TestBuildReportQuery_JobRequiresStage(t *testing.T) {
	resetReportFlags()
	reportPipeline = "default"
	reportRun = 1
	reportJob = "compile"
	_, err := buildReportQuery()
	if err == nil {
		t.Fatal("expected job requires stage error")
	}
}

func TestBuildReportQuery_ValidWithJob(t *testing.T) {
	resetReportFlags()
	reportPipeline = "default"
	reportRun = 1
	reportStage = "build"
	reportJob = "compile"

	q, err := buildReportQuery()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
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
