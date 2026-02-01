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

	err := os.WriteFile(testFile, []byte(yamlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewParser(testFile)
	pipeline, rootNode, err := parser.Parse()

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if pipeline == nil {
		t.Error("Expected pipeline to be parsed, got nil")
	}

	if rootNode == nil {
		t.Error("Expected rootNode to be parsed, got nil")
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
