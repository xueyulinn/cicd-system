package worker

import (
	"context"
	"strings"
	"testing"

	"github.com/xueyulinn/cicd-system/internal/messages"
)

func TestHandleJobMessageRequiresDockerClient(t *testing.T) {
	svc := &Service{docker: nil}
	err := svc.handleJobMessage(context.Background(), messages.JobExecutionMessage{Pipeline: "p", RunNo: 1, Stage: "s"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "docker client not available") {
		t.Fatalf("err=%v", err)
	}
}
