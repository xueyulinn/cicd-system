package execution

import (
	"reflect"
	"testing"

	"github.com/xueyulinn/cicd-system/internal/api"
	"github.com/xueyulinn/cicd-system/internal/models"
)

func TestBuildJobConfigs(t *testing.T) {
	pipeline := &models.Pipeline{
		Jobs: []models.Job{
			{Name: "compile", Stage: "build", Failures: false},
			{Name: "lint", Stage: "build", Failures: true, Needs: []string{"compile"}},
			{Name: "deploy", Stage: "release", Failures: false},
		},
	}

	jobConfigs := buildJobConfigs(pipeline)

	if jobConfigs[jobKey{stage: "build", name: "compile"}].allowFailures {
		t.Fatalf("expected compile to require success")
	}

	lintConfig := jobConfigs[jobKey{stage: "build", name: "lint"}]
	if !lintConfig.allowFailures {
		t.Fatalf("expected lint to allow failure")
	}
	if !reflect.DeepEqual(lintConfig.needs, []string{"compile"}) {
		t.Fatalf("expected lint needs [compile], got %v", lintConfig.needs)
	}

	if jobConfigs[jobKey{stage: "release", name: "deploy"}].allowFailures {
		t.Fatalf("expected deploy to require success")
	}
}

func TestBuildJobConfigsNilPipeline(t *testing.T) {
	jobConfigs := buildJobConfigs(nil)
	if len(jobConfigs) != 0 {
		t.Fatalf("expected empty map for nil pipeline, got %d entries", len(jobConfigs))
	}
}

func TestBuildJobConfigsCopiesNeeds(t *testing.T) {
	pipeline := &models.Pipeline{
		Jobs: []models.Job{
			{Name: "compile", Stage: "build"},
			{Name: "integration-tests", Stage: "build", Needs: []string{"compile"}},
		},
	}

	jobConfigs := buildJobConfigs(pipeline)
	integrationConfig := jobConfigs[jobKey{stage: "build", name: "integration-tests"}]
	if !reflect.DeepEqual(integrationConfig.needs, []string{"compile"}) {
		t.Fatalf("expected integration-tests needs [compile], got %v", integrationConfig.needs)
	}
}

func TestBuildRunRequestKeyStableForIdenticalRequest(t *testing.T) {
	req := api.RunRequest{
		YAMLContent:   "name: demo\njobs: []\n",
		Branch:        "main",
		Commit:        "abc123",
		RepoURL:       "https://example.com/repo.git",
		WorkspacePath: "/tmp/worktree",
	}

	first := buildRunRequestKey(req, "demo")
	second := buildRunRequestKey(req, "demo")

	if first == "" {
		t.Fatal("expected non-empty request key")
	}
	if first != second {
		t.Fatalf("expected deterministic request key, got %q and %q", first, second)
	}
}

func TestBuildRunRequestKeyChangesWhenRequestChanges(t *testing.T) {
	base := api.RunRequest{
		YAMLContent:   "name: demo\njobs: []\n",
		Branch:        "main",
		Commit:        "abc123",
		RepoURL:       "https://example.com/repo.git",
		WorkspacePath: "/tmp/worktree",
	}
	modified := base
	modified.Commit = "def456"

	if buildRunRequestKey(base, "demo") == buildRunRequestKey(modified, "demo") {
		t.Fatal("expected request key to change when run identity changes")
	}
}

func TestBuildRunRequestKeyIgnoresWorkspacePath(t *testing.T) {
	base := api.RunRequest{
		YAMLContent:   "name: demo\njobs: []\n",
		Branch:        "main",
		Commit:        "abc123",
		RepoURL:       "https://example.com/repo.git",
		WorkspacePath: "/tmp/worktree-a",
	}
	modified := base
	modified.WorkspacePath = "/tmp/worktree-b"

	if buildRunRequestKey(base, "demo") != buildRunRequestKey(modified, "demo") {
		t.Fatal("expected request key to ignore workspace path differences")
	}
}
