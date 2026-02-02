package verifier

import (
	"os"
	"strings"
	"testing"

	"github.com/CS7580-SEA-SP26/e-team/internal/models"
	"gopkg.in/yaml.v3"
)

// Helper function to parse YAML string
func parseYAML(t *testing.T, yamlContent string) (*models.Pipeline, *yaml.Node) {
	var pipeline models.Pipeline
	var rootNode yaml.Node

	err := yaml.Unmarshal([]byte(yamlContent), &pipeline)
	if err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}

	err = yaml.Unmarshal([]byte(yamlContent), &rootNode)
	if err != nil {
		t.Fatalf("Failed to parse YAML node: %v", err)
	}

	return &pipeline, &rootNode
}

func TestValidPipeline(t *testing.T) {
	yamlContent := `
name: "Test Pipeline"
stages:
  - name: "build"
jobs:
  - name: "compile"
    stage: "build"
    image: "golang:1.21"
    script:
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
name: "Test Pipeline"
stages: []
jobs:
  - name: "compile"
    stage: "build"
    image: "golang:1.21"
    script:
      - "go build"
`

	pipeline, rootNode := parseYAML(t, yamlContent)
	verifier := NewPipelineVerifier("test.yaml", pipeline, rootNode)

	errors := verifier.Verify()
	if len(errors) == 0 {
		t.Error("Expected error for no stages, got none")
	}

	// Check error message contains expected text
	found := false
	for _, err := range errors {
		if strings.Contains(err.Error(), "at least one stage") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected error about 'at least one stage', got: %v", errors)
	}
}

func TestDuplicateStageNames(t *testing.T) {
	yamlContent := `
name: "Test Pipeline"
stages:
  - name: "build"
  - name: "build"
jobs:
  - name: "compile"
    stage: "build"
    image: "golang:1.21"
    script:
      - "go build"
`

	pipeline, rootNode := parseYAML(t, yamlContent)
	verifier := NewPipelineVerifier("test.yaml", pipeline, rootNode)

	errors := verifier.Verify()
	if len(errors) == 0 {
		t.Error("Expected error for duplicate stage names, got none")
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
  - name: "compile"
    stage: "test"
    image: "golang:1.21"
    script:
      - "go test"
`

	pipeline, rootNode := parseYAML(t, yamlContent)
	verifier := NewPipelineVerifier("test.yaml", pipeline, rootNode)

	errors := verifier.Verify()
	if len(errors) == 0 {
		t.Error("Expected error for duplicate job names, got none")
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
`

	pipeline, rootNode := parseYAML(t, yamlContent)
	verifier := NewPipelineVerifier("test.yaml", pipeline, rootNode)

	errors := verifier.Verify()
	if len(errors) == 0 {
		t.Error("Expected error for empty stage, got none")
	}

	found := false
	for _, err := range errors {
		if strings.Contains(err.Error(), "no jobs assigned") {
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
name: "Test Pipeline"
stages:
  - name: "build"
jobs:
  - name: "compile"
    stage: "build"
    image: "golang:1.21"
    needs: ["nonexistent"]
    script:
      - "go build"
`

	pipeline, rootNode := parseYAML(t, yamlContent)
	verifier := NewPipelineVerifier("test.yaml", pipeline, rootNode)

	errors := verifier.Verify()
	if len(errors) == 0 {
		t.Error("Expected error for undefined job in needs, got none")
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
name: "Test Pipeline"
stages:
  - name: "build"
jobs:
  - name: "compile"
    stage: "build"
    image: "golang:1.21"
    needs: ["compile"]
    script:
      - "go build"
`

	pipeline, rootNode := parseYAML(t, yamlContent)
	verifier := NewPipelineVerifier("test.yaml", pipeline, rootNode)

	errors := verifier.Verify()
	if len(errors) == 0 {
		t.Error("Expected error for self dependency, got none")
	}

	found := false
	for _, err := range errors {
		msg := err.Error()
		if strings.Contains(msg, "cannot depend on itself") || strings.Contains(msg, "cycle") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected error about self-dependency or cycle, got: %v", errors)
	}
}

func TestCircularDependency(t *testing.T) {
	yamlContent := `
name: "Test Pipeline"
stages:
  - name: "build"
jobs:
  - name: "J1"
    stage: "build"
    image: "golang:1.21"
    needs: ["J2"]
    script:
      - "echo J1"
  - name: "J2"
    stage: "build"
    image: "golang:1.21"
    needs: ["J3"]
    script:
      - "echo J2"
  - name: "J3"
    stage: "build"
    image: "golang:1.21"
    needs: ["J1"]
    script:
      - "echo J3"
`

	pipeline, rootNode := parseYAML(t, yamlContent)
	verifier := NewPipelineVerifier("test.yaml", pipeline, rootNode)

	errors := verifier.Verify()
	if len(errors) == 0 {
		t.Error("Expected error for circular dependency, got none")
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
name: "Test Pipeline"
stages:
  - name: "build"
jobs:
  - name: "compile"
    stage: "nonexistent"
    image: "golang:1.21"
    script:
      - "go build"
`

	pipeline, rootNode := parseYAML(t, yamlContent)
	verifier := NewPipelineVerifier("test.yaml", pipeline, rootNode)

	errors := verifier.Verify()
	if len(errors) == 0 {
		t.Error("Expected error for undefined stage reference, got none")
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

func TestMissingRequiredFields(t *testing.T) {
	yamlContent := `
name: "Test Pipeline"
stages:
  - name: "build"
jobs:
  - name: "compile"
    stage: "build"
    script:
      - "go build"
`

	pipeline, rootNode := parseYAML(t, yamlContent)
	verifier := NewPipelineVerifier("test.yaml", pipeline, rootNode)

	errors := verifier.Verify()
	if len(errors) == 0 {
		t.Error("Expected error for missing image field, got none")
	}

	found := false
	for _, err := range errors {
		if strings.Contains(err.Error(), "must have an `image` field") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected error about missing image field, got: %v", errors)
	}
}

func TestPipelineFormat(t *testing.T) {
	data, err := os.ReadFile(".pipelines/prof_example.yaml")
	if err != nil {
		t.Fatalf("Failed to read pipeline example: %v", err)
	}

	pipeline, rootNode := parseYAML(t, string(data))
	verifier := NewPipelineVerifier(".pipelines/prof_example.yaml", pipeline, rootNode)

	errors := verifier.Verify()
	if len(errors) > 0 {
		t.Fatalf("Expected pipeline to be valid, got errors: %v", errors)
	}
}
