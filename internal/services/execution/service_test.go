package execution

import (
	"reflect"
	"testing"

	"github.com/CS7580-SEA-SP26/e-team/internal/models"
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

	if jobConfigs[jobKey{stage: "build", name: "compile"}].failures {
		t.Fatalf("expected compile to require success")
	}

	lintConfig := jobConfigs[jobKey{stage: "build", name: "lint"}]
	if !lintConfig.failures {
		t.Fatalf("expected lint to allow failure")
	}
	if !reflect.DeepEqual(lintConfig.needs, []string{"compile"}) {
		t.Fatalf("expected lint needs [compile], got %v", lintConfig.needs)
	}

	if jobConfigs[jobKey{stage: "release", name: "deploy"}].failures {
		t.Fatalf("expected deploy to require success")
	}
}

func TestBuildJobConfigsNilPipeline(t *testing.T) {
	jobConfigs := buildJobConfigs(nil)
	if len(jobConfigs) != 0 {
		t.Fatalf("expected empty map for nil pipeline, got %d entries", len(jobConfigs))
	}
}

func TestBlockedByFailedDependency(t *testing.T) {
	allowedFailedJobs := map[jobKey]bool{
		{stage: "build", name: "lint"}: true,
	}

	failedNeed, blocked := blockedByFailedDependency("build", []string{"compile", "lint"}, allowedFailedJobs)
	if !blocked {
		t.Fatal("expected dependency check to block job")
	}
	if failedNeed != "lint" {
		t.Fatalf("expected lint to block job, got %q", failedNeed)
	}
}

func TestBlockedByFailedDependencyDifferentStage(t *testing.T) {
	allowedFailedJobs := map[jobKey]bool{
		{stage: "build", name: "lint"}: true,
	}

	if failedNeed, blocked := blockedByFailedDependency("test", []string{"lint"}, allowedFailedJobs); blocked {
		t.Fatalf("expected no block across different stages, got dependency %q", failedNeed)
	}
}
