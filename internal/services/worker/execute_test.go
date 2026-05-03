package worker

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/xueyulinn/cicd-system/internal/common/snapshot"
	"github.com/xueyulinn/cicd-system/internal/objectstorage"
)

type fakeWorkspaceStorage struct {
	downloadFn func(context.Context, string, string) error
}

func (f *fakeWorkspaceStorage) UploadWorkspace(context.Context, string, string) error {
	return nil
}

func (f *fakeWorkspaceStorage) DownloadWorkspace(ctx context.Context, objectName, filePath string) error {
	if f.downloadFn != nil {
		return f.downloadFn(ctx, objectName, filePath)
	}
	return nil
}

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

func TestMaterializeWorkspaceObject(t *testing.T) {
	sourceDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceDir, "root.txt"), []byte("root file"), 0o644); err != nil {
		t.Fatalf("WriteFile(root.txt) error = %v", err)
	}
	archivePath := filepath.Join(t.TempDir(), "workspace.tar.gz")
	if err := snapshot.Pack(sourceDir, archivePath); err != nil {
		t.Fatalf("Pack() error = %v", err)
	}

	originalFactory := newWorkspaceStorage
	defer func() { newWorkspaceStorage = originalFactory }()

	newWorkspaceStorage = func() (objectstorage.Storage, error) {
		return &fakeWorkspaceStorage{
			downloadFn: func(_ context.Context, objectName, filePath string) error {
				if objectName != "workspaces/pack-v1/commits/abc123/workspace.tar.gz" {
					t.Fatalf("objectName = %q", objectName)
				}
				data, err := os.ReadFile(archivePath)
				if err != nil {
					return err
				}
				return os.WriteFile(filePath, data, 0o644)
			},
		}, nil
	}

	workspaceDir, cleanup, err := materializeWorkspace(context.Background(), "", "", "workspaces/pack-v1/commits/abc123/workspace.tar.gz")
	if err != nil {
		t.Fatalf("materializeWorkspace returned error: %v", err)
	}
	defer cleanup()

	data, err := os.ReadFile(filepath.Join(workspaceDir, "root.txt"))
	if err != nil {
		t.Fatalf("ReadFile(root.txt) error = %v", err)
	}
	if string(data) != "root file" {
		t.Fatalf("root.txt = %q, want %q", string(data), "root file")
	}
}

func TestMaterializeWorkspaceObjectDownloadError(t *testing.T) {
	originalFactory := newWorkspaceStorage
	defer func() { newWorkspaceStorage = originalFactory }()

	newWorkspaceStorage = func() (objectstorage.Storage, error) {
		return &fakeWorkspaceStorage{
			downloadFn: func(context.Context, string, string) error {
				return errors.New("download failed")
			},
		}, nil
	}

	_, _, err := materializeWorkspace(context.Background(), "", "", "workspaces/pack-v1/commits/abc123/workspace.tar.gz")
	if err == nil || err.Error() != "download failed" {
		t.Fatalf("materializeWorkspace error = %v, want download failed", err)
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
