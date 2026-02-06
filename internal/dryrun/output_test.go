package dryrun

import (
	"reflect"
	"strings"
	"testing"

	"github.com/CS7580-SEA-SP26/e-team/internal/models"
)

func TestBuildDryRunOutput_SingleStageSingleJob(t *testing.T) {
	pipeline := &models.Pipeline{
		Stages: []models.Stage{{Name: "build"}},
		Jobs: []models.Job{
			{Name: "compile", Stage: "build", Image: "golang:1.21", Script: []string{"go build"}},
		},
	}

	output := BuildDryRunOutput(pipeline)

	if output == nil {
		t.Fatal("Expected non-nil output")
	}
	if _, ok := output["build"]; !ok {
		t.Fatal("Expected 'build' stage in output")
	}
	if _, ok := output["build"]["compile"]; !ok {
		t.Fatal("Expected 'compile' job in build stage")
	}
	jobOut := output["build"]["compile"]
	if jobOut.Image != "golang:1.21" {
		t.Errorf("Expected image 'golang:1.21', got %q", jobOut.Image)
	}
	if !reflect.DeepEqual(jobOut.Script, []string{"go build"}) {
		t.Errorf("Expected script [\"go build\"], got %v", jobOut.Script)
	}
}

func TestBuildDryRunOutput_MultipleStagesMultipleJobs(t *testing.T) {
	pipeline := &models.Pipeline{
		Stages: []models.Stage{{Name: "build"}, {Name: "test"}},
		Jobs: []models.Job{
			{Name: "compile", Stage: "build", Image: "golang:1.21", Script: []string{"go build"}},
			{Name: "unit-tests", Stage: "test", Image: "golang:1.21", Script: []string{"go test"}},
			{Name: "integration-tests", Stage: "test", Image: "golang:1.21", Script: []string{"go test ./..."}},
		},
	}

	output := BuildDryRunOutput(pipeline)

	if output == nil {
		t.Fatal("Expected non-nil output")
	}

	// Build stage
	if _, ok := output["build"]; !ok {
		t.Fatal("Expected 'build' stage")
	}
	if len(output["build"]) != 1 {
		t.Errorf("Expected 1 job in build, got %d", len(output["build"]))
	}
	if out := output["build"]["compile"]; out.Image != "golang:1.21" {
		t.Errorf("Expected compile image 'golang:1.21', got %q", out.Image)
	}

	// Test stage
	if _, ok := output["test"]; !ok {
		t.Fatal("Expected 'test' stage")
	}
	if len(output["test"]) != 2 {
		t.Errorf("Expected 2 jobs in test, got %d", len(output["test"]))
	}
	if out := output["test"]["unit-tests"]; out.Image != "golang:1.21" {
		t.Errorf("Expected unit-tests image 'golang:1.21', got %q", out.Image)
	}
	if out := output["test"]["integration-tests"]; out.Script[0] != "go test ./..." {
		t.Errorf("Expected integration-tests script, got %v", out.Script)
	}
}

func TestBuildDryRunOutput_EmptyStages(t *testing.T) {
	pipeline := &models.Pipeline{
		Stages: []models.Stage{},
		Jobs:   []models.Job{},
	}

	output := BuildDryRunOutput(pipeline)

	if output == nil {
		t.Fatal("Expected non-nil output")
	}
	if len(output) != 0 {
		t.Errorf("Expected empty output, got %d stages", len(output))
	}
}

func TestBuildDryRunOutput_StageWithNoJobs(t *testing.T) {
	pipeline := &models.Pipeline{
		Stages: []models.Stage{{Name: "build"}, {Name: "empty-stage"}},
		Jobs: []models.Job{
			{Name: "compile", Stage: "build", Image: "golang:1.21", Script: []string{"go build"}},
		},
	}

	output := BuildDryRunOutput(pipeline)

	if _, ok := output["build"]; !ok {
		t.Fatal("Expected 'build' stage")
	}
	if _, ok := output["empty-stage"]; !ok {
		t.Fatal("Expected 'empty-stage' in output")
	}
	if len(output["empty-stage"]) != 0 {
		t.Errorf("Expected empty-stage to have 0 jobs, got %d", len(output["empty-stage"]))
	}
}

func TestBuildDryRunOutput_JobWithMultipleScriptLines(t *testing.T) {
	pipeline := &models.Pipeline{
		Stages: []models.Stage{{Name: "build"}},
		Jobs: []models.Job{
			{
				Name:   "build",
				Stage:  "build",
				Image:  "gradle:8.12-jdk21",
				Script: []string{"./gradlew classes", "./gradlew jar"},
			},
		},
	}

	output := BuildDryRunOutput(pipeline)

	jobOut := output["build"]["build"]
	expected := []string{"./gradlew classes", "./gradlew jar"}
	if !reflect.DeepEqual(jobOut.Script, expected) {
		t.Errorf("Expected script %v, got %v", expected, jobOut.Script)
	}
	if jobOut.Image != "gradle:8.12-jdk21" {
		t.Errorf("Expected image 'gradle:8.12-jdk21', got %q", jobOut.Image)
	}
}

func TestBuildDryRunOutput_StageOrderPreservedInYAML(t *testing.T) {
	pipeline := &models.Pipeline{
		Stages: []models.Stage{{Name: "build"}, {Name: "test"}, {Name: "doc"}, {Name: "deploy"}},
		Jobs: []models.Job{
			{Name: "compile", Stage: "build", Image: "gradle:jdk21", Script: []string{"gradle build"}},
			{Name: "unit-tests", Stage: "test", Image: "gradle:jdk21", Script: []string{"gradle test"}},
			{Name: "javadoc", Stage: "doc", Image: "gradle:jdk21", Script: []string{"gradle javadoc"}},
			{Name: "package", Stage: "deploy", Image: "gradle:jdk21", Script: []string{"gradle assembleDist"}},
		},
	}

	output := BuildDryRunOutput(pipeline)
	bytes, err := MarshalOutputStruct(output, pipeline.Stages)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}
	yamlStr := string(bytes)

	// Stages should appear in declaration order: build, test, doc, deploy
	buildIdx := strings.Index(yamlStr, "build:")
	testIdx := strings.Index(yamlStr, "test:")
	docIdx := strings.Index(yamlStr, "doc:")
	deployIdx := strings.Index(yamlStr, "deploy:")
	if buildIdx == -1 || testIdx == -1 || docIdx == -1 || deployIdx == -1 {
		t.Fatalf("Expected all stages in output, got: %s", yamlStr)
	}
	if !(buildIdx < testIdx && testIdx < docIdx && docIdx < deployIdx) {
		t.Errorf("Stages should be in order build, test, doc, deploy. YAML:\n%s", yamlStr)
	}
}
