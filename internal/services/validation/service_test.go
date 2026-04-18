package validation

import (
	"strings"
	"testing"

	"github.com/xueyulinn/cicd-system/internal/models"
)

func TestValidateYAMLValidPipeline(t *testing.T) {
	svc := NewService()

	resp := svc.ValidateYAML(validPipelineYAML)
	if !resp.Valid {
		t.Fatalf("ValidateYAML valid = false, errors = %+v", resp.Errors)
	}
}

func TestValidateYAMLInvalidPipeline(t *testing.T) {
	svc := NewService()

	resp := svc.ValidateYAML(invalidPipelineYAML)
	if resp.Valid {
		t.Fatal("expected invalid response")
	}
	if len(resp.Errors) == 0 {
		t.Fatal("expected validation errors")
	}
}

func TestDryRunYAMLValidPipeline(t *testing.T) {
	svc := NewService()

	resp := svc.DryRunYAML(validPipelineYAML)
	if !resp.Valid {
		t.Fatalf("DryRunYAML valid = false, errors = %+v", resp.Errors)
	}
	if !strings.Contains(resp.Output, "build:") || !strings.Contains(resp.Output, "compile:") {
		t.Fatalf("unexpected dryrun output: %q", resp.Output)
	}
}

func TestDryRunYAMLInvalidPipeline(t *testing.T) {
	svc := NewService()

	resp := svc.DryRunYAML(invalidPipelineYAML)
	if resp.Valid {
		t.Fatal("expected invalid response")
	}
	if len(resp.Errors) == 0 {
		t.Fatal("expected validation errors")
	}
	if resp.Output != "" {
		t.Fatalf("expected empty output for invalid pipeline, got %q", resp.Output)
	}
}

func TestValidateYAMLParserError(t *testing.T) {
	svc := NewService()

	resp := svc.ValidateYAML(":::")
	if resp.Valid {
		t.Fatal("expected invalid response for malformed YAML")
	}
	if len(resp.Errors) == 0 {
		t.Fatal("expected parser errors")
	}
}

func TestMarshalExecutionPlan(t *testing.T) {
	plan := &models.ExecutionPlan{
		Stages: []models.StageExecutionPlan{
			{
				Name: "build",
				Jobs: []models.JobExecutionPlan{
					{
						Name:   "compile",
						Image:  "golang:1.25",
						Script: []string{"go test ./...", "go build ./..."},
					},
				},
			},
		},
	}

	out, err := marshalExecutionPlan(plan)
	if err != nil {
		t.Fatalf("marshalExecutionPlan error = %v", err)
	}
	if !strings.Contains(out, "build:") {
		t.Fatalf("expected stage in output, got %q", out)
	}
	if !strings.Contains(out, "compile:") {
		t.Fatalf("expected job in output, got %q", out)
	}
	if !strings.Contains(out, "- go test ./...") {
		t.Fatalf("expected script line in output, got %q", out)
	}
}
