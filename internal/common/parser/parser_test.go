package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseValidFile(t *testing.T) {
	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.yaml")

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

	err := os.WriteFile(testFile, []byte(yamlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewParser(testFile)
	pipeline, rootNode, err := parser.Parse()

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if pipeline == nil {
		t.Fatal("Expected pipeline to be parsed, got nil")
		return
	}

	if rootNode == nil {
		t.Fatal("Expected rootNode to be parsed, got nil")
		return
	}

	// Check parsed content
	if pipeline.Name != "Test Pipeline" {
		t.Errorf("Expected pipeline name 'Test Pipeline', got '%s'", pipeline.Name)
	}

	if len(pipeline.Stages) != 1 {
		t.Errorf("Expected 1 stage, got %d", len(pipeline.Stages))
	}

	if len(pipeline.Jobs) != 1 {
		t.Errorf("Expected 1 job, got %d", len(pipeline.Jobs))
	}

	// Test GetJobNodes method
	jobNodes := parser.GetJobNodes(rootNode)
	if len(jobNodes) != 1 {
		t.Errorf("Expected 1 job node, got %d", len(jobNodes))
	}
	if jobNodes[0].Name != "compile" {
		t.Errorf("Expected job node name 'compile', got '%s'", jobNodes[0].Name)
	}
}

func TestParseNonExistentFile(t *testing.T) {
	parser := NewParser("nonexistent.yaml")
	_, _, err := parser.Parse()

	if err == nil {
		t.Error("Expected error for nonexistent file, got none")
	}
}

func TestParseInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "invalid.yaml")

	// Invalid YAML (tabs instead of spaces)
	invalidContent := "\tinvalid yaml"
	err := os.WriteFile(testFile, []byte(invalidContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewParser(testFile)
	_, _, err = parser.Parse()

	if err == nil {
		t.Error("Expected error for invalid YAML, got none")
	}
}

func TestGetFilePath(t *testing.T) {
	testPath := "test.yaml"
	parser := NewParser(testPath)

	if parser.GetFilePath() != testPath {
		t.Errorf("Expected file path '%s', got '%s'", testPath, parser.GetFilePath())
	}
}

func TestParseWithDefaultStages(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.yaml")

	yamlContent := `
pipeline:
  name: "Test Pipeline"

compile:
  - stage: build
  - image: "golang:1.21"
  - script:
    - "go build"
`

	err := os.WriteFile(testFile, []byte(yamlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewParser(testFile)
	pipeline, _, err := parser.Parse()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should have default stages
	if len(pipeline.Stages) != 3 {
		t.Errorf("Expected 3 default stages, got %d", len(pipeline.Stages))
	}

	stageNames := []string{pipeline.Stages[0].Name, pipeline.Stages[1].Name, pipeline.Stages[2].Name}
	expectedStages := []string{"build", "test", "docs"}

	for i, expected := range expectedStages {
		if stageNames[i] != expected {
			t.Errorf("Expected stage %d to be '%s', got '%s'", i, expected, stageNames[i])
		}
	}
}

func TestParseMultipleJobs(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.yaml")

	yamlContent := `
pipeline:
  name: "Multi Job Pipeline"

stages:
  - build
  - test

compile:
  - stage: build
  - image: "golang:1.21"
  - script:
    - "go build"

test:
  - stage: test
  - image: "golang:1.21"
  - needs: [compile]
  - script:
    - "go test"
`

	err := os.WriteFile(testFile, []byte(yamlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewParser(testFile)
	pipeline, rootNode, err := parser.Parse()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(pipeline.Jobs) != 2 {
		t.Errorf("Expected 2 jobs, got %d", len(pipeline.Jobs))
	}

	// Test GetJobNodes with multiple jobs
	jobNodes := parser.GetJobNodes(rootNode)
	if len(jobNodes) != 2 {
		t.Errorf("Expected 2 job nodes, got %d", len(jobNodes))
	}
}

func TestParseEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "empty.yaml")

	err := os.WriteFile(testFile, []byte(""), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewParser(testFile)
	_, _, err = parser.Parse()

	if err == nil {
		t.Error("Expected error for empty file, got none")
	}
}

func TestParserContentValid(t *testing.T) {
	yamlContent := `
pipeline:
  name: "Content Pipeline"

stages:
  - build

compile:
  - stage: build
  - script:
    - "go build ./..."
`

	parser := NewParserFromContent(yamlContent)
	pipeline, rootNode, err := parser.Parse()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if pipeline == nil {
		t.Fatal("Expected pipeline to be parsed, got nil")
		return
	}
	if rootNode == nil {
		t.Fatal("Expected root node to be parsed, got nil")
		return
	}
	if pipeline.Name != "Content Pipeline" {
		t.Fatalf("Expected pipeline name Content Pipeline, got %q", pipeline.Name)
	}
	if len(pipeline.Jobs) != 1 {
		t.Fatalf("Expected 1 job, got %d", len(pipeline.Jobs))
	}
}

func TestParserContentInvalidYAML(t *testing.T) {
	parser := NewParserFromContent("\tinvalid yaml")
	_, _, err := parser.Parse()
	if err == nil {
		t.Fatal("Expected parse error, got nil")
	}
}

func TestParseJobFailuresTrue(t *testing.T) {
	yamlContent := `
pipeline:
  name: "Failures True Pipeline"

stages:
  - build

compile:
  - stage: build
  - failures: true
  - script:
    - "go build"
`

	parser := NewParserFromContent(yamlContent)
	pipeline, _, err := parser.Parse()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if len(pipeline.Jobs) != 1 {
		t.Fatalf("Expected 1 job, got %d", len(pipeline.Jobs))
	}
	if !pipeline.Jobs[0].Failures {
		t.Fatal("Expected job failures to be true")
	}
}

func TestParseJobFailuresFalse(t *testing.T) {
	yamlContent := `
pipeline:
  name: "Failures False Pipeline"

stages:
  - build

compile:
  - stage: build
  - failures: false
  - script:
    - "go build"
`

	parser := NewParserFromContent(yamlContent)
	pipeline, _, err := parser.Parse()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if len(pipeline.Jobs) != 1 {
		t.Fatalf("Expected 1 job, got %d", len(pipeline.Jobs))
	}
	if pipeline.Jobs[0].Failures {
		t.Fatal("Expected job failures to be false")
	}
}

func TestParseJobFailuresMissingDefaultsFalse(t *testing.T) {
	yamlContent := `
pipeline:
  name: "Failures Default Pipeline"

stages:
  - build

compile:
  - stage: build
  - script:
    - "go build"
`

	parser := NewParserFromContent(yamlContent)
	pipeline, _, err := parser.Parse()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if len(pipeline.Jobs) != 1 {
		t.Fatalf("Expected 1 job, got %d", len(pipeline.Jobs))
	}
	if pipeline.Jobs[0].Failures {
		t.Fatal("Expected missing failures to default to false")
	}
}
