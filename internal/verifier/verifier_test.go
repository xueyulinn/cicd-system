package verifier

import (
	"os"
	"strings"
	"testing"

	"github.com/CS7580-SEA-SP26/e-team/internal/models"
	"github.com/CS7580-SEA-SP26/e-team/internal/parser"
	"gopkg.in/yaml.v3"
)

// Helper function to parse YAML string using the parser
func parseYAML(t *testing.T, yamlContent string) (*models.Pipeline, *yaml.Node) {
	// Create a temporary file for parsing
	tmpFile := t.TempDir() + "/test.yaml"
	err := os.WriteFile(tmpFile, []byte(yamlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	// Use the parser to parse the file
	p := parser.NewParser(tmpFile)
	pipeline, rootNode, err := p.Parse()
	if err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}

	return pipeline, rootNode
}

func TestValidPipeline(t *testing.T) {
	yamlContent := `
pipeline:
  name: "Test Pipeline"

stages:
  - build

compile:
  - stage: build
  - image: "golang:1.21"
  - script:
    - "go build"
`

	pipeline, rootNode := parseYAML(t, yamlContent)
	verifier := NewPipelineVerifier("test.yaml", pipeline, rootNode)

	errors := verifier.Verify()
	if len(errors) > 0 {
		t.Errorf("Expected no errors, got %d: %v", len(errors), errors)
	}
}

func TestNoStages(t *testing.T) {
	yamlContent := `
pipeline:
  name: "Test Pipeline"

compile:
  - stage: build
  - image: "golang:1.21"
  - script:
    - "go build"
`

	pipeline, rootNode := parseYAML(t, yamlContent)
	verifier := NewPipelineVerifier("test.yaml", pipeline, rootNode)

	errors := verifier.Verify()
	if len(errors) == 0 {
		t.Error("Expected errors, got none")
	}

	// Should have errors about default stages having no jobs
	found := false
	for _, err := range errors {
		if strings.Contains(err.Error(), "has no jobs assigned") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected error about empty stages, got: %v", errors)
	}
}

func TestDuplicateStageNames(t *testing.T) {
	yamlContent := `
pipeline:
  name: "Test Pipeline"

stages:
  - build
  - build

compile:
  - stage: build
  - image: "golang:1.21"
  - script:
    - "go build"
`

	pipeline, rootNode := parseYAML(t, yamlContent)
	verifier := NewPipelineVerifier("test.yaml", pipeline, rootNode)

	errors := verifier.Verify()
	if len(errors) == 0 {
		t.Error("Expected error about 'duplicate stage name', got none")
	}

	found := false
	for _, err := range errors {
		if strings.Contains(err.Error(), "duplicate stage name") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected error about 'duplicate stage name', got: %v", errors)
	}
}

func TestDuplicateJobNames(t *testing.T) {
	yamlContent := `
pipeline:
  name: "Test Pipeline"

stages:
  - build

compile:
  - stage: build
  - image: "golang:1.21"
  - script:
    - "go build"

compile:
  - stage: build
  - image: "golang:1.21"
  - script:
    - "go test"
`

	pipeline, rootNode := parseYAML(t, yamlContent)
	verifier := NewPipelineVerifier("test.yaml", pipeline, rootNode)

	errors := verifier.Verify()
	if len(errors) == 0 {
		t.Error("Expected error about 'duplicate job name', got none")
	}

	found := false
	for _, err := range errors {
		if strings.Contains(err.Error(), "duplicate job name") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected error about 'duplicate job name', got: %v", errors)
	}
}

func TestEmptyStage(t *testing.T) {
	yamlContent := `
pipeline:
  name: "Test Pipeline"

stages:
  - build
  - test

compile:
  - stage: build
  - image: "golang:1.21"
  - script:
    - "go build"
`

	pipeline, rootNode := parseYAML(t, yamlContent)
	verifier := NewPipelineVerifier("test.yaml", pipeline, rootNode)

	errors := verifier.Verify()
	if len(errors) == 0 {
		t.Error("Expected error about 'no jobs assigned', got none")
	}

	found := false
	for _, err := range errors {
		if strings.Contains(err.Error(), "has no jobs assigned") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected error about 'no jobs assigned', got: %v", errors)
	}
}

func TestUndefinedJobInNeeds(t *testing.T) {
	yamlContent := `
pipeline:
  name: "Test Pipeline"

stages:
  - build

compile:
  - stage: build
  - image: "golang:1.21"
  - script:
    - "go build"

test:
  - stage: build
  - image: "golang:1.21"
  - needs: [non-existent-job]
  - script:
    - "go test"
`

	pipeline, rootNode := parseYAML(t, yamlContent)
	verifier := NewPipelineVerifier("test.yaml", pipeline, rootNode)

	errors := verifier.Verify()
	if len(errors) == 0 {
		t.Error("Expected error about 'undefined job', got none")
	}

	found := false
	for _, err := range errors {
		if strings.Contains(err.Error(), "undefined job") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected error about 'undefined job', got: %v", errors)
	}
}

func TestSelfDependency(t *testing.T) {
	yamlContent := `
pipeline:
  name: "Test Pipeline"

stages:
  - build

compile:
  - stage: build
  - image: "golang:1.21"
  - needs: [compile]
  - script:
    - "go build"
`

	pipeline, rootNode := parseYAML(t, yamlContent)
	verifier := NewPipelineVerifier("test.yaml", pipeline, rootNode)

	errors := verifier.Verify()
	if len(errors) == 0 {
		t.Error("Expected error about self-dependency or cycle, got none")
	}

	found := false
	for _, err := range errors {
		if strings.Contains(err.Error(), "cycle detected") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected error about 'cycle detected', got: %v", errors)
	}
}

func TestCircularDependency(t *testing.T) {
	yamlContent := `
pipeline:
  name: "Test Pipeline"

stages:
  - build

job1:
  - stage: build
  - image: "golang:1.21"
  - needs: [job2]
  - script:
    - "go build"

job2:
  - stage: build
  - image: "golang:1.21"
  - needs: [job1]
  - script:
    - "go test"
`

	pipeline, rootNode := parseYAML(t, yamlContent)
	verifier := NewPipelineVerifier("test.yaml", pipeline, rootNode)

	errors := verifier.Verify()
	if len(errors) == 0 {
		t.Error("Expected error about 'cycle detected', got none")
	}

	found := false
	for _, err := range errors {
		if strings.Contains(err.Error(), "cycle detected") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected error about 'cycle detected', got: %v", errors)
	}
}

func TestJobReferencesUndefinedStage(t *testing.T) {
	yamlContent := `
pipeline:
  name: "Test Pipeline"

stages:
  - build

compile:
  - stage: nonexistent-stage
  - image: "golang:1.21"
  - script:
    - "go build"
`

	pipeline, rootNode := parseYAML(t, yamlContent)
	verifier := NewPipelineVerifier("test.yaml", pipeline, rootNode)

	errors := verifier.Verify()
	if len(errors) == 0 {
		t.Error("Expected error about 'undefined stage', got none")
	}

	found := false
	for _, err := range errors {
		if strings.Contains(err.Error(), "undefined stage") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected error about 'undefined stage', got: %v", errors)
	}
}

func TestValidStringScript(t *testing.T) {
	yamlContent := `
pipeline:
  name: "Test Pipeline"

stages:
  - build

compile:
  - stage: build
  - image: "golang:1.21"
  - script: "go build"
`

	pipeline, rootNode := parseYAML(t, yamlContent)
	verifier := NewPipelineVerifier("test.yaml", pipeline, rootNode)

	errors := verifier.Verify()
	if len(errors) > 0 {
		t.Errorf("Expected no errors for string script, got %d: %v", len(errors), errors)
	}
}

func TestNewPipelineVerifier(t *testing.T) {
	yamlContent := `
pipeline:
  name: "Test Pipeline"

stages:
  - build

compile:
  - stage: build
  - image: "golang:1.21"
  - script:
    - "go build"
`

	pipeline, rootNode := parseYAML(t, yamlContent)
	verifier := NewPipelineVerifier("test.yaml", pipeline, rootNode)

	if verifier == nil {
		t.Fatal("Expected verifier to be created, got nil")
	}

	if verifier.filePath != "test.yaml" {
		t.Errorf("Expected filePath 'test.yaml', got '%s'", verifier.filePath)
	}

	if verifier.pipeline != pipeline {
		t.Error("Expected pipeline to be set")
	}

	if verifier.rootNode != rootNode {
		t.Error("Expected rootNode to be set")
	}
}
