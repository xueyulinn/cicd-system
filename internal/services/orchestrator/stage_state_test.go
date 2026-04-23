package orchestrator

import (
	"reflect"
	"testing"

	"github.com/xueyulinn/cicd-system/internal/models"
)

func TestNewStageStateAndReadyFlow(t *testing.T) {
	plan := models.StagePlan{
		Name: "build",
		Jobs: []models.JobExecutionPlan{
			{Name: "compile"},
			{Name: "unit"},
			{Name: "lint"},
		},
		Dependents: map[string][]string{
			"compile": {"unit", "lint"},
		},
		InDegree: map[string]int{
			"compile": 0,
			"unit":    1,
			"lint":    1,
		},
		JobByName: map[string]models.JobExecutionPlan{
			"compile": {Name: "compile"},
			"unit":    {Name: "unit"},
			"lint":    {Name: "lint"},
		},
	}

	state := newStageState(plan)

	ready := state.getReadyJobs()
	if len(ready) != 1 || ready[0].Name != "compile" {
		t.Fatalf("initial ready jobs = %#v, want [compile]", ready)
	}

	if again := state.getReadyJobs(); len(again) != 0 {
		t.Fatalf("second getReadyJobs() = %#v, want empty", again)
	}

	newlyReady := state.markJobSucceeded("compile")
	gotNames := []string{newlyReady[0].Name, newlyReady[1].Name}
	if !reflect.DeepEqual(gotNames, []string{"unit", "lint"}) {
		t.Fatalf("newlyReady after compile = %v, want [unit lint]", gotNames)
	}

	if state.isStageComplete() {
		t.Fatal("stage should not be complete yet")
	}

	_ = state.markJobSucceeded("unit")
	_ = state.markJobSucceeded("lint")
	if !state.isStageComplete() {
		t.Fatal("stage should be complete after all jobs finish")
	}
}

func TestMarkJobSucceeded_IdempotentAfterCompletion(t *testing.T) {
	plan := models.StagePlan{
		Jobs:       []models.JobExecutionPlan{{Name: "only"}},
		Dependents: map[string][]string{},
		InDegree:   map[string]int{"only": 0},
		JobByName:  map[string]models.JobExecutionPlan{"only": {Name: "only"}},
	}

	state := newStageState(plan)
	_ = state.getReadyJobs()
	if out := state.markJobSucceeded("only"); len(out) != 0 {
		t.Fatalf("first markJobSucceeded returned %#v, want empty", out)
	}
	if out := state.markJobSucceeded("only"); out != nil {
		t.Fatalf("second markJobSucceeded returned %#v, want nil", out)
	}
}

func TestPipelineRuntimeNextStageName(t *testing.T) {
	rt := &pipelineRuntime{stageOrder: []string{"build", "test", "deploy"}}

	next, ok := rt.nextStageName("build")
	if !ok || next != "test" {
		t.Fatalf("nextStageName(build) = (%q, %v), want (test, true)", next, ok)
	}

	_, ok = rt.nextStageName("deploy")
	if ok {
		t.Fatal("nextStageName(deploy) should not exist")
	}
}
