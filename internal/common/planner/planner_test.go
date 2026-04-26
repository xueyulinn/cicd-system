package planner

import (
	"reflect"
	"testing"

	"github.com/xueyulinn/cicd-system/internal/models"
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
		return
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
		return
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
		return
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

// Test job ordering within a stage (Needs dependency): unit-tests before integration-tests
func TestGenerateExecutionPlan_JobOrderWithDependencies(t *testing.T) {
	pipeline := &models.Pipeline{
		Stages: []models.Stage{{Name: "test"}},
		Jobs: []models.Job{
			{Name: "integration-tests", Stage: "test", Needs: []string{"unit-tests"}},
			{Name: "unit-tests", Stage: "test", Needs: nil},
		},
	}
	plan, err := GenerateExecutionPlan(pipeline)
	if err != nil {
		t.Fatalf("GenerateExecutionPlan: %v", err)
	}
	stage, _ := getStagePlan(plan, "test")
	if len(stage.Jobs) != 2 {
		t.Fatalf("Expected 2 jobs, got %d", len(stage.Jobs))
	}
	if stage.Jobs[0].Name != "unit-tests" {
		t.Errorf("Expected unit-tests first (no deps), got %q", stage.Jobs[0].Name)
	}
	if stage.Jobs[1].Name != "integration-tests" {
		t.Errorf("Expected integration-tests second, got %q", stage.Jobs[1].Name)
	}
}

// Test job ordering chain: a -> b -> c
func TestGenerateExecutionPlan_JobOrderChain(t *testing.T) {
	pipeline := &models.Pipeline{
		Stages: []models.Stage{{Name: "test"}},
		Jobs: []models.Job{
			{Name: "c", Stage: "test", Needs: []string{"b"}},
			{Name: "a", Stage: "test", Needs: nil},
			{Name: "b", Stage: "test", Needs: []string{"a"}},
		},
	}
	plan, err := GenerateExecutionPlan(pipeline)
	if err != nil {
		t.Fatalf("GenerateExecutionPlan: %v", err)
	}
	stage, _ := getStagePlan(plan, "test")
	names := make([]string, len(stage.Jobs))
	for i, j := range stage.Jobs {
		names[i] = j.Name
	}
	expected := []string{"a", "b", "c"}
	if !reflect.DeepEqual(names, expected) {
		t.Errorf("Expected job order %v, got %v", expected, names)
	}
}

// Test parallel then join: a,b no deps; c needs both a and b
func TestGenerateExecutionPlan_JobOrderParallelThenJoin(t *testing.T) {
	pipeline := &models.Pipeline{
		Stages: []models.Stage{{Name: "test"}},
		Jobs: []models.Job{
			{Name: "c", Stage: "test", Needs: []string{"a", "b"}},
			{Name: "a", Stage: "test", Needs: nil},
			{Name: "b", Stage: "test", Needs: nil},
		},
	}
	plan, err := GenerateExecutionPlan(pipeline)
	if err != nil {
		t.Fatalf("GenerateExecutionPlan: %v", err)
	}
	stage, _ := getStagePlan(plan, "test")
	if len(stage.Jobs) != 3 {
		t.Fatalf("Expected 3 jobs, got %d", len(stage.Jobs))
	}
	cIdx := -1
	for i, j := range stage.Jobs {
		if j.Name == "c" {
			cIdx = i
			break
		}
	}
	if cIdx < 0 {
		t.Fatal("Expected job 'c' in plan")
	}
	var aBeforeC, bBeforeC bool
	for i := 0; i < cIdx; i++ {
		if stage.Jobs[i].Name == "a" {
			aBeforeC = true
		}
		if stage.Jobs[i].Name == "b" {
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
