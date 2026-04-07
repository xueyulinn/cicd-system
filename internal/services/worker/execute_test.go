package worker

import "testing"

func TestCloneAuthUsesGitHubTokenForGitHubRepos(t *testing.T) {
	t.Setenv("GIT_USERNAME", "")
	t.Setenv("GIT_PASSWORD", "")
	t.Setenv("GITHUB_TOKEN", "token-123")

	auth := cloneAuth("https://github.com/org/private-repo.git")
	if auth == nil {
		t.Fatal("expected auth for GitHub token")
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
