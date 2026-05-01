package gitutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing/object"
)

func TestReadFileAtCommit_ReturnsContentFromTargetCommit(t *testing.T) {
	repoDir, firstCommit, secondCommit := setupRepoWithFileHistory(t)

	repo, err := Open(repoDir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	pipelineDir := filepath.Join(repoDir, ".pipelines")
	if err := os.Chdir(pipelineDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origWD) }()

	firstContent, err := repo.ReadFileAtCommit(firstCommit, "build.yaml")
	if err != nil {
		t.Fatalf("ReadFileAtCommit(first): %v", err)
	}
	if got := string(firstContent); got != "version: one\n" {
		t.Fatalf("unexpected first commit content: %q", got)
	}

	secondContent, err := repo.ReadFileAtCommit(secondCommit, "build.yaml")
	if err != nil {
		t.Fatalf("ReadFileAtCommit(second): %v", err)
	}
	if got := string(secondContent); got != "version: two\n" {
		t.Fatalf("unexpected second commit content: %q", got)
	}
}

func TestReadFileAtCommit_RejectsPathOutsideRepository(t *testing.T) {
	repoDir, commit, _ := setupRepoWithFileHistory(t)

	repo, err := Open(repoDir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	outsideFile := filepath.Join(t.TempDir(), "outside.yaml")
	if err := os.WriteFile(outsideFile, []byte("outside\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err = repo.ReadFileAtCommit(commit, outsideFile)
	if err == nil {
		t.Fatal("expected error for file outside repository")
	}
	if !strings.Contains(err.Error(), "outside repository") {
		t.Fatalf("expected outside repository error, got: %v", err)
	}
}

func TestRemoteBranchContainsCommit_RemoteNotFound(t *testing.T) {
	cloneDir, _, _ := setupLocalRemoteClone(t)

	repo, err := Open(cloneDir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	_, err = repo.RemoteBranchContainsCommit("missing", "master", strings.Repeat("a", 40), nil)
	if err == nil || !strings.Contains(err.Error(), `remote "missing" not found`) {
		t.Fatalf("expected remote not found error, got: %v", err)
	}
}

func TestRemoteBranchContainsCommit_ContainsAndMissing(t *testing.T) {
	cloneDir, headCommit, remoteName := setupLocalRemoteClone(t)

	repo, err := Open(cloneDir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	contains, err := repo.RemoteBranchContainsCommit(remoteName, "master", headCommit, nil)
	if err != nil {
		t.Fatalf("RemoteBranchContainsCommit contains check failed: %v", err)
	}
	if !contains {
		t.Fatal("expected commit to be found on remote branch")
	}

	missingCommit := strings.Repeat("f", 40)
	contains, err = repo.RemoteBranchContainsCommit(remoteName, "master", missingCommit, nil)
	if err != nil {
		t.Fatalf("RemoteBranchContainsCommit missing check failed: %v", err)
	}
	if contains {
		t.Fatal("expected commit to be missing on remote branch")
	}
}

func setupLocalRemoteClone(t *testing.T) (cloneDir, headCommit, remoteName string) {
	t.Helper()

	base := t.TempDir()
	remoteBare := filepath.Join(base, "remote.git")
	src := filepath.Join(base, "src")
	clone := filepath.Join(base, "clone")
	remoteURL := localPathToFileURL(remoteBare)

	if _, err := git.PlainInit(remoteBare, true); err != nil {
		t.Fatalf("init bare remote: %v", err)
	}

	srcRepo, err := git.PlainInit(src, false)
	if err != nil {
		t.Fatalf("init src repo: %v", err)
	}
	if _, err := srcRepo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{remoteURL},
	}); err != nil {
		t.Fatalf("create remote: %v", err)
	}

	if err := os.WriteFile(filepath.Join(src, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	wt, err := srcRepo.Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}
	if _, err := wt.Add("README.md"); err != nil {
		t.Fatalf("add file: %v", err)
	}
	hash, err := wt.Commit("init", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "test-user",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("commit: %v", err)
	}

	if err := srcRepo.Push(&git.PushOptions{
		RemoteName: "origin",
		RefSpecs:   []config.RefSpec{"refs/heads/master:refs/heads/master"},
	}); err != nil {
		t.Fatalf("push: %v", err)
	}

	if _, err := git.PlainClone(clone, &git.CloneOptions{
		URL: remoteURL,
	}); err != nil {
		t.Fatalf("clone: %v", err)
	}

	return clone, hash.String(), "origin"
}

func localPathToFileURL(path string) string {
	slashed := filepath.ToSlash(path)
	if len(slashed) >= 2 && slashed[1] == ':' {
		return "file:///" + slashed
	}
	return "file://" + slashed
}

func setupRepoWithFileHistory(t *testing.T) (repoDir, firstCommit, secondCommit string) {
	t.Helper()

	repoDir = t.TempDir()
	repo, err := git.PlainInit(repoDir, false)
	if err != nil {
		t.Fatalf("PlainInit: %v", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}

	pipelineFile := filepath.Join(repoDir, ".pipelines", "build.yaml")
	if err := os.MkdirAll(filepath.Dir(pipelineFile), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	if err := os.WriteFile(pipelineFile, []byte("version: one\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(first): %v", err)
	}
	if _, err := worktree.Add(".pipelines/build.yaml"); err != nil {
		t.Fatalf("Add(first): %v", err)
	}
	firstHash, err := worktree.Commit("first", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "test-user",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("Commit(first): %v", err)
	}

	if err := os.WriteFile(pipelineFile, []byte("version: two\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(second): %v", err)
	}
	if _, err := worktree.Add(".pipelines/build.yaml"); err != nil {
		t.Fatalf("Add(second): %v", err)
	}
	secondHash, err := worktree.Commit("second", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "test-user",
			Email: "test@example.com",
			When:  time.Now().Add(time.Second),
		},
	})
	if err != nil {
		t.Fatalf("Commit(second): %v", err)
	}

	return repoDir, firstHash.String(), secondHash.String()
}
