package store

import (
	"context"
	"os"
	"testing"
	"time"
)

func connURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = os.Getenv("REPORT_DB_URL")
	}
	if url == "" {
		t.Skip("DATABASE_URL or REPORT_DB_URL not set, skipping store tests")
	}
	return url
}

func TestStore_CreateRun_GetRun_GetRunsByPipeline(t *testing.T) {
	ctx := context.Background()
	url := connURL(t)
	s, err := New(ctx, url)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Close()

	pipeline := "test-pipeline-create-get"
	start := time.Now().UTC()

	runNo, err := s.CreateRun(ctx, CreateRunInput{
		Pipeline:  pipeline,
		StartTime: start,
		Status:    StatusRunning,
		GitHash:   "abc123",
		GitRepo:   "https://example.com/repo",
	})
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if runNo < 1 {
		t.Fatalf("expected run_no >= 1, got %d", runNo)
	}

	got, err := s.GetRun(ctx, pipeline, runNo)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.Pipeline != pipeline || got.RunNo != runNo || got.Status != StatusRunning {
		t.Fatalf("GetRun: got pipeline=%q run_no=%d status=%q", got.Pipeline, got.RunNo, got.Status)
	}
	if got.GitHash != "abc123" || got.GitRepo != "https://example.com/repo" {
		t.Fatalf("GetRun: git fields: hash=%q repo=%q", got.GitHash, got.GitRepo)
	}

	runs, err := s.GetRunsByPipeline(ctx, pipeline)
	if err != nil {
		t.Fatalf("GetRunsByPipeline: %v", err)
	}
	if len(runs) < 1 {
		t.Fatalf("GetRunsByPipeline: expected at least 1 run, got %d", len(runs))
	}
	found := false
	for _, r := range runs {
		if r.RunNo == runNo {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("GetRunsByPipeline: run_no %d not found in %+v", runNo, runs)
	}
}

func TestStore_GetRun_NotFound(t *testing.T) {
	ctx := context.Background()
	url := connURL(t)
	s, err := New(ctx, url)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Close()

	_, err = s.GetRun(ctx, "nonexistent-pipeline-xyz", 99999)
	if err == nil {
		t.Fatal("expected ErrNotFound")
	}
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestStore_CreateStage_UpdateStage_GetStagesForRun(t *testing.T) {
	ctx := context.Background()
	url := connURL(t)
	s, err := New(ctx, url)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Close()

	pipeline := "test-pipeline-stage"
	runNo, err := s.CreateRun(ctx, CreateRunInput{
		Pipeline:  pipeline,
		StartTime: time.Now().UTC(),
		Status:    StatusRunning,
	})
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	err = s.CreateStage(ctx, CreateStageInput{
		Pipeline:  pipeline,
		RunNo:     runNo,
		Stage:     "build",
		StartTime: time.Now().UTC(),
		Status:    StatusRunning,
	})
	if err != nil {
		t.Fatalf("CreateStage: %v", err)
	}

	end := time.Now().UTC()
	err = s.UpdateStage(ctx, pipeline, runNo, "build", UpdateStageInput{EndTime: &end, Status: StatusSuccess})
	if err != nil {
		t.Fatalf("UpdateStage: %v", err)
	}

	stages, err := s.GetStagesForRun(ctx, pipeline, runNo, "")
	if err != nil {
		t.Fatalf("GetStagesForRun: %v", err)
	}
	if len(stages) != 1 || stages[0].Stage != "build" || stages[0].Status != StatusSuccess {
		t.Fatalf("GetStagesForRun: got %+v", stages)
	}
}

func TestStore_CreateJob_UpdateJob_GetJobsForRun(t *testing.T) {
	ctx := context.Background()
	url := connURL(t)
	s, err := New(ctx, url)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Close()

	pipeline := "test-pipeline-job"
	runNo, err := s.CreateRun(ctx, CreateRunInput{
		Pipeline:  pipeline,
		StartTime: time.Now().UTC(),
		Status:    StatusRunning,
	})
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	_ = s.CreateStage(ctx, CreateStageInput{
		Pipeline: pipeline, RunNo: runNo, Stage: "build",
		StartTime: time.Now().UTC(), Status: StatusRunning,
	})

	err = s.CreateJob(ctx, CreateJobInput{
		Pipeline: pipeline, RunNo: runNo, Stage: "build", Job: "compile",
		StartTime: time.Now().UTC(), Status: StatusRunning,
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	end := time.Now().UTC()
	err = s.UpdateJob(ctx, pipeline, runNo, "build", "compile", UpdateJobInput{EndTime: &end, Status: StatusSuccess})
	if err != nil {
		t.Fatalf("UpdateJob: %v", err)
	}

	jobs, err := s.GetJobsForRun(ctx, pipeline, runNo, "", "")
	if err != nil {
		t.Fatalf("GetJobsForRun: %v", err)
	}
	if len(jobs) != 1 || jobs[0].Job != "compile" || jobs[0].Status != StatusSuccess {
		t.Fatalf("GetJobsForRun: got %+v", jobs)
	}
}

func TestStore_CreateRunOrGetActive_DeduplicatesIdenticalInFlightRun(t *testing.T) {
	ctx := context.Background()
	url := connURL(t)
	s, err := New(ctx, url)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Close()

	pipeline := "test-pipeline-dedupe"
	input := CreateRunInput{
		Pipeline:   pipeline,
		StartTime:  time.Now().UTC(),
		Status:     StatusQueued,
		GitHash:    "abc123",
		GitRepo:    "https://example.com/repo",
		RequestKey: "same-request",
	}

	first, err := s.CreateRunOrGetActive(ctx, input)
	if err != nil {
		t.Fatalf("CreateRunOrGetActive first: %v", err)
	}
	if first.Deduped {
		t.Fatal("expected first request to create a new run")
	}

	second, err := s.CreateRunOrGetActive(ctx, input)
	if err != nil {
		t.Fatalf("CreateRunOrGetActive second: %v", err)
	}
	if !second.Deduped {
		t.Fatal("expected second request to dedupe to existing run")
	}
	if second.RunNo != first.RunNo {
		t.Fatalf("expected deduped run_no %d, got %d", first.RunNo, second.RunNo)
	}

	runs, err := s.GetRunsByPipeline(ctx, pipeline)
	if err != nil {
		t.Fatalf("GetRunsByPipeline: %v", err)
	}
	count := 0
	for _, run := range runs {
		if run.RequestKey == input.RequestKey {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 persisted run for request_key %q, got %d", input.RequestKey, count)
	}
}
