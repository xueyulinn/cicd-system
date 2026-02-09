package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRunDryRun_PrintsOrderedJobs(t *testing.T) {
	configPath := writeTempPipeline(t, `
pipeline:
  name: "Test Pipeline"

stages:
  - build
  - test

compile:
  - stage: build
  - image: golang:1.21
  - script:
    - "make build"

unit-tests:
  - stage: test
  - image: golang:1.21
  - script:
    - "make test"

integration-tests:
  - stage: test
  - needs: [unit-tests]
  - image: golang:1.21
  - script:
    - "make integration"
`)

	output, err := captureStdout(t, func() error {
		return runDryRun(&cobra.Command{}, []string{configPath})
	})
	if err != nil {
		t.Fatalf("runDryRun returned error: %v", err)
	}

	buildIdx := strings.Index(output, "build:")
	testIdx := strings.Index(output, "test:")
	if buildIdx == -1 || testIdx == -1 {
		t.Fatalf("Expected build/test stages in output, got:\n%s", output)
	}
	if buildIdx >= testIdx {
		t.Errorf("Expected build before test, got:\n%s", output)
	}

	unitIdx := strings.Index(output, "unit-tests:")
	integrationIdx := strings.Index(output, "integration-tests:")
	if unitIdx == -1 || integrationIdx == -1 {
		t.Fatalf("Expected unit/integration jobs in output, got:\n%s", output)
	}
	if unitIdx >= integrationIdx {
		t.Errorf("Expected unit-tests before integration-tests, got:\n%s", output)
	}
}

func TestRunDryRun_FailsOnEmptyStage(t *testing.T) {
	configPath := writeTempPipeline(t, `
pipeline:
  name: "Test Pipeline"

stages:
  - build
  - test

compile:
  - stage: build
  - image: golang:1.21
  - script:
    - "make build"
`)

	_, err := captureStdout(t, func() error {
		return runDryRun(&cobra.Command{}, []string{configPath})
	})
	if err == nil {
		t.Fatal("Expected error for stage with no jobs, got nil")
	}
	if !strings.Contains(err.Error(), "has no jobs assigned") {
		t.Fatalf("Expected empty stage error, got: %v", err)
	}
}

func TestDryRunCmd_PrintsValidationMessage(t *testing.T) {
	configPath, cleanup := writeTempPipelineInGitRepo(t, `
pipeline:
  name: "Test Pipeline"

stages:
  - build
  - test

compile:
  - stage: build
  - image: golang:1.21
  - script:
    - "make build"

unit-tests:
  - stage: test
  - image: golang:1.21
  - script:
    - "make test"
`)
	defer cleanup()

	output, err := runDryRunCommand(t, configPath)
	if err != nil {
		t.Fatalf("dryrun command returned error: %v", err)
	}
	if !strings.Contains(output, "Configuration is valid") {
		t.Fatalf("Expected validation output, got:\n%s", output)
	}
	if !strings.Contains(output, "build:") {
		t.Fatalf("Expected dryrun output, got:\n%s", output)
	}
}

func TestDryRunCmd_FailsOnInvalidConfig(t *testing.T) {
	configPath, cleanup := writeTempPipelineInGitRepo(t, `
pipeline:
  name: "Test Pipeline"

stages:
  - build
  - test

compile:
  - stage: build
  - image: golang:1.21
  - script:
    - "make build"
`)
	defer cleanup()

	output, err := runDryRunCommand(t, configPath)
	if err == nil {
		t.Fatal("Expected error for invalid config, got nil")
	}
	if strings.Contains(output, "build:") {
		t.Fatalf("Did not expect dryrun output, got:\n%s", output)
	}
}

func runDryRunCommand(t *testing.T, configPath string) (string, error) {
	t.Helper()
	cmd := *rootCmd
	cmd.SetArgs([]string{"dryrun", configPath})
	return captureStdout(t, func() error {
		return cmd.Execute()
	})
}

func writeTempPipeline(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "pipeline.yaml")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
		t.Fatalf("Failed to write pipeline file: %v", err)
	}
	return path
}
