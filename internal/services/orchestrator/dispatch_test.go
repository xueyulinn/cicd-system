package orchestrator

import (
	"context"
	"errors"
	"reflect"
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

type scriptedPublisher struct {
	published []messages.JobExecutionMessage
	errs      []error
	onPublish func(messages.JobExecutionMessage)
}

func (p *scriptedPublisher) PublishJob(_ context.Context, msg messages.JobExecutionMessage) error {
	p.published = append(p.published, msg)
	if p.onPublish != nil {
		p.onPublish(msg)
	}
	if len(p.errs) == 0 {
		return nil
	}
	err := p.errs[0]
	p.errs = p.errs[1:]
	return err
}

func (p *scriptedPublisher) Close() error { return nil }

func TestBuildJobExecutionMessage(t *testing.T) {
	svc := &Service{}
	job := models.JobExecutionPlan{Name: "compile", Image: "golang:1.25", Script: []string{"go test ./..."}}
	info := runInfo{RepoURL: "https://github.com/o/r.git", Commit: "abc", WorkspaceObjectName: "workspaces/pack-v1/commits/abc/workspace.tar.gz"}

	msg := svc.buildJobExecutionMessage(7, "pipe", "build", job, info)
	if msg.RunNo != 7 || msg.PipelineName != "pipe" || msg.Stage != "build" {
		t.Fatalf("unexpected envelope: %#v", msg)
	}
	if msg.Job.Name != "compile" || msg.RepoURL != info.RepoURL || msg.Commit != info.Commit || msg.WorkspaceObjectName != info.WorkspaceObjectName {
		t.Fatalf("unexpected message fields: %#v", msg)
	}
}

func TestNextPublisherRoundRobin(t *testing.T) {
	p1 := &capturePublisher{}
	p2 := &capturePublisher{}
	svc := &Service{publisherManager: &publisherManager{current: &publisherSet{pubs: []mq.Publisher{p1, p2}}}}

	set := svc.publisherManager.acquireCurrentSet()
	if set == nil {
		t.Fatal("expected current publisher set")
	}
	defer svc.publisherManager.releaseSet(set)

	if got := set.nextPublisher(); got != p1 {
		t.Fatal("first publisher should be p1")
	}
	if got := set.nextPublisher(); got != p2 {
		t.Fatal("second publisher should be p2")
	}
	if got := set.nextPublisher(); got != p1 {
		t.Fatal("third publisher should cycle to p1")
	}
}

func TestEnqueueJobNoPublisher(t *testing.T) {
	svc := &Service{publisherManager: &publisherManager{}}
	err := svc.enqueueJob(context.Background(), messages.JobExecutionMessage{PipelineName: "p", Stage: "s", Job: models.JobExecutionPlan{Name: "j"}})
	if err == nil {
		t.Fatal("enqueueJob error=nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "job publisher is not initialized") {
		t.Fatalf("error=%v", err)
	}
}

func TestEnqueueJobPublisherError(t *testing.T) {
	svc := &Service{publisherManager: &publisherManager{current: &publisherSet{pubs: []mq.Publisher{&capturePublisher{err: errors.New("publish fail")}}}}}
	err := svc.enqueueJob(context.Background(), messages.JobExecutionMessage{PipelineName: "p", Stage: "s", Job: models.JobExecutionPlan{Name: "j"}})
	if err == nil {
		t.Fatal("enqueueJob error=nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "publish fail") {
		t.Fatalf("error=%v", err)
	}
}

func TestEnqueueJobRetriesConnectionClosedAndRepublishesMessage(t *testing.T) {
	successPublisher := &capturePublisher{}
	successSet := &publisherSet{gen: 1, pubs: []mq.Publisher{successPublisher}}
	manager := &publisherManager{}

	firstPublisher := &scriptedPublisher{
		errs: []error{mq.ErrConnectionClosed},
		onPublish: func(messages.JobExecutionMessage) {
			manager.mu.Lock()
			manager.current = successSet
			manager.mu.Unlock()
		},
	}
	manager.current = &publisherSet{gen: 0, pubs: []mq.Publisher{firstPublisher}}

	svc := &Service{publisherManager: manager}
	msg := messages.JobExecutionMessage{
		RunNo:        7,
		PipelineName: "pipe",
		Stage:        "build",
		Job:          models.JobExecutionPlan{Name: "compile"},
	}

	err := svc.enqueueJob(context.Background(), msg)
	if err != nil {
		t.Fatalf("enqueueJob error: %v", err)
	}

	if len(firstPublisher.published) != 1 {
		t.Fatalf("first publisher publish count = %d, want 1", len(firstPublisher.published))
	}
	if len(successPublisher.published) != 1 {
		t.Fatalf("second publisher publish count = %d, want 1", len(successPublisher.published))
	}

	firstMsg := firstPublisher.published[0]
	secondMsg := successPublisher.published[0]
	if !reflect.DeepEqual(firstMsg, msg) {
		t.Fatalf("first published message = %#v, want %#v", firstMsg, msg)
	}
	if !reflect.DeepEqual(secondMsg, msg) {
		t.Fatalf("second published message = %#v, want %#v", secondMsg, msg)
	}
}

func TestEnqueueReadyJobsPublishesAllJobs(t *testing.T) {
	pub := &capturePublisher{}
	svc := &Service{publisherManager: &publisherManager{current: &publisherSet{pubs: []mq.Publisher{pub}}}}
	jobs := []models.JobExecutionPlan{{Name: "a"}, {Name: "b"}}

	err := svc.enqueueReadyJobs(context.Background(), "pipe", "build", 2, jobs, runInfo{})
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

func TestEnqueueFirstReadyStageJobsPropagatesStageError(t *testing.T) {
	state := newStageState(models.StagePlan{
		Jobs:      []models.JobExecutionPlan{{Name: "compile"}},
		InDegree:  map[string]int{"compile": 0},
		JobByName: map[string]models.JobExecutionPlan{"compile": {Name: "compile"}},
	})

	svc := &Service{publisherManager: &publisherManager{}}
	err := svc.enqueueFirstReadyStageJobs(
		context.Background(),
		"pipe",
		1,
		[]models.StageExecutionPlan{{Name: "build", Jobs: []models.JobExecutionPlan{{Name: "compile"}}}},
		map[string]*stageState{"build": state},
		runInfo{},
	)
	if err == nil {
		t.Fatal("enqueueFirstReadyStageJobs error=nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "enqueue ready jobs for stage \"build\"") {
		t.Fatalf("error=%v", err)
	}
}

func TestDispatchPipelineStartJobsRequiresInitializedRuntime(t *testing.T) {
	svc := &Service{}
	err := svc.dispatchPipelineStartJobs(context.Background(), nil)
	if err == nil {
		t.Fatal("dispatchPipelineStartJobs error=nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "pipeline runtime is required") {
		t.Fatalf("error=%v", err)
	}
}

func TestEnqueueFirstReadyStageJobsSkipsMissingStageStates(t *testing.T) {
	svc := &Service{}
	err := svc.enqueueFirstReadyStageJobs(
		context.Background(),
		"pipe",
		1,
		[]models.StageExecutionPlan{{Name: "build", Jobs: []models.JobExecutionPlan{{Name: "compile"}}}},
		map[string]*stageState{},
		runInfo{},
	)
	if err != nil {
		t.Fatalf("enqueueFirstReadyStageJobs returned error: %v", err)
	}
}

func TestEnqueueFirstReadyStageJobsContinuesWhenStageHasNoReadyJobs(t *testing.T) {
	svc := &Service{}
	state := newStageState(models.StagePlan{
		Jobs:      []models.JobExecutionPlan{{Name: "compile"}},
		InDegree:  map[string]int{"compile": 1},
		JobByName: map[string]models.JobExecutionPlan{"compile": {Name: "compile"}},
	})

	err := svc.enqueueFirstReadyStageJobs(
		context.Background(),
		"pipe",
		1,
		[]models.StageExecutionPlan{{Name: "build", Jobs: []models.JobExecutionPlan{{Name: "compile"}}}},
		map[string]*stageState{"build": state},
		runInfo{},
	)
	if err != nil {
		t.Fatalf("enqueueFirstReadyStageJobs returned error: %v", err)
	}
}

func TestDispatchPipelineStartJobsNoStagesSucceeds(t *testing.T) {
	svc := &Service{}
	runtime := &pipelineRuntime{
		pipeline:      "demo",
		runNo:         1,
		executionPlan: models.ExecutionPlan{Stages: nil},
		stageStates:   map[string]*stageState{},
		runInfo:       runInfo{},
		pipelineStart: time.Now().UTC(),
	}

	err := svc.dispatchPipelineStartJobs(context.Background(), runtime)
	if err != nil {
		t.Fatalf("dispatchPipelineStartJobs returned error: %v", err)
	}
}
