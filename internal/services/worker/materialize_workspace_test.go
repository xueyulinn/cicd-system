package worker

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	git "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/object"
)

func createTempRepoWithCommit(t *testing.T) (repoPath string, commit string) {
	t.Helper()
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("init repo: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}
	if _, err := wt.Add("README.md"); err != nil {
		t.Fatalf("add file: %v", err)
	}
	hash, err := wt.Commit("init", &git.CommitOptions{Author: &object.Signature{Name: "t", Email: "t@example.com", When: time.Now()}})
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	return dir, hash.String()
}

func TestMaterializeWorkspace_CloneAndCheckoutSuccess(t *testing.T) {
	repoPath, commit := createTempRepoWithCommit(t)

	path, cleanup, err := materializeWorkspace(context.Background(), repoPath, commit, "")
	if err != nil {
		t.Fatalf("materializeWorkspace error: %v", err)
	}
	if path == "" {
		t.Fatal("expected non-empty workspace path")
	}
	if cleanup == nil {
		t.Fatal("expected non-nil cleanup")
	}
	if _, err := os.Stat(filepath.Join(path, "README.md")); err != nil {
		t.Fatalf("expected cloned file in workspace: %v", err)
	}

	cleanup()
	if _, err := os.Stat(path); err == nil {
		t.Fatal("expected workspace to be removed by cleanup")
	}
}

func TestMaterializeWorkspace_CheckoutFailureCleansTemp(t *testing.T) {
	repoPath, _ := createTempRepoWithCommit(t)
	invalidCommit := "ffffffffffffffffffffffffffffffffffffffff"

	path, cleanup, err := materializeWorkspace(context.Background(), repoPath, invalidCommit, "")
	if err == nil {
		t.Fatal("expected checkout error")
	}
	if cleanup != nil {
		t.Fatal("cleanup should be nil on checkout failure")
	}
	if path != "" {
		t.Fatalf("path=%q, want empty", path)
	}
}
