package orchestrator

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/xueyulinn/cicd-system/internal/messages"
	"github.com/xueyulinn/cicd-system/internal/models"
	"github.com/xueyulinn/cicd-system/internal/mq"
)

type capturePublisher struct {
	published []messages.JobExecutionMessage
	err       error
}

func (p *capturePublisher) PublishJob(_ context.Context, msg messages.JobExecutionMessage) error {
	if p.err != nil {
		return p.err
	}
	p.published = append(p.published, msg)
	return nil
}

func (p *capturePublisher) Close() error { return nil }

func TestBuildJobExecutionMessage(t *testing.T) {
	svc := &Service{}
	job := models.JobExecutionPlan{Name: "compile", Image: "golang:1.25", Script: []string{"go test ./..."}}
	info := runInfo{RepoURL: "https://github.com/o/r.git", Branch: "main", Commit: "abc", WorkspacePath: "/tmp/wt"}

	msg := svc.buildJobExecutionMessage(7, "pipe", "build", job, info)
	if msg.RunNo != 7 || msg.Pipeline != "pipe" || msg.Stage != "build" {
		t.Fatalf("unexpected envelope: %#v", msg)
	}
	if msg.Job.Name != "compile" || msg.RepoURL != info.RepoURL || msg.Commit != info.Commit || msg.WorkspacePath != info.WorkspacePath {
		t.Fatalf("unexpected message fields: %#v", msg)
	}
}

func TestNextPublisherRoundRobin(t *testing.T) {
	p1 := &capturePublisher{}
	p2 := &capturePublisher{}
	svc := &Service{jobPublishers: []mq.Publisher{p1, p2}}

	if got := svc.nextPublisher(); got != p1 {
		t.Fatal("first publisher should be p1")
	}
	if got := svc.nextPublisher(); got != p2 {
		t.Fatal("second publisher should be p2")
	}
	if got := svc.nextPublisher(); got != p1 {
		t.Fatal("third publisher should cycle to p1")
	}
}

func TestEnqueueJobNoPublisher(t *testing.T) {
	svc := &Service{}
	err := svc.enqueueJob(context.Background(), messages.JobExecutionMessage{Pipeline: "p", Stage: "s", Job: models.JobExecutionPlan{Name: "j"}})
	if err == nil {
		t.Fatal("enqueueJob error=nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "job publisher is not initialized") {
		t.Fatalf("error=%v", err)
	}
}

func TestEnqueueJobPublisherError(t *testing.T) {
	svc := &Service{jobPublishers: []mq.Publisher{&capturePublisher{err: errors.New("publish fail")}}}
	err := svc.enqueueJob(context.Background(), messages.JobExecutionMessage{Pipeline: "p", Stage: "s", Job: models.JobExecutionPlan{Name: "j"}})
	if err == nil {
		t.Fatal("enqueueJob error=nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "publish fail") {
		t.Fatalf("error=%v", err)
	}
}

func TestEnqueueReadyJobsPublishesAllJobs(t *testing.T) {
	pub := &capturePublisher{}
	svc := &Service{jobPublishers: []mq.Publisher{pub}}
	jobs := []models.JobExecutionPlan{{Name: "a"}, {Name: "b"}}

	err := svc.enqueueReadyJobs(context.Background(), "pipe", "build", 2, jobs, runInfo{Branch: "main"})
	if err != nil {
		t.Fatalf("enqueueReadyJobs error: %v", err)
	}
	if len(pub.published) != 2 {
		t.Fatalf("published=%d, want 2", len(pub.published))
	}
	if pub.published[0].Job.Name != "a" || pub.published[1].Job.Name != "b" {
		t.Fatalf("unexpected published jobs: %#v", pub.published)
	}
}

func TestEnqueueInitialReadyJobsPropagatesStageError(t *testing.T) {
	state := newStageState(models.StagePlan{
		Jobs:      []models.JobExecutionPlan{{Name: "compile"}},
		InDegree:  map[string]int{"compile": 0},
		JobByName: map[string]models.JobExecutionPlan{"compile": {Name: "compile"}},
	})

	svc := &Service{}
	err := svc.enqueueInitialReadyJobs(
		context.Background(),
		"pipe",
		1,
		[]models.StageExecutionPlan{{Name: "build", Jobs: []models.JobExecutionPlan{{Name: "compile"}}}},
		map[string]*stageState{"build": state},
		runInfo{},
	)
	if err == nil {
		t.Fatal("enqueueInitialReadyJobs error=nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "enqueue ready jobs for stage \"build\"") {
		t.Fatalf("error=%v", err)
	}
}

func TestDispatchInitialReadyJobsRequiresInitializedRuntime(t *testing.T) {
	svc := &Service{}
	err := svc.dispatchInitialReadyJobs(context.Background(), PreparedRun{}, nil)
	if err == nil {
		t.Fatal("dispatchInitialReadyJobs error=nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "initialized run is required") {
		t.Fatalf("error=%v", err)
	}
}

func TestEnqueueInitialReadyJobsSkipsMissingStageStates(t *testing.T) {
	svc := &Service{}
	err := svc.enqueueInitialReadyJobs(
		context.Background(),
		"pipe",
		1,
		[]models.StageExecutionPlan{{Name: "build", Jobs: []models.JobExecutionPlan{{Name: "compile"}}}},
		map[string]*stageState{},
		runInfo{},
	)
	if err != nil {
		t.Fatalf("enqueueInitialReadyJobs returned error: %v", err)
	}
}

func TestEnqueueInitialReadyJobsContinuesWhenStageHasNoReadyJobs(t *testing.T) {
	svc := &Service{}
	state := newStageState(models.StagePlan{
		Jobs:      []models.JobExecutionPlan{{Name: "compile"}},
		InDegree:  map[string]int{"compile": 1},
		JobByName: map[string]models.JobExecutionPlan{"compile": {Name: "compile"}},
	})

	err := svc.enqueueInitialReadyJobs(
		context.Background(),
		"pipe",
		1,
		[]models.StageExecutionPlan{{Name: "build", Jobs: []models.JobExecutionPlan{{Name: "compile"}}}},
		map[string]*stageState{"build": state},
		runInfo{},
	)
	if err != nil {
		t.Fatalf("enqueueInitialReadyJobs returned error: %v", err)
	}
}

func TestDispatchInitialReadyJobsNoStagesSucceeds(t *testing.T) {
	svc := &Service{}
	prepared := PreparedRun{
		Pipeline:      &models.Pipeline{Name: "demo"},
		ExecutionPlan: &models.ExecutionPlan{Stages: nil},
	}
	initialized := &initializedRun{
		runNo: 1,
		runtime: &pipelineRuntime{
			pipeline:      "demo",
			runNo:         1,
			stageStates:   map[string]*stageState{},
			runInfo:       runInfo{},
			pipelineStart: time.Now().UTC(),
		},
	}

	err := svc.dispatchInitialReadyJobs(context.Background(), prepared, initialized)
	if err != nil {
		t.Fatalf("dispatchInitialReadyJobs returned error: %v", err)
	}
}
