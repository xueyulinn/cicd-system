package cli

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// writeTempPipelineInGitRepo creates a temp dir, runs "git init", writes pipeline content
// to pipeline.yaml there, and changes cwd to that dir so checkGitRepo() passes.
// Caller must call the returned cleanup func (e.g. defer cleanup()).
func writeTempPipelineInGitRepo(t *testing.T, content string) (configPath string, cleanup func()) {
	t.Helper()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "pipeline.yaml")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
		t.Fatalf("Failed to write pipeline file: %v", err)
	}
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	return path, func() { _ = os.Chdir(origWd) }
}

func TestRunVerify_ValidFile(t *testing.T) {
	startValidationGatewayServer(t)

	configPath, cleanup := writeTempPipelineInGitRepo(t, `
pipeline:
  name: "Test Pipeline"

stages:
  - build

compile:
  - stage: build
  - image: golang:1.21
  - script:
    - "go build"
`)
	defer cleanup()

	output, err := captureStdout(t, func() error {
		return runVerify(&cobra.Command{}, []string{configPath})
	})
	if err != nil {
		t.Fatalf("runVerify returned error: %v", err)
	}
	if !strings.Contains(output, "Configuration is valid") {
		t.Errorf("Expected success message in output, got:\n%s", output)
	}
}

func TestRunVerify_InvalidFile_ReturnsError(t *testing.T) {
	startValidationGatewayServer(t)

	configPath, cleanup := writeTempPipelineInGitRepo(t, `
pipeline:
  name: "Test"

stages:
  - build
  - test

compile:
  - stage: build
  - image: golang:1.21
  - script:
    - "go build"
`)
	defer cleanup()

	_, err := captureStdout(t, func() error {
		return runVerify(&cobra.Command{}, []string{configPath})
	})
	if err == nil {
		t.Fatal("Expected error for stage with no jobs, got nil")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Errorf("Expected validation failed error, got: %v", err)
	}
}

func TestRunVerify_MissingFile_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}
	origWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	configPath := filepath.Join(tmpDir, "does-not-exist.yaml")

	_, err := captureStdout(t, func() error {
		return runVerify(&cobra.Command{}, []string{configPath})
	})
	if err == nil {
		t.Fatal("Expected error for missing file, got nil")
	}
	// runVerify returns the error from os.Stat (platform-specific message)
	if !strings.Contains(err.Error(), "stat") &&
		!strings.Contains(err.Error(), "no such file") &&
		!strings.Contains(err.Error(), "GetFileAttributesEx") &&
		!strings.Contains(err.Error(), "cannot find the file") {
		t.Errorf("Expected stat/no such file error, got: %v", err)
	}
}

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	original := os.Stdout
	os.Stdout = writer
	errRun := fn()
	_ = writer.Close()
	os.Stdout = original
	bytes, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil {
		t.Fatalf("Failed to read stdout: %v", err)
	}
	return string(bytes), errRun
}
