package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/xueyulinn/cicd-system/internal/api"
	"github.com/xueyulinn/cicd-system/internal/common/parser"
	"github.com/xueyulinn/cicd-system/internal/common/planner"
)

// PrepareRun validates the incoming YAML and returns static execution plan and pipeline dto.
func (s *Service) prepareRun(ctx context.Context, req api.RunRequest) (*PreparedRun, *api.RunResponse, error) {
	ctx, span := s.tracer.Start(ctx, "prepare.pipeline")
	defer span.End()

	if strings.TrimSpace(req.YAMLContent) == "" {
		return nil, &api.RunResponse{
			Pipeline: "",
			Status:   "failed",
			Errors:   []string{"yaml_content is required"},
		}, nil
	}

	validationResp, err := s.validatePipeline(ctx, req.YAMLContent)
	if err != nil {
		return nil, nil, fmt.Errorf("validate pipeline failed: %w", err)
	}

	if !validationResp.Valid {
		return nil, &api.RunResponse{
			Pipeline: "",
			Status:   "failed",
			Errors:   validationResp.Errors,
		}, nil
	}

	p := parser.NewParserFromContent(req.YAMLContent)
	pipeline, _, err := p.Parse()
	if err != nil {
		return nil, &api.RunResponse{
			Pipeline: "",
			Status:   "failed",
			Errors:   []string{fmt.Sprintf("pipeline parse failed: %v", err)},
		}, nil
	}

	// generate static executionPlan for current pipeline run
	executionPlan, err := planner.GenerateExecutionPlan(pipeline)
	if err != nil {
		return nil, &api.RunResponse{
			Pipeline: pipeline.Name,
			Status:   "failed",
			Errors:   []string{fmt.Sprintf("generate execution plan failed: %v", err)},
		}, nil
	}

	return &PreparedRun{
		Pipeline:      pipeline,
		ExecutionPlan: executionPlan,
	}, nil, nil
}

// validatePipeline calls validation service and returns validation result.
func (s *Service) validatePipeline(ctx context.Context, yamlContent string) (*api.ValidateResponse, error) {
	ctx, span := s.tracer.Start(ctx, "validate.pipeline")
	defer span.End()

	validateReq := map[string]string{
		"yaml_content": yamlContent,
	}

	bodyBytes, err := json.Marshal(validateReq)
	if err != nil {
		return nil, fmt.Errorf("fail to marshal request body %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.validationURL+"/validate", bytes.NewReader(bodyBytes))

	resp, err := s.validationClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call validation service: %w", err)
	}
	defer func() {
		_ = resp.Body.Close() // Ignore close error as we're done with the body
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read validation response: %w", err)
	}

	var validationResp api.ValidateResponse
	if err := json.Unmarshal(body, &validationResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal validation response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Keep server-provided validation details when available.
		if len(validationResp.Errors) == 0 {
			validationResp.Errors = []string{fmt.Sprintf("validation service returned status %d", resp.StatusCode)}
		}
		validationResp.Valid = false
	}

	return &validationResp, nil
}
