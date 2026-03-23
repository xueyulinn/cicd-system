package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindPipelineByName_FindsMatch(t *testing.T) {
	repoDir, cleanup := initTempGitRepo(t)
	defer cleanup()

	pipelinesDir := filepath.Join(repoDir, ".pipelines")
	if err := os.MkdirAll(pipelinesDir, 0o755); err != nil {
		t.Fatalf("mkdir .pipelines: %v", err)
	}

	content := `
pipeline:
  name: "target-pipeline"

stages:
  - build

compile:
  - stage: build
  - image: golang:1.21
  - script:
    - "go build ./..."
`
	if err := os.WriteFile(filepath.Join(pipelinesDir, "target.yaml"), []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
		t.Fatalf("write pipeline file: %v", err)
	}

	got, err := findPipelineByName("target-pipeline")
	if err != nil {
		t.Fatalf("findPipelineByName returned error: %v", err)
	}
	if got != filepath.Join(".pipelines", "target.yaml") {
		t.Fatalf("unexpected file path: got %q", got)
	}
}

func TestFindPipelineByName_NotFound(t *testing.T) {
	repoDir, cleanup := initTempGitRepo(t)
	defer cleanup()

	pipelinesDir := filepath.Join(repoDir, ".pipelines")
	if err := os.MkdirAll(pipelinesDir, 0o755); err != nil {
		t.Fatalf("mkdir .pipelines: %v", err)
	}

	content := `
pipeline:
  name: "another-pipeline"

stages:
  - build

compile:
  - stage: build
  - image: golang:1.21
  - script:
    - "go build ./..."
`
	if err := os.WriteFile(filepath.Join(pipelinesDir, "another.yaml"), []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
		t.Fatalf("write pipeline file: %v", err)
	}

	_, err := findPipelineByName("missing")
	if err == nil {
		t.Fatal("expected not found error, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestGetCurrentCommit_ReturnsHeadCommit(t *testing.T) {
	repoDir, cleanup := initTempGitRepo(t)
	defer cleanup()

	writeAndCommitFile(t, repoDir, "README.md", "first")
	writeAndCommitFile(t, repoDir, "README.md", "second")

	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD failed: %v", err)
	}
	expected := strings.TrimSpace(string(out))

	got, err := getCurrentCommit()
	if err != nil {
		t.Fatalf("getCurrentCommit returned error: %v", err)
	}
	if got != expected {
		t.Fatalf("unexpected commit hash: got %q want %q", got, expected)
	}
}

func TestRunPreRunE_DefaultsBranchAndCommit(t *testing.T) {
	repoDir, cleanup := initTempGitRepo(t)
	defer cleanup()
	writeAndCommitFile(t, repoDir, "README.md", "first")

	pipelinePath := writePipelineFile(t, repoDir)
	resetRunGlobals()
	runFile = pipelinePath

	err := runPreRunE(nil, nil)
	if err != nil {
		t.Fatalf("runPreRunE returned error: %v", err)
	}

	currentCommit, err := getCurrentCommit()
	if err != nil {
		t.Fatalf("getCurrentCommit failed: %v", err)
	}

	if runBranch != "main" {
		t.Fatalf("expected default branch main, got %q", runBranch)
	}
	if runCommit != currentCommit {
		t.Fatalf("expected default commit %q, got %q", currentCommit, runCommit)
	}
}

func TestRunPreRunE_FailsWhenBranchMismatch(t *testing.T) {
	repoDir, cleanup := initTempGitRepo(t)
	defer cleanup()
	writeAndCommitFile(t, repoDir, "README.md", "first")

	pipelinePath := writePipelineFile(t, repoDir)
	resetRunGlobals()
	runFile = pipelinePath
	runBranch = "feature/not-current"

	err := runPreRunE(nil, nil)
	if err == nil {
		t.Fatal("expected branch mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "does not match current checked out branch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPreRunE_FailsWhenCommitMismatch(t *testing.T) {
	repoDir, cleanup := initTempGitRepo(t)
	defer cleanup()
	writeAndCommitFile(t, repoDir, "README.md", "first")

	pipelinePath := writePipelineFile(t, repoDir)
	resetRunGlobals()
	runFile = pipelinePath
	runBranch = "main"
	runCommit = "deadbeef"

	err := runPreRunE(nil, nil)
	if err == nil {
		t.Fatal("expected commit mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "does not match current checked out commit") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizeRepoURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "https", in: "https://github.com/org/repo.git", want: "https://github.com/org/repo.git"},
		{name: "git ssh", in: "git@github.com:org/repo.git", want: "https://github.com/org/repo.git"},
		{name: "ssh url", in: "ssh://git@github.com/org/repo.git", want: "https://github.com/org/repo.git"},
		{name: "empty", in: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeRepoURL(tt.in); got != tt.want {
				t.Fatalf("normalizeRepoURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func initTempGitRepo(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	runGit(t, tmpDir, "init", "-b", "main")
	runGit(t, tmpDir, "config", "user.email", "test@example.com")
	runGit(t, tmpDir, "config", "user.name", "test-user")

	return tmpDir, func() {
		resetRunGlobals()
		_ = os.Chdir(origWd)
	}
}

func writeAndCommitFile(t *testing.T, repoDir, name, content string) {
	t.Helper()
	path := filepath.Join(repoDir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	runGit(t, repoDir, "add", name)
	runGit(t, repoDir, "commit", "-m", "test commit")
}

func writePipelineFile(t *testing.T, repoDir string) string {
	t.Helper()
	pipelinePath := filepath.Join(repoDir, "pipeline.yaml")
	content := `
pipeline:
  name: "test-pipeline"

stages:
  - build

compile:
  - stage: build
  - image: golang:1.21
  - script:
    - "go build ./..."
`
	if err := os.WriteFile(pipelinePath, []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
		t.Fatalf("write pipeline file: %v", err)
	}
	return pipelinePath
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
}

func resetRunGlobals() {
	runFile = ""
	runName = ""
	runBranch = ""
	runCommit = ""
}

