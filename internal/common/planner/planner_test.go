package planner

import (
	"reflect"
	"testing"

	"github.com/CS7580-SEA-SP26/e-team/internal/models"
)

func getJobPlan(jobs []models.JobExecutionPlan, name string) (models.JobExecutionPlan, bool) {
	for _, j := range jobs {
		if j.Name == name {
			return j, true
		}
	}
	return models.JobExecutionPlan{}, false
}

func getStagePlan(plan *models.ExecutionPlan, stageName string) (models.StageExecutionPlan, bool) {
	for _, s := range plan.Stages {
		if s.Name == stageName {
			return s, true
		}
	}
	return models.StageExecutionPlan{}, false
}

func TestGenerateExecutionPlan_SingleStageSingleJob(t *testing.T) {
	pipeline := &models.Pipeline{
		Stages: []models.Stage{{Name: "build"}},
		Jobs: []models.Job{
			{Name: "compile", Stage: "build", Image: "golang:1.21", Script: []string{"go build"}},
		},
	}

	plan, err := GenerateExecutionPlan(pipeline)
	if err != nil {
		t.Fatalf("GenerateExecutionPlan: %v", err)
	}
	if plan == nil {
		t.Fatal("Expected non-nil plan")
	}
	s, ok := getStagePlan(plan, "build")
	if !ok {
		t.Fatal("Expected 'build' stage in plan")
	}
	job, ok := getJobPlan(s.Jobs, "compile")
	if !ok {
		t.Fatal("Expected 'compile' job in build stage")
	}
	if job.Image != "golang:1.21" {
		t.Errorf("Expected image 'golang:1.21', got %q", job.Image)
	}
	if !reflect.DeepEqual(job.Script, []string{"go build"}) {
		t.Errorf("Expected script [\"go build\"], got %v", job.Script)
	}
}

func TestGenerateExecutionPlan_MultipleStagesMultipleJobs(t *testing.T) {
	pipeline := &models.Pipeline{
		Stages: []models.Stage{{Name: "build"}, {Name: "test"}},
		Jobs: []models.Job{
			{Name: "compile", Stage: "build", Image: "golang:1.21", Script: []string{"go build"}},
			{Name: "unit-tests", Stage: "test", Image: "golang:1.21", Script: []string{"go test"}},
			{Name: "integration-tests", Stage: "test", Image: "golang:1.21", Script: []string{"go test ./..."}},
		},
	}

	plan, err := GenerateExecutionPlan(pipeline)
	if err != nil {
		t.Fatalf("GenerateExecutionPlan: %v", err)
	}
	if plan == nil {
		t.Fatal("Expected non-nil plan")
	}

	build, _ := getStagePlan(plan, "build")
	if len(build.Jobs) != 1 {
		t.Errorf("Expected 1 job in build, got %d", len(build.Jobs))
	}
	if out, _ := getJobPlan(build.Jobs, "compile"); out.Image != "golang:1.21" {
		t.Errorf("Expected compile image 'golang:1.21', got %q", out.Image)
	}

	test, _ := getStagePlan(plan, "test")
	if len(test.Jobs) != 2 {
		t.Errorf("Expected 2 jobs in test, got %d", len(test.Jobs))
	}
	if out, _ := getJobPlan(test.Jobs, "unit-tests"); out.Image != "golang:1.21" {
		t.Errorf("Expected unit-tests image 'golang:1.21', got %q", out.Image)
	}
	if out, _ := getJobPlan(test.Jobs, "integration-tests"); out.Script[0] != "go test ./..." {
		t.Errorf("Expected integration-tests script, got %v", out.Script)
	}
}

func TestGenerateExecutionPlan_EmptyStages(t *testing.T) {
	pipeline := &models.Pipeline{
		Stages: []models.Stage{},
		Jobs:   []models.Job{},
	}

	plan, err := GenerateExecutionPlan(pipeline)
	if err != nil {
		t.Fatalf("GenerateExecutionPlan: %v", err)
	}
	if plan == nil {
		t.Fatal("Expected non-nil plan")
	}
	if len(plan.Stages) != 0 {
		t.Errorf("Expected empty stages, got %d", len(plan.Stages))
	}
}

func TestGenerateExecutionPlan_StageWithNoJobs(t *testing.T) {
	pipeline := &models.Pipeline{
		Stages: []models.Stage{{Name: "build"}, {Name: "empty-stage"}},
		Jobs: []models.Job{
			{Name: "compile", Stage: "build", Image: "golang:1.21", Script: []string{"go build"}},
		},
	}

	plan, err := GenerateExecutionPlan(pipeline)
	if err != nil {
		t.Fatalf("GenerateExecutionPlan: %v", err)
	}

	if _, ok := getStagePlan(plan, "build"); !ok {
		t.Fatal("Expected 'build' stage")
	}
	empty, ok := getStagePlan(plan, "empty-stage")
	if !ok {
		t.Fatal("Expected 'empty-stage' in plan")
	}
	if len(empty.Jobs) != 0 {
		t.Errorf("Expected empty-stage to have 0 jobs, got %d", len(empty.Jobs))
	}
}

func TestGenerateExecutionPlan_JobWithMultipleScriptLines(t *testing.T) {
	pipeline := &models.Pipeline{
		Stages: []models.Stage{{Name: "build"}},
		Jobs: []models.Job{
			{
				Name:   "build",
				Stage:  "build",
				Image:  "gradle:8.12-jdk21",
				Script: []string{"./gradlew classes", "./gradlew jar"},
			},
		},
	}

	plan, err := GenerateExecutionPlan(pipeline)
	if err != nil {
		t.Fatalf("GenerateExecutionPlan: %v", err)
	}

	build, _ := getStagePlan(plan, "build")
	job, _ := getJobPlan(build.Jobs, "build")
	expected := []string{"./gradlew classes", "./gradlew jar"}
	if !reflect.DeepEqual(job.Script, expected) {
		t.Errorf("Expected script %v, got %v", expected, job.Script)
	}
	if job.Image != "gradle:8.12-jdk21" {
		t.Errorf("Expected image 'gradle:8.12-jdk21', got %q", job.Image)
	}
}

func TestGenerateExecutionPlan_StageOrderPreserved(t *testing.T) {
	pipeline := &models.Pipeline{
		Stages: []models.Stage{{Name: "build"}, {Name: "test"}, {Name: "doc"}, {Name: "deploy"}},
		Jobs: []models.Job{
			{Name: "compile", Stage: "build", Image: "gradle:jdk21", Script: []string{"gradle build"}},
			{Name: "unit-tests", Stage: "test", Image: "gradle:jdk21", Script: []string{"gradle test"}},
			{Name: "javadoc", Stage: "doc", Image: "gradle:jdk21", Script: []string{"gradle javadoc"}},
			{Name: "package", Stage: "deploy", Image: "gradle:jdk21", Script: []string{"gradle assembleDist"}},
		},
	}

	plan, err := GenerateExecutionPlan(pipeline)
	if err != nil {
		t.Fatalf("GenerateExecutionPlan: %v", err)
	}

	expectedOrder := []string{"build", "test", "doc", "deploy"}
	if len(plan.Stages) != len(expectedOrder) {
		t.Fatalf("Expected %d stages, got %d", len(expectedOrder), len(plan.Stages))
	}
	for i, name := range expectedOrder {
		if plan.Stages[i].Name != name {
			t.Errorf("Stage %d: expected %q, got %q", i, name, plan.Stages[i].Name)
		}
	}
}
