package cli

import (
	"strings"
	"testing"

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
