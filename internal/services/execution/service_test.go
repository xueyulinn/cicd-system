package execution

import (
	"testing"

	"github.com/CS7580-SEA-SP26/e-team/internal/models"
)

func TestBuildFailuresByJob(t *testing.T) {
	pipeline := &models.Pipeline{
		Jobs: []models.Job{
			{Name: "compile", Stage: "build", Failures: false},
			{Name: "lint", Stage: "build", Failures: true},
			{Name: "deploy", Stage: "release", Failures: false},
		},
	}

	failuresByJob := buildFailuresByJob(pipeline)

	if failuresByJob[jobKey{stage: "build", name: "compile"}] {
		t.Fatalf("expected compile to require success")
	}
	if !failuresByJob[jobKey{stage: "build", name: "lint"}] {
		t.Fatalf("expected lint to allow failure")
	}
	if failuresByJob[jobKey{stage: "release", name: "deploy"}] {
		t.Fatalf("expected deploy to require success")
	}
}

func TestBuildFailuresByJobNilPipeline(t *testing.T) {
	failuresByJob := buildFailuresByJob(nil)
	if len(failuresByJob) != 0 {
		t.Fatalf("expected empty map for nil pipeline, got %d entries", len(failuresByJob))
	}
}
