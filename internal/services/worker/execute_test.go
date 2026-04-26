package worker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCloneAuthUsesGitHubTokenForGitHubRepos(t *testing.T) {
	t.Setenv("GIT_USERNAME", "")
	t.Setenv("GIT_PASSWORD", "")
	t.Setenv("GITHUB_TOKEN", "token-123")

	auth := cloneAuth("https://github.com/org/private-repo.git")
	if auth == nil {
		t.Fatal("expected auth for GitHub token")
		return
	}
	if auth.Username != "x-access-token" {
		t.Fatalf("auth.Username = %q, want x-access-token", auth.Username)
	}
	if auth.Password != "token-123" {
		t.Fatalf("auth.Password = %q, want token-123", auth.Password)
	}
}

func TestCloneAuthPrefersExplicitGitCredentials(t *testing.T) {
	t.Setenv("GIT_USERNAME", "git-user")
	t.Setenv("GIT_PASSWORD", "git-pass")
	t.Setenv("GITHUB_TOKEN", "token-123")

	auth := cloneAuth("https://github.com/org/private-repo.git")
	if auth == nil {
		t.Fatal("expected auth for explicit git credentials")
		return
	}
	if auth.Username != "git-user" {
		t.Fatalf("auth.Username = %q, want git-user", auth.Username)
	}
	if auth.Password != "git-pass" {
		t.Fatalf("auth.Password = %q, want git-pass", auth.Password)
	}
}

func TestCloneAuthReturnsNilWithoutCredentials(t *testing.T) {
	t.Setenv("GIT_USERNAME", "")
	t.Setenv("GIT_PASSWORD", "")
	t.Setenv("GITHUB_TOKEN", "")

	if auth := cloneAuth("https://github.com/org/private-repo.git"); auth != nil {
		t.Fatal("expected nil auth without credentials")
	}
}

func TestResolveWorkspacePathKeepsExistingPath(t *testing.T) {
	dir := t.TempDir()

	got, err := resolveWorkspacePath(dir)
	if err != nil {
		t.Fatalf("resolveWorkspacePath returned error: %v", err)
	}
	if got != dir {
		t.Fatalf("resolved path = %q, want %q", got, dir)
	}
}

func TestResolveWorkspacePathMapsWindowsPathToHostTemp(t *testing.T) {
	hostTemp := t.TempDir()
	t.Setenv("WORKSPACE_HOST_TEMP_DIR", hostTemp)

	const wtName = "cicd-run-wt-2605657174"
	mappedPath := filepath.Join(hostTemp, wtName)
	if err := os.MkdirAll(mappedPath, 0o755); err != nil {
		t.Fatalf("mkdir mapped path failed: %v", err)
	}

	got, err := resolveWorkspacePath(`Z:\no-such-host-temp\` + wtName)
	if err != nil {
		t.Fatalf("resolveWorkspacePath returned error: %v", err)
	}
	if got != mappedPath {
		t.Fatalf("resolved path = %q, want %q", got, mappedPath)
	}
}

func TestContainerResourcesFromEnvUsesCPULimit(t *testing.T) {
	t.Setenv("WORKER_JOB_CPU_LIMIT", "0.5")
	t.Setenv("WORKER_JOB_NANO_CPUS", "")
	t.Setenv("WORKER_JOB_MEMORY_LIMIT_MB", "")

	resources, err := containerResourcesFromEnv()
	if err != nil {
		t.Fatalf("containerResourcesFromEnv returned error: %v", err)
	}
	if resources.NanoCPUs != 500000000 {
		t.Fatalf("resources.NanoCPUs = %d, want 500000000", resources.NanoCPUs)
	}
	if resources.Memory != 0 {
		t.Fatalf("resources.Memory = %d, want 0", resources.Memory)
	}
}

func TestContainerResourcesFromEnvUsesMemoryLimitMB(t *testing.T) {
	t.Setenv("WORKER_JOB_CPU_LIMIT", "")
	t.Setenv("WORKER_JOB_NANO_CPUS", "")
	t.Setenv("WORKER_JOB_MEMORY_LIMIT_MB", "256")

	resources, err := containerResourcesFromEnv()
	if err != nil {
		t.Fatalf("containerResourcesFromEnv returned error: %v", err)
	}
	if resources.Memory != 256*1024*1024 {
		t.Fatalf("resources.Memory = %d, want %d", resources.Memory, 256*1024*1024)
	}
}

func TestContainerResourcesFromEnvRejectsConflictingCPUVars(t *testing.T) {
	t.Setenv("WORKER_JOB_CPU_LIMIT", "1")
	t.Setenv("WORKER_JOB_NANO_CPUS", "1000000000")
	t.Setenv("WORKER_JOB_MEMORY_LIMIT_MB", "")

	if _, err := containerResourcesFromEnv(); err == nil {
		t.Fatal("expected error when both cpu env vars are set")
	}
}

func TestContainerResourcesFromEnvRejectsInvalidCPU(t *testing.T) {
	t.Setenv("WORKER_JOB_CPU_LIMIT", "bad")
	t.Setenv("WORKER_JOB_NANO_CPUS", "")
	t.Setenv("WORKER_JOB_MEMORY_LIMIT_MB", "")

	if _, err := containerResourcesFromEnv(); err == nil {
		t.Fatal("expected error for invalid WORKER_JOB_CPU_LIMIT")
	}
}
