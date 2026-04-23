package orchestrator

import (
	"testing"
	"time"
)

func TestRuntimeKey(t *testing.T) {
	if got := runtimeKey("pipeline-a", 42); got != "pipeline-a:42" {
		t.Fatalf("runtimeKey()=%q, want %q", got, "pipeline-a:42")
	}
}

func TestServiceRuntimePutGetDelete(t *testing.T) {
	svc := &Service{runtimes: make(map[string]*pipelineRuntime)}
	rt := &pipelineRuntime{pipeline: "demo", runNo: 3, jobStartTimes: map[jobKey]time.Time{}}

	svc.putRuntime(rt)
	got := svc.getPipelineRuntime("demo", 3)
	if got == nil || got.runNo != 3 || got.pipeline != "demo" {
		t.Fatalf("getPipelineRuntime=%#v, want runtime demo:3", got)
	}

	svc.deleteRuntime("demo", 3)
	if got := svc.getPipelineRuntime("demo", 3); got != nil {
		t.Fatalf("runtime should be deleted, got %#v", got)
	}
}

func TestNoteAndPopJobStartTime(t *testing.T) {
	svc := &Service{runtimes: make(map[string]*pipelineRuntime)}
	rt := &pipelineRuntime{
		pipeline:      "demo",
		runNo:         1,
		jobStartTimes: make(map[jobKey]time.Time),
	}
	svc.putRuntime(rt)

	svc.noteJobStarted("demo", 1, "build", "compile")
	start := svc.popJobStartTime("demo", 1, "build", "compile")
	if start.IsZero() {
		t.Fatal("expected non-zero start time")
	}
	again := svc.popJobStartTime("demo", 1, "build", "compile")
	if !again.IsZero() {
		t.Fatal("expected start time to be removed after pop")
	}
}
