package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/CS7580-SEA-SP26/e-team/internal/models"
)

func TestFormatExecutionPlanYAML_StageOrder(t *testing.T) {
	plan := &models.ExecutionPlan{
		Stages: []models.StageExecutionPlan{
			{Name: "build", Jobs: []models.JobExecutionPlan{{Name: "compile", Image: "golang:1.21", Script: []string{"go build"}}}},
			{Name: "test", Jobs: []models.JobExecutionPlan{{Name: "unit-tests", Image: "golang:1.21", Script: []string{"go test"}}}},
			{Name: "doc", Jobs: []models.JobExecutionPlan{{Name: "javadoc", Image: "gradle:jdk21", Script: []string{"gradle javadoc"}}}},
			{Name: "deploy", Jobs: []models.JobExecutionPlan{{Name: "package", Image: "gradle:jdk21", Script: []string{"gradle assembleDist"}}}},
		},
	}

	out, err := FormatExecutionPlanYAML(plan)
	if err != nil {
		t.Fatalf("FormatExecutionPlanYAML: %v", err)
	}
	yamlStr := string(out)
	buildIdx := strings.Index(yamlStr, "build:")
	testIdx := strings.Index(yamlStr, "test:")
	docIdx := strings.Index(yamlStr, "doc:")
	deployIdx := strings.Index(yamlStr, "deploy:")
	if buildIdx == -1 || testIdx == -1 || docIdx == -1 || deployIdx == -1 {
		t.Fatalf("Expected all stages in output, got: %s", yamlStr)
	}
	if buildIdx >= testIdx || testIdx >= docIdx || docIdx >= deployIdx {
		t.Errorf("Stages should be in order build, test, doc, deploy. YAML:\n%s", yamlStr)
	}
}

func TestFormatExecutionPlanJSON_Valid(t *testing.T) {
	plan := &models.ExecutionPlan{
		Stages: []models.StageExecutionPlan{
			{Name: "build", Jobs: []models.JobExecutionPlan{
				{Name: "compile", Image: "golang:1.21", Script: []string{"go build"}},
			}},
		},
	}

	out, err := FormatExecutionPlanJSON(plan)
	if err != nil {
		t.Fatalf("FormatExecutionPlanJSON: %v", err)
	}
	if !strings.Contains(string(out), `"stages"`) {
		t.Errorf("Expected JSON to contain stages key, got: %s", out)
	}
	if !strings.Contains(string(out), `"build"`) {
		t.Errorf("Expected JSON to contain build stage, got: %s", out)
	}
}

func TestFormatExecutionPlanYAML_NilPlan(t *testing.T) {
	_, err := FormatExecutionPlanYAML(nil)
	if err == nil {
		t.Error("Expected error for nil plan")
	}
}

func TestFormatExecutionPlanJSON_NilPlan(t *testing.T) {
	_, err := FormatExecutionPlanJSON(nil)
	if err == nil {
		t.Error("Expected error for nil plan")
	}
}

func TestFormatReportYAML_Valid(t *testing.T) {
	start := time.Date(2025, 8, 29, 16, 17, 52, 0, time.FixedZone("-0700", -7*3600))
	end := time.Date(2025, 8, 29, 16, 21, 32, 0, time.FixedZone("-0700", -7*3600))

	report := &models.ReportResponse{
		Pipeline: models.ReportPipeline{
			Name: "default",
			Runs: []models.ReportRun{
				{
					RunNo:     1,
					Status:    "success",
					GitRepo:   "git@gitlab.com:neu-seattle/devops/fa25.git",
					GitBranch: "main",
					GitHash:   "c3aefda",
					Start:     start,
					End:       &end,
				},
			},
		},
	}

	out, err := FormatReportYAML(report)
	if err != nil {
		t.Fatalf("FormatReportYAML: %v", err)
	}

	outStr := string(out)
	if !strings.Contains(outStr, "pipeline:") || !strings.Contains(outStr, "run-no: 1") {
		t.Fatalf("unexpected report yaml output: %s", outStr)
	}
}

func TestFormatReportJSON_NilReport(t *testing.T) {
	if _, err := FormatReportJSON(nil); err == nil {
		t.Fatal("expected error for nil report")
	}
}

// TestFormatReportYAML_JobFailuresField verifies that job output always includes the failures field.
func TestFormatReportYAML_JobFailuresField(t *testing.T) {
	start := time.Date(2025, 9, 1, 10, 0, 0, 0, time.UTC)
	end := time.Date(2025, 9, 1, 10, 5, 0, 0, time.UTC)

	report := &models.ReportResponse{
		Pipeline: models.ReportPipeline{
			Name:   "Default Pipeline",
			RunNo:  1,
			Status: "success",
			Start:  start,
			End:    &end,
			Stage: []models.ReportStage{
				{
					Name:   "build",
					Status: "success",
					Start:  start,
					End:    &end,
					Job: []models.ReportJob{
						{Name: "compile", Status: "success", Start: start, End: &end, Failures: false},
						{Name: "lint", Status: "failed", Start: start, End: &end, Failures: true},
					},
				},
			},
		},
	}

	out, err := FormatReportYAML(report)
	if err != nil {
		t.Fatalf("FormatReportYAML: %v", err)
	}
	outStr := string(out)
	if !strings.Contains(outStr, "failures:") {
		t.Errorf("report YAML must include failures field for jobs; got:\n%s", outStr)
	}
	if !strings.Contains(outStr, "failures: false") {
		t.Errorf("report YAML must include failures: false; got:\n%s", outStr)
	}
	if !strings.Contains(outStr, "failures: true") {
		t.Errorf("report YAML must include failures: true; got:\n%s", outStr)
	}
}

func TestFormatReportJSON_JobFailuresField(t *testing.T) {
	start := time.Date(2025, 9, 1, 10, 0, 0, 0, time.UTC)
	end := time.Date(2025, 9, 1, 10, 5, 0, 0, time.UTC)

	report := &models.ReportResponse{
		Pipeline: models.ReportPipeline{
			Name:   "Default Pipeline",
			RunNo:  1,
			Status: "success",
			Start:  start,
			End:    &end,
			Stage: []models.ReportStage{
				{
					Name:   "build",
					Status: "success",
					Start:  start,
					End:    &end,
					Job: []models.ReportJob{
						{Name: "compile", Status: "success", Start: start, End: &end, Failures: false},
					},
				},
			},
		},
	}

	out, err := FormatReportJSON(report)
	if err != nil {
		t.Fatalf("FormatReportJSON: %v", err)
	}
	outStr := string(out)
	if !strings.Contains(outStr, `"failures"`) {
		t.Errorf("report JSON must include failures field; got:\n%s", outStr)
	}
}

func TestFormatReportYAML_TraceIDField(t *testing.T) {
	start := time.Date(2025, 9, 1, 10, 0, 0, 0, time.UTC)
	end := time.Date(2025, 9, 1, 10, 5, 0, 0, time.UTC)

	report := &models.ReportResponse{
		Pipeline: models.ReportPipeline{
			Name:    "Default Pipeline",
			RunNo:   7,
			Status:  "success",
			TraceID: "4bf92f3577b34da6a3ce929d0e0e4736",
			Start:   start,
			End:     &end,
		},
	}

	out, err := FormatReportYAML(report)
	if err != nil {
		t.Fatalf("FormatReportYAML: %v", err)
	}
	if !strings.Contains(string(out), "trace-id: 4bf92f3577b34da6a3ce929d0e0e4736") {
		t.Fatalf("expected trace-id in YAML output, got:\n%s", string(out))
	}
}

func TestFormatReportJSON_TraceIDField(t *testing.T) {
	start := time.Date(2025, 9, 1, 10, 0, 0, 0, time.UTC)
	end := time.Date(2025, 9, 1, 10, 5, 0, 0, time.UTC)

	report := &models.ReportResponse{
		Pipeline: models.ReportPipeline{
			Name:    "Default Pipeline",
			RunNo:   7,
			Status:  "success",
			TraceID: "4bf92f3577b34da6a3ce929d0e0e4736",
			Start:   start,
			End:     &end,
		},
	}

	out, err := FormatReportJSON(report)
	if err != nil {
		t.Fatalf("FormatReportJSON: %v", err)
	}
	if !strings.Contains(string(out), `"trace-id": "4bf92f3577b34da6a3ce929d0e0e4736"`) {
		t.Fatalf("expected trace-id in JSON output, got:\n%s", string(out))
	}
}
