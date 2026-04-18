package worker

import (
	"context"
	"strings"
	"testing"

	"github.com/xueyulinn/cicd-system/internal/models"
)

func TestExecuteJob_RequiresClientAndJob(t *testing.T) {
	if _, err := ExecuteJob(context.Background(), nil, nil, "", "", ""); err == nil {
		t.Fatal("expected error for nil client/job")
	}

	job := &models.JobExecutionPlan{Name: "x"}
	if _, err := ExecuteJob(context.Background(), nil, job, "", "", ""); err == nil {
		t.Fatal("expected error for nil client")
	}
}

func TestMaterializeWorkspace_InvalidCloneURL(t *testing.T) {
	path, cleanup, err := materializeWorkspace(context.Background(), "://bad-url", "abc", "")
	if err == nil {
		t.Fatal("expected clone error")
	}
	if cleanup != nil {
		t.Fatal("cleanup should be nil on clone failure")
	}
	if path != "" {
		t.Fatalf("path=%q, want empty", path)
	}
}

func TestBuildWorkspaceArchive_PathNotFound(t *testing.T) {
	_, err := buildWorkspaceArchive("/path/does/not/exist")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "cannot find") && !strings.Contains(strings.ToLower(err.Error()), "no such") {
		t.Fatalf("err=%v", err)
	}
}
