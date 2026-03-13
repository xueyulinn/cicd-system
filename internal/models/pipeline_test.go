package models

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestPipelineUnmarshal(t *testing.T) {
	yamlContent := `
name: "Test Pipeline"
stages:
  - name: "build"
  - name: "test"
jobs:
  - name: "compile"
    stage: "build"
    image: "golang:1.21"
    script:
      - "go build"
  - name: "unit-test"
    stage: "test"
    image: "golang:1.21"
    needs: ["compile"]
    script:
      - "go test"
`

	var pipeline Pipeline
	err := yaml.Unmarshal([]byte(yamlContent), &pipeline)
	if err != nil {
		t.Fatalf("Failed to unmarshal pipeline: %v", err)
	}

	// Test pipeline name
	if pipeline.Name != "Test Pipeline" {
		t.Errorf("Expected name 'Test Pipeline', got '%s'", pipeline.Name)
	}

	// Test stages
	if len(pipeline.Stages) != 2 {
		t.Fatalf("Expected 2 stages, got %d", len(pipeline.Stages))
	}

	if pipeline.Stages[0].Name != "build" {
		t.Errorf("Expected first stage 'build', got '%s'", pipeline.Stages[0].Name)
	}

	if pipeline.Stages[1].Name != "test" {
		t.Errorf("Expected second stage 'test', got '%s'", pipeline.Stages[1].Name)
	}

	// Test jobs
	if len(pipeline.Jobs) != 2 {
		t.Fatalf("Expected 2 jobs, got %d", len(pipeline.Jobs))
	}

	// Test first job
	job1 := pipeline.Jobs[0]
	if job1.Name != "compile" {
		t.Errorf("Expected job name 'compile', got '%s'", job1.Name)
	}
	if job1.Stage != "build" {
		t.Errorf("Expected job stage 'build', got '%s'", job1.Stage)
	}
	if job1.Image != "golang:1.21" {
		t.Errorf("Expected job image 'golang:1.21', got '%s'", job1.Image)
	}
	if len(job1.Script) != 1 {
		t.Errorf("Expected 1 script command, got %d", len(job1.Script))
	}
	if len(job1.Needs) != 0 {
		t.Errorf("Expected no needs, got %d", len(job1.Needs))
	}

	// Test second job
	job2 := pipeline.Jobs[1]
	if job2.Name != "unit-test" {
		t.Errorf("Expected job name 'unit-test', got '%s'", job2.Name)
	}
	if len(job2.Needs) != 1 {
		t.Fatalf("Expected 1 need, got %d", len(job2.Needs))
	}
	if job2.Needs[0] != "compile" {
		t.Errorf("Expected need 'compile', got '%s'", job2.Needs[0])
	}
}

func TestStageUnmarshal(t *testing.T) {
	yamlContent := `
name: "build"
`

	var stage Stage
	err := yaml.Unmarshal([]byte(yamlContent), &stage)
	if err != nil {
		t.Fatalf("Failed to unmarshal stage: %v", err)
	}

	if stage.Name != "build" {
		t.Errorf("Expected stage name 'build', got '%s'", stage.Name)
	}
}

func TestJobUnmarshal(t *testing.T) {
	yamlContent := `
name: "compile"
stage: "build"
image: "golang:1.21"
script:
  - "go build"
  - "go test"
needs:
  - "setup"
`

	var job Job
	err := yaml.Unmarshal([]byte(yamlContent), &job)
	if err != nil {
		t.Fatalf("Failed to unmarshal job: %v", err)
	}

	if job.Name != "compile" {
		t.Errorf("Expected job name 'compile', got '%s'", job.Name)
	}

	if job.Stage != "build" {
		t.Errorf("Expected stage 'build', got '%s'", job.Stage)
	}

	if job.Image != "golang:1.21" {
		t.Errorf("Expected image 'golang:1.21', got '%s'", job.Image)
	}

	if len(job.Script) != 2 {
		t.Fatalf("Expected 2 script commands, got %d", len(job.Script))
	}

	if job.Script[0] != "go build" {
		t.Errorf("Expected first script 'go build', got '%s'", job.Script[0])
	}

	if len(job.Needs) != 1 {
		t.Fatalf("Expected 1 need, got %d", len(job.Needs))
	}

	if job.Needs[0] != "setup" {
		t.Errorf("Expected need 'setup', got '%s'", job.Needs[0])
	}

	if job.Failures {
		t.Errorf("Expected failures to default to false, got true")
	}
}

func TestJobWithoutOptionalFields(t *testing.T) {
	yamlContent := `
name: "simple-job"
stage: "build"
image: "alpine"
script:
  - "echo hello"
`

	var job Job
	err := yaml.Unmarshal([]byte(yamlContent), &job)
	if err != nil {
		t.Fatalf("Failed to unmarshal job: %v", err)
	}

	if len(job.Needs) != 0 {
		t.Errorf("Expected no needs for job without needs field, got %d", len(job.Needs))
	}
}

func TestValidationErrorError(t *testing.T) {
	err := &ValidationError{
		FilePath: "test.yaml",
		Location: Location{Line: 5, Column: 10},
		Message:  "some error",
	}

	expected := "test.yaml:5:10: some error"
	if err.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
	}
}

func TestEmptyPipeline(t *testing.T) {
	yamlContent := `
name: ""
stages: []
jobs: []
`

	var pipeline Pipeline
	err := yaml.Unmarshal([]byte(yamlContent), &pipeline)
	if err != nil {
		t.Fatalf("Failed to unmarshal empty pipeline: %v", err)
	}

	if pipeline.Name != "" {
		t.Errorf("Expected empty name, got '%s'", pipeline.Name)
	}

	if len(pipeline.Stages) != 0 {
		t.Errorf("Expected 0 stages, got %d", len(pipeline.Stages))
	}

	if len(pipeline.Jobs) != 0 {
		t.Errorf("Expected 0 jobs, got %d", len(pipeline.Jobs))
	}
}

func TestPipelineMarshal(t *testing.T) {
	pipeline := Pipeline{
		Name: "Test Pipeline",
		Stages: []Stage{
			{Name: "build"},
			{Name: "test"},
		},
		Jobs: []Job{
			{
				Name:     "compile",
				Stage:    "build",
				Image:    "golang:1.21",
				Script:   []string{"go build"},
				Failures: true,
			},
		},
	}

	data, err := yaml.Marshal(&pipeline)
	if err != nil {
		t.Fatalf("Failed to marshal pipeline: %v", err)
	}

	// Unmarshal it back
	var unmarshaled Pipeline
	err = yaml.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal marshaled data: %v", err)
	}

	if unmarshaled.Name != pipeline.Name {
		t.Errorf("Name mismatch after marshal/unmarshal")
	}

	if len(unmarshaled.Stages) != len(pipeline.Stages) {
		t.Errorf("Stages count mismatch after marshal/unmarshal")
	}

	if len(unmarshaled.Jobs) != len(pipeline.Jobs) {
		t.Errorf("Jobs count mismatch after marshal/unmarshal")
	}

	if !unmarshaled.Jobs[0].Failures {
		t.Errorf("Expected failures field to survive marshal/unmarshal")
	}
}

func TestStageUnmarshalString(t *testing.T) {
	yamlContent := `"build"`

	var stage Stage
	err := yaml.Unmarshal([]byte(yamlContent), &stage)
	if err != nil {
		t.Fatalf("Failed to unmarshal stage string: %v", err)
	}

	if stage.Name != "build" {
		t.Errorf("Expected stage name 'build', got '%s'", stage.Name)
	}
}

func TestStageUnmarshalObject(t *testing.T) {
	yamlContent := `
name: "build"
description: "Build stage"
`

	var stage Stage
	err := yaml.Unmarshal([]byte(yamlContent), &stage)
	if err != nil {
		t.Fatalf("Failed to unmarshal stage object: %v", err)
	}

	if stage.Name != "build" {
		t.Errorf("Expected stage name 'build', got '%s'", stage.Name)
	}
}

func TestJobUnmarshalMinimal(t *testing.T) {
	yamlContent := `
name: "simple-job"
stage: "build"
image: "alpine"
script:
  - "echo hello"
`

	var job Job
	err := yaml.Unmarshal([]byte(yamlContent), &job)
	if err != nil {
		t.Fatalf("Failed to unmarshal job: %v", err)
	}

	if job.Name != "simple-job" {
		t.Errorf("Expected job name 'simple-job', got '%s'", job.Name)
	}

	if len(job.Script) != 1 {
		t.Errorf("Expected 1 script command, got %d", len(job.Script))
	}

	if job.Script[0] != "echo hello" {
		t.Errorf("Expected script 'echo hello', got '%s'", job.Script[0])
	}

	if job.Failures {
		t.Errorf("Expected missing failures to default to false, got true")
	}
}

func TestJobUnmarshalFailuresTrue(t *testing.T) {
	yamlContent := `
name: "simple-job"
stage: "build"
failures: true
image: "alpine"
script:
  - "echo hello"
`

	var job Job
	err := yaml.Unmarshal([]byte(yamlContent), &job)
	if err != nil {
		t.Fatalf("Failed to unmarshal job: %v", err)
	}

	if !job.Failures {
		t.Errorf("Expected failures to be true, got false")
	}
}

func TestJobMarshalJSONIncludesFailures(t *testing.T) {
	job := Job{
		Name:     "simple-job",
		Stage:    "build",
		Failures: true,
	}

	data, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("Failed to marshal job to JSON: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("Failed to unmarshal marshaled job JSON: %v", err)
	}

	failures, ok := payload["failures"].(bool)
	if !ok {
		t.Fatalf("Expected failures key in JSON output, got %v", payload)
	}
	if !failures {
		t.Errorf("Expected failures to marshal as true, got false")
	}
}

func TestJobUnmarshalStringScript(t *testing.T) {
	yamlContent := `
name: "job-with-string-script"
stage: "build"
image: "alpine"
script: "echo hello"
`

	var job Job
	err := yaml.Unmarshal([]byte(yamlContent), &job)
	// This should fail because Job.Script is []string, not string
	if err == nil {
		t.Error("Expected error for string script field, got none")
	}
}

func TestValidationErrorWithEmptyMessage(t *testing.T) {
	err := &ValidationError{
		FilePath: "test.yaml",
		Location: Location{Line: 1, Column: 1},
		Message:  "",
	}

	expected := "test.yaml:1:1: "
	if err.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
	}
}

func TestValidationErrorWithEmptyFilePath(t *testing.T) {
	err := &ValidationError{
		FilePath: "",
		Location: Location{Line: 1, Column: 1},
		Message:  "test error",
	}

	expected := ":1:1: test error"
	if err.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
	}
}

func TestPipelineWithComplexNeeds(t *testing.T) {
	yamlContent := `
name: "Complex Pipeline"
stages:
  - name: "build"
  - name: "test"
jobs:
  - name: "compile"
    stage: "build"
    image: "golang:1.21"
    script:
      - "go build"
  - name: "unit-test"
    stage: "test"
    image: "golang:1.21"
    needs: ["compile", "lint"]
    script:
      - "go test"
  - name: "integration-test"
    stage: "test"
    image: "golang:1.21"
    needs: ["unit-test"]
    script:
      - "go test -integration"
`

	var pipeline Pipeline
	err := yaml.Unmarshal([]byte(yamlContent), &pipeline)
	if err != nil {
		t.Fatalf("Failed to unmarshal pipeline: %v", err)
	}

	// Test job with multiple needs
	unitTest := pipeline.Jobs[1]
	if len(unitTest.Needs) != 2 {
		t.Errorf("Expected 2 needs for unit-test, got %d", len(unitTest.Needs))
	}

	if unitTest.Needs[0] != "compile" || unitTest.Needs[1] != "lint" {
		t.Errorf("Expected needs [compile, lint], got %v", unitTest.Needs)
	}

	// Test job with single need
	integrationTest := pipeline.Jobs[2]
	if len(integrationTest.Needs) != 1 {
		t.Errorf("Expected 1 need for integration-test, got %d", len(integrationTest.Needs))
	}

	if integrationTest.Needs[0] != "unit-test" {
		t.Errorf("Expected need [unit-test], got %v", integrationTest.Needs)
	}
}
