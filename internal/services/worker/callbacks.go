package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/xueyulinn/cicd-system/internal/api"
	"github.com/xueyulinn/cicd-system/internal/messages"
)

func (s *Service) callbackJobStarted(ctx context.Context, msg messages.JobExecutionMessage) error {
	return s.postJobCallback(ctx, "/callbacks/job-started", api.JobStatusCallbackRequest{
		Pipeline: msg.PipelineName,
		RunNo:    msg.RunNo,
		Stage:    msg.Stage,
		Job:      msg.Job.Name,
		Status:   "started",
	})
}

func (s *Service) callbackJobFinished(ctx context.Context, msg messages.JobExecutionMessage, status string, logs string, errMsg string) error {
	return s.postJobCallback(ctx, "/callbacks/job-finished", api.JobStatusCallbackRequest{
		Pipeline: msg.PipelineName,
		RunNo:    msg.RunNo,
		Stage:    msg.Stage,
		Job:      msg.Job.Name,
		Status:   status,
		Logs:     logs,
		Error:    errMsg,
	})
}

func (s *Service) postJobCallback(ctx context.Context, path string, payload api.JobStatusCallbackRequest) error {
	if s.httpClient == nil {
		return fmt.Errorf("http client is not initialized")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal callback payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.orchestratorURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create callback request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send callback request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("callback returned status %d", resp.StatusCode)
	}
	return nil
}
