package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRunVerify_DirectoryScenarios(t *testing.T) {
	repoDir, cleanup := initTempGitRepo(t)
	defer cleanup()

	t.Run("no yaml in dir", func(t *testing.T) {
		dir := filepath.Join(repoDir, "configs")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}

		_, err := captureStdout(t, func() error { return runVerify(&cobra.Command{}, []string{dir}) })
		if err == nil || !strings.Contains(err.Error(), "no YAML files") {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("multiple yaml valid in test mode", func(t *testing.T) {
		dir := filepath.Join(repoDir, "multi")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		content := `pipeline:
  name: "p"
stages:
  - build
compile:
  - stage: build
  - image: alpine
  - script:
    - echo ok
`
		if err := os.WriteFile(filepath.Join(dir, "a.yaml"), []byte(content), 0o644); err != nil {
			t.Fatalf("write a: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "b.yml"), []byte(content), 0o644); err != nil {
			t.Fatalf("write b: %v", err)
		}
		t.Setenv("CICD_TEST_MODE", "1")

		out, err := captureStdout(t, func() error { return runVerify(&cobra.Command{}, []string{dir}) })
		if err != nil {
			t.Fatalf("runVerify err=%v", err)
		}
		if !strings.Contains(out, "All configurations are valid") {
			t.Fatalf("unexpected output: %s", out)
		}
	})
}

func TestRunVerify_GatewayValidationErrorPath(t *testing.T) {
	repoDir, err := os.MkdirTemp("", "cli-repo-*")
	if err != nil {
		t.Fatalf("mkdir temp repo: %v", err)
	}
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("chdir repo: %v", err)
	}
	runGit(t, repoDir, "init", "-b", "main")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "test-user")

	cfg := writePipelineFile(t, repoDir)
	runGit(t, repoDir, "add", filepath.Base(cfg))
	runGit(t, repoDir, "commit", "-m", "add pipeline")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid from gateway"}`))
	}))
	defer srv.Close()
	t.Setenv("GATEWAY_URL", srv.URL)
	t.Setenv("CICD_TEST_MODE", "0")

	_, err = captureStdout(t, func() error { return runVerify(&cobra.Command{}, []string{cfg}) })
	if err == nil || !strings.Contains(err.Error(), "validation failed") {
		t.Fatalf("err=%v", err)
	}
}
