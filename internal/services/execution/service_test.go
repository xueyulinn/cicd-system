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
