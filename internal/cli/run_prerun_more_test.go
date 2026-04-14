package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunPreRunE_InputValidationBranches(t *testing.T) {
	repoDir, cleanup := initTempGitRepo(t)
	defer cleanup()
	writeAndCommitFile(t, repoDir, "README.md", "x")

	resetRunGlobals()
	if err := runPreRunE(nil, nil); err == nil || !strings.Contains(err.Error(), "at least one") {
		t.Fatalf("err=%v", err)
	}

	resetRunGlobals()
	runFile = "a.yaml"
	runName = "pipeline"
	if err := runPreRunE(nil, nil); err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("err=%v", err)
	}

	resetRunGlobals()
	runFile = "missing.yaml"
	if err := runPreRunE(nil, nil); err == nil || !strings.Contains(err.Error(), "invalid --file") {
		t.Fatalf("err=%v", err)
	}
}

func TestRunPreRunE_WithNameResolvesPipeline(t *testing.T) {
	repoDir, cleanup := initTempGitRepo(t)
	defer cleanup()
	writeAndCommitFile(t, repoDir, "README.md", "x")

	pipelinesDir := filepath.Join(repoDir, ".pipelines")
	if err := os.MkdirAll(pipelinesDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := `pipeline:
  name: "my-pipeline"
stages:
  - build
compile:
  - stage: build
  - image: alpine
  - script:
    - echo ok
`
	if err := os.WriteFile(filepath.Join(pipelinesDir, "p.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	resetRunGlobals()
	runName = "my-pipeline"
	if err := runPreRunE(nil, nil); err != nil {
		t.Fatalf("runPreRunE err=%v", err)
	}
	if !strings.HasSuffix(filepath.ToSlash(runFile), ".pipelines/p.yaml") {
		t.Fatalf("runFile=%q", runFile)
	}
}

func TestGetCurrentBranch_DetachedHead(t *testing.T) {
	repoDir, cleanup := initTempGitRepo(t)
	defer cleanup()
	writeAndCommitFile(t, repoDir, "README.md", "x")
	commit, err := getCurrentCommit()
	if err != nil {
		t.Fatalf("getCurrentCommit: %v", err)
	}
	runGit(t, repoDir, "checkout", commit)

	_, err = getCurrentBranch()
	if err == nil || !strings.Contains(err.Error(), "detached HEAD") {
		t.Fatalf("err=%v", err)
	}
}
