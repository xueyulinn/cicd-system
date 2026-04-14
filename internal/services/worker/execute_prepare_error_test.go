package worker

import (
	"context"
	"strings"
	"testing"

	"github.com/CS7580-SEA-SP26/e-team/internal/models"
	"github.com/moby/moby/client"
)

func TestExecuteJob_PrepareWorkspaceError(t *testing.T) {
	job := &models.JobExecutionPlan{Name: "compile"}
	_, err := ExecuteJob(context.Background(), &client.Client{}, job, "://bad-url", "abc", "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "prepare workspace") {
		t.Fatalf("err=%v", err)
	}
}
