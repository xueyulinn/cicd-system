package dryrun

import (
	"reflect"
	"testing"

	"github.com/CS7580-SEA-SP26/e-team/internal/models"
)

func TestScheduleJobs_Empty(t *testing.T) {
	result := ScheduleJobs(nil)
	if result != nil {
		t.Errorf("Expected nil for nil input, got %v", result)
	}

	result = ScheduleJobs([]models.Job{})
	if result != nil {
		t.Errorf("Expected nil for empty input, got %v", result)
	}
}

func TestScheduleJobs_SingleJob(t *testing.T) {
	jobs := []models.Job{
		{Name: "build", Stage: "build", Image: "gradle:jdk21", Script: []string{"gradle build"}},
	}
	result := ScheduleJobs(jobs)
	if len(result) != 1 {
		t.Fatalf("Expected 1 job, got %d", len(result))
	}
	if result[0].Name != "build" {
		t.Errorf("Expected job 'build', got %q", result[0].Name)
	}
}

func TestScheduleJobs_NoDependencies(t *testing.T) {
	jobs := []models.Job{
		{Name: "job-a", Stage: "test", Needs: nil},
		{Name: "job-b", Stage: "test", Needs: nil},
		{Name: "job-c", Stage: "test", Needs: nil},
	}
	result := ScheduleJobs(jobs)
	if len(result) != 3 {
		t.Fatalf("Expected 3 jobs, got %d", len(result))
	}
	names := make(map[string]bool)
	for _, j := range result {
		names[j.Name] = true
	}
	for _, name := range []string{"job-a", "job-b", "job-c"} {
		if !names[name] {
			t.Errorf("Expected job %q in result", name)
		}
	}
}

func TestScheduleJobs_WithDependencies(t *testing.T) {
	jobs := []models.Job{
		{Name: "integration-tests", Stage: "test", Needs: []string{"unit-tests"}},
		{Name: "unit-tests", Stage: "test", Needs: nil},
	}
	result := ScheduleJobs(jobs)
	if len(result) != 2 {
		t.Fatalf("Expected 2 jobs, got %d", len(result))
	}
	if result[0].Name != "unit-tests" {
		t.Errorf("Expected unit-tests first (no deps), got %q", result[0].Name)
	}
	if result[1].Name != "integration-tests" {
		t.Errorf("Expected integration-tests second, got %q", result[1].Name)
	}
}

func TestScheduleJobs_Chain(t *testing.T) {
	jobs := []models.Job{
		{Name: "c", Stage: "test", Needs: []string{"b"}},
		{Name: "a", Stage: "test", Needs: nil},
		{Name: "b", Stage: "test", Needs: []string{"a"}},
	}
	result := ScheduleJobs(jobs)
	names := make([]string, len(result))
	for i, j := range result {
		names[i] = j.Name
	}
	expected := []string{"a", "b", "c"}
	if !reflect.DeepEqual(names, expected) {
		t.Errorf("Expected %v, got %v", expected, names)
	}
}

func TestScheduleJobs_ParallelThenJoin(t *testing.T) {
	// a and b have no deps; c needs both a and b
	jobs := []models.Job{
		{Name: "c", Stage: "test", Needs: []string{"a", "b"}},
		{Name: "a", Stage: "test", Needs: nil},
		{Name: "b", Stage: "test", Needs: nil},
	}
	result := ScheduleJobs(jobs)
	if len(result) != 3 {
		t.Fatalf("Expected 3 jobs, got %d", len(result))
	}
	// a and b must come before c
	cIdx := -1
	for i, j := range result {
		if j.Name == "c" {
			cIdx = i
			break
		}
	}
	if cIdx < 0 {
		t.Fatal("Expected job 'c' in result")
	}
	aBeforeC := false
	bBeforeC := false
	for i := 0; i < cIdx; i++ {
		if result[i].Name == "a" {
			aBeforeC = true
		}
		if result[i].Name == "b" {
			bBeforeC = true
		}
	}
	if !aBeforeC {
		t.Error("Expected 'a' before 'c'")
	}
	if !bBeforeC {
		t.Error("Expected 'b' before 'c'")
	}
}
