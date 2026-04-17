package gitutil

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/go-git/go-git/v6/plumbing/storer"
	"github.com/go-git/go-git/v6/plumbing/transport"
	githttp "github.com/go-git/go-git/v6/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v6/plumbing/transport/ssh"
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

// Root returns the repository worktree root.
func (r *Repository) Root() string {
	return r.root
}

// GetCurrentBranch returns the branch name referenced by HEAD in the current
// Git worktree. It discovers the repository from the current directory, so it
// works when called from a repository subdirectory. If HEAD points directly to
// a commit instead of a branch ref, it returns a detached HEAD error because
// there is no current branch name to report.
func (r *Repository) GetHeadBranch() (string, error) {
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
func (r *Repository) GetHeadCommit() (string, error) {
	ref, err := r.repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD: %w", err)
	}

	return ref.Hash().String(), nil
}

// getLatestCommitByBranch returns the tip commit hash of a local branch from
// refs/heads/<branch> in the current repository.
func (r *Repository) GetHeadCommitByBranch(branch string) (string, error) {
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
func (r *Repository) CreateDetachedWorktree(commit string) (string, func(), error) {
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

func (r *Repository) BranchContainsCommit(branch string, commitSHA string) (bool, error) {
	ref, err := r.repo.Reference(plumbing.NewBranchReferenceName(branch), true)
	if err != nil {
		return false, err
	}

	target := plumbing.NewHash(commitSHA)
	if ref.Hash() == target {
		return true, nil
	}

	iter, err := r.repo.Log(&git.LogOptions{From: ref.Hash()})
	if err != nil {
		return false, err
	}
	defer iter.Close()

	found := false
	err = iter.ForEach(func(c *object.Commit) error {
		if c.Hash == target {
			found = true
			return storer.ErrStop
		}
		return nil
	})
	if err != nil && err != storer.ErrStop {
		return false, err
	}
	return found, nil
}

func (r *Repository) buildRemoteAuth(remoteURL string) (transport.AuthMethod, error) {
	ep, err := transport.NewEndpoint(strings.TrimSpace(remoteURL))
	if err != nil {
		return nil, fmt.Errorf("parse remote URL: %w", err)
	}

	switch ep.Scheme {
	case "http", "https":
		token := strings.TrimSpace(os.Getenv("CICD_GIT_TOKEN"))
		if token == "" {
			return nil, nil
		}
		user := strings.TrimSpace(os.Getenv("CICD_GIT_USERNAME"))
		if user == "" {
			user = "x-access-token"
		}
		return &githttp.BasicAuth{
			Username: user,
			Password: token,
		}, nil
	case "ssh":
		keyPath := strings.TrimSpace(os.Getenv("CICD_SSH_KEY_PATH"))
		if keyPath == "" {
			return nil, nil
		}
		sshUser := strings.TrimSpace(os.Getenv("CICD_SSH_USER"))
		if sshUser == "" {
			sshUser = "git"
		}
		passphrase := os.Getenv("CICD_SSH_KEY_PASSPHRASE")
		auth, err := gitssh.NewPublicKeysFromFile(sshUser, keyPath, passphrase)
		if err != nil {
			return nil, fmt.Errorf("load ssh auth from %q: %w", keyPath, err)
		}
		return auth, nil
	default:
		return nil, nil
	}
}

func firstNonEmptyURL(urls []string) string {
	for _, raw := range urls {
		url := strings.TrimSpace(raw)
		if url != "" {
			return url
		}
	}
	return ""
}

func (r *Repository) GetRepoURL(remoteName string) (string, error) {
	remoteName = strings.TrimSpace(remoteName)
	if remoteName == "" {
		return "", fmt.Errorf("remote name is required")
	}

	remote, err := r.repo.Remote(remoteName)
	if err != nil {
		return "", fmt.Errorf("remote %q not found: %w", remoteName, err)
	}

	cfg := remote.Config()

	if cfg == nil || len(cfg.URLs) == 0 {
		return "", fmt.Errorf("remote %q has no configured URL", remoteName)
	}

	if url := firstNonEmptyURL(cfg.URLs); url != "" {
		return url, nil
	}

	return "", fmt.Errorf("remote %q has no configured URL", remoteName)
}

func (r *Repository) RemoteBranchContainsCommit(
	remoteName, branch, commitSHA string,
	auth transport.AuthMethod,
) (bool, error) {
	remoteName = strings.TrimSpace(remoteName)
	branch = strings.TrimSpace(branch)
	commitSHA = strings.TrimSpace(commitSHA)
	if remoteName == "" {
		return false, fmt.Errorf("remote name is required")
	}
	if branch == "" {
		return false, fmt.Errorf("branch is required")
	}
	if commitSHA == "" {
		return false, fmt.Errorf("commit is required")
	}

	remoteURL, err := r.GetRepoURL(remoteName)
	if err != nil {
		return false, err
	}

	refspec := config.RefSpec(fmt.Sprintf(
		"+refs/heads/%s:refs/remotes/%s/%s",
		branch, remoteName, branch,
	))

	fetch := func(fetchAuth transport.AuthMethod) error {
		fetchErr := r.repo.Fetch(&git.FetchOptions{
			RemoteName: remoteName,
			RefSpecs:   []config.RefSpec{refspec},
			Auth:       fetchAuth,
			Tags:       git.NoTags,
			Force:      true,
		})
		if fetchErr != nil && !errors.Is(fetchErr, git.NoErrAlreadyUpToDate) {
			return fetchErr
		}
		return nil
	}

	err = fetch(nil)
	if err != nil {
		if errors.Is(err, transport.ErrAuthenticationRequired) || errors.Is(err, transport.ErrAuthorizationFailed) {
			if auth == nil {
				resolvedAuth, resolveErr := r.buildRemoteAuth(remoteURL)
				if resolveErr != nil {
					return false, resolveErr
				}
				auth = resolvedAuth
			}
			if auth == nil {
				return false, fmt.Errorf(
					"remote %q requires authentication; set CICD_GIT_TOKEN (HTTPS) or CICD_SSH_KEY_PATH (SSH)",
					remoteName,
				)
			}
			if err = fetch(auth); err != nil {
				return false, fmt.Errorf("fetch remote branch with auth: %w", err)
			}
		} else {
			return false, fmt.Errorf("fetch remote branch: %w", err)
		}
	}

	remoteRefName := plumbing.NewRemoteReferenceName(remoteName, branch)
	ref, err := r.repo.Reference(remoteRefName, true)
	if err != nil {
		return false, err
	}

	target := plumbing.NewHash(commitSHA)
	if ref.Hash() == target {
		return true, nil
	}

	iter, err := r.repo.Log(&git.LogOptions{From: ref.Hash()})
	if err != nil {
		return false, err
	}
	defer iter.Close()

	found := false
	err = iter.ForEach(func(c *object.Commit) error {
		if c.Hash == target {
			found = true
			return storer.ErrStop
		}
		return nil
	})
	if err != nil && !errors.Is(err, storer.ErrStop) {
		return false, err
	}
	return found, nil
}
