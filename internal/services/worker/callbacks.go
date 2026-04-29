package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/xueyulinn/cicd-system/internal/api"
	"github.com/xueyulinn/cicd-system/internal/messages"
)

func (s *Service) callbackJobStarted(ctx context.Context, msg messages.JobExecutionMessage) error {
	ctx, span := s.serviceTracer().Start(ctx, "callback.start.job")
	defer span.End()

	return s.postJobCallback(ctx, "/callbacks/job-started", api.JobStatusCallbackRequest{
		Pipeline: msg.PipelineName,
		RunNo:    msg.RunNo,
		Stage:    msg.Stage,
		Job:      msg.Job.Name,
		Status:   "started",
	})
}

func (s *Service) callbackJobFinished(ctx context.Context, msg messages.JobExecutionMessage, status string, logs string, errMsg string) error {
	ctx, span := s.serviceTracer().Start(ctx, "callback.finish.job")
	defer span.End()

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
	_, err := s.doJobCallbackRequest(ctx, path, payload)
	return err
}

func (s *Service) doJobCallbackRequest(ctx context.Context, path string, payload api.JobStatusCallbackRequest) ([]byte, error) {
	if s.httpClient == nil {
		return nil, fmt.Errorf("http client is not initialized")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal callback payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.orchestratorURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create callback request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send callback request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read callback response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("callback returned status %d", resp.StatusCode)
	}

	return respBody, nil
}
