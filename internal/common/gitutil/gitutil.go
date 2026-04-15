package gitutil

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
)

 type Repository struct {
        repo *git.Repository
        root string
  }

func Open(dir string) (*Repository, error) {
        repo, err := git.PlainOpenWithOptions(dir, &git.PlainOpenOptions{
                DetectDotGit: true,
        })
        if err != nil {
                return nil, fmt.Errorf("directory %q is not inside a git repository: %w", dir, err)
        }

        wt, err := repo.Worktree()
        if err != nil {
                return nil, fmt.Errorf("failed to get git worktree: %w", err)
        }

        return &Repository{
                repo: repo,
                root: wt.Filesystem.Root(),
        }, nil
  }

// GetCurrentBranch returns the branch name referenced by HEAD in the current
// Git worktree. It discovers the repository from the current directory, so it
// works when called from a repository subdirectory. If HEAD points directly to
// a commit instead of a branch ref, it returns a detached HEAD error because
// there is no current branch name to report.
func(r *Repository) GetHeadBranch() (string, error) {
	ref, err := r.repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD: %w", err)
	}

	if ref.Name().IsBranch() {
		return ref.Name().Short(), nil
	}

	return "", fmt.Errorf("detached HEAD at %s", ref.Hash().String())
}

// GetCurrentCommit returns the commit hash currently checked out at HEAD.
func(r *Repository) GetHeadCommit() (string, error) {
	ref, err := r.repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD: %w", err)
	}

	return ref.Hash().String(), nil
}

// getLatestCommitByBranch returns the tip commit hash of a local branch from
// refs/heads/<branch> in the current repository.
func(r *Repository) GetHeadCommitByBranch(branch string) (string, error) {
	// refs/heads/branch
	localRefName := plumbing.NewBranchReferenceName(branch)
	ref, err := r.repo.Reference(localRefName, true)
	if err != nil {
		return "", fmt.Errorf("branch: %q not found: %w", branch, err)
	}

	return ref.Hash().String(), nil
}

// createDetachedWorktree creates a temporary detached git worktree at commit
// and returns the directory path with a cleanup callback.
func(r *Repository) CreateDetachedWorktree(commit string) (string, func(), error) {
	tmpDir, err := os.MkdirTemp("", "cicd-run-wt-*")
	if err != nil {
		return "", nil, fmt.Errorf("create temp dir failed: %w", err)
	}

	cmd := exec.Command("git", "-C", r.root, "worktree", "add", "--detach", tmpDir, commit)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", nil, fmt.Errorf("git worktree add failed: %v, output: %s", err, string(out))
	}

	cleanup := func() {
		_ = exec.Command("git", "-C", r.root, "worktree", "remove", "--force", tmpDir).Run()
		_ = exec.Command("git", "-C", r.root, "worktree", "prune").Run()
		_ = os.RemoveAll(tmpDir)
	}
	return tmpDir, cleanup, nil
}