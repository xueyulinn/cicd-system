package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetWorkspacePath_SuccessAndFailure(t *testing.T) {
	repoDir, cleanup := initTempGitRepo(t)
	defer cleanup()
	writeAndCommitFile(t, repoDir, "README.md", "x")

	path, err := getWorkspacePath()
	if err != nil {
		t.Fatalf("getWorkspacePath err=%v", err)
	}
	if filepath.Clean(path) != filepath.Clean(repoDir) {
		t.Fatalf("path=%q want=%q", path, repoDir)
	}

	orig, _ := os.Getwd()
	nonRepo := t.TempDir()
	if err := os.Chdir(nonRepo); err != nil {
		t.Fatalf("chdir nonrepo: %v", err)
	}
	defer func() { _ = os.Chdir(orig) }()

	if _, err := getWorkspacePath(); err == nil {
		t.Fatal("expected error outside git repo")
	}
}

func TestGetRepoURL_Variants(t *testing.T) {
	t.Run("no remote", func(t *testing.T) {
		repoDir, cleanup := initTempGitRepo(t)
		defer cleanup()
		writeAndCommitFile(t, repoDir, "README.md", "x")
		if got := getRepoURL(); got != "" {
			t.Fatalf("got=%q want empty", got)
		}
	})

	t.Run("origin ssh", func(t *testing.T) {
		repoDir, cleanup := initTempGitRepo(t)
		defer cleanup()
		writeAndCommitFile(t, repoDir, "README.md", "x")
		runGit(t, repoDir, "remote", "add", "origin", "git@github.com:org/repo.git")
		if got := getRepoURL(); got != "https://github.com/org/repo.git" {
			t.Fatalf("got=%q", got)
		}
	})

	t.Run("fallback first remote", func(t *testing.T) {
		repoDir, cleanup := initTempGitRepo(t)
		defer cleanup()
		writeAndCommitFile(t, repoDir, "README.md", "x")
		runGit(t, repoDir, "remote", "add", "upstream", "ssh://git@github.com/org/up.git")
		if got := getRepoURL(); got != "https://github.com/org/up.git" {
			t.Fatalf("got=%q", got)
		}
	})
}

func TestCreateDetachedWorktree_ResolveRunFile_AndHead(t *testing.T) {
	repoDir, cleanupRepo := initTempGitRepo(t)
	defer cleanupRepo()

	pipelinePath := writePipelineFile(t, repoDir)
	runGit(t, repoDir, "add", filepath.Base(pipelinePath))
	runGit(t, repoDir, "commit", "-m", "add pipeline")

	commit, err := getCurrentCommit()
	if err != nil {
		t.Fatalf("getCurrentCommit err=%v", err)
	}

	wt, cleanup, err := createDetachedWorktree(repoDir, commit)
	if err != nil {
		t.Fatalf("createDetachedWorktree err=%v", err)
	}
	defer cleanup()

	head, err := getHEADCommitAtPath(wt)
	if err != nil {
		t.Fatalf("getHEADCommitAtPath err=%v", err)
	}
	if head != commit {
		t.Fatalf("head=%q want=%q", head, commit)
	}

	resolved, err := resolveRunFileInWorkspace(pipelinePath, repoDir, wt)
	if err != nil {
		t.Fatalf("resolveRunFileInWorkspace err=%v", err)
	}
	if !strings.HasSuffix(filepath.ToSlash(resolved), "/pipeline.yaml") {
		t.Fatalf("resolved=%q", resolved)
	}

	outside := filepath.Join(t.TempDir(), "x.yaml")
	if err := os.WriteFile(outside, []byte("x"), 0o644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	if _, err := resolveRunFileInWorkspace(outside, repoDir, wt); err == nil {
		t.Fatal("expected outside repo error")
	}
}

func TestCreateDetachedWorktree_InvalidCommitAndHeadFailure(t *testing.T) {
	repoDir, cleanup := initTempGitRepo(t)
	defer cleanup()
	writeAndCommitFile(t, repoDir, "README.md", "x")

	if _, _, err := createDetachedWorktree(repoDir, "deadbeef"); err == nil {
		t.Fatal("expected invalid commit error")
	}

	nonRepo := t.TempDir()
	if _, err := getHEADCommitAtPath(nonRepo); err == nil {
		t.Fatal("expected rev-parse failure")
	}
}

func TestRunRun_TestModeAndErrorPath(t *testing.T) {
	repoDir, cleanup := initTempGitRepo(t)
	defer cleanup()

	pipelinePath := writePipelineFile(t, repoDir)
	runGit(t, repoDir, "add", filepath.Base(pipelinePath))
	runGit(t, repoDir, "commit", "-m", "add pipeline")

	commit, err := getCurrentCommit()
	if err != nil {
		t.Fatalf("getCurrentCommit err=%v", err)
	}

	resetRunGlobals()
	runFile = pipelinePath
	runBranch = "main"
	runCommit = commit
	t.Setenv("CICD_TEST_MODE", "1")

	if err := runRun(nil, nil); err != nil {
		t.Fatalf("runRun test mode err=%v", err)
	}

	resetRunGlobals()
	runFile = pipelinePath
	runBranch = "main"
	runCommit = "deadbeef"
	if err := runRun(nil, nil); err == nil {
		t.Fatal("expected worktree error for invalid commit")
	}
}
