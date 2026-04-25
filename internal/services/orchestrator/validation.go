package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/xueyulinn/cicd-system/internal/api"
	"github.com/xueyulinn/cicd-system/internal/common/parser"
	"github.com/xueyulinn/cicd-system/internal/common/planner"
)

// prepareRun validates the pipeline and returns a pipeline plan.
func (s *Service) prepareRun(ctx context.Context, req api.RunRequest) (*PipelinePlan, error) {
	ctx, span := s.serviceTracer().Start(ctx, "prepare.pipeline")
	defer span.End()

	if strings.TrimSpace(req.YAMLContent) == "" {
		return nil, fmt.Errorf("pipeline content can not be empty")
	}

	err := s.validatePipeline(ctx, req.YAMLContent)
	if err != nil {
		return nil, fmt.Errorf("validate pipeline failed: %w", err)
	}

	p := parser.NewParserFromContent(req.YAMLContent)
	pipeline, _, err := p.Parse()
	if err != nil {
		return nil, fmt.Errorf("pipeline parse failed: %w", err)
	}

	// generate static executionPlan for current pipeline run
	executionPlan, err := planner.GenerateExecutionPlan(pipeline)
	if err != nil {
		return nil, fmt.Errorf("generate execution plan failed: %w", err)
	}

	return &PipelinePlan{
		Pipeline:      pipeline,
		ExecutionPlan: executionPlan,
	}, nil
}

// validatePipeline calls validation service and returns error if validation fails.
func (s *Service) validatePipeline(ctx context.Context, yamlContent string) error {
	ctx, span := s.serviceTracer().Start(ctx, "validate.pipeline")
	defer span.End()

	validateReq := map[string]string{
		"yaml_content": yamlContent,
	}

	bodyBytes, err := json.Marshal(validateReq)
	if err != nil {
		return fmt.Errorf("fail to marshal request body %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.validationURL+"/validate", bytes.NewReader(bodyBytes))

	resp, err := s.validationClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call validation service: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read validation response body: %w", err)
	}

	var validationResp api.ValidateResponse
	if len(bodyBytes) > 0 {
		if err := json.Unmarshal(respBody, &validationResp); err != nil {
			return fmt.Errorf("failed to decode validation response: %w", err)
		}
	}

	if resp.StatusCode != http.StatusOK {
		// Keep server-provided validation details when available.
		if len(validationResp.Errors) == 0 {
			validationResp.Errors = []string{fmt.Sprintf("validation service returned status %d", resp.StatusCode)}
		}

		errs := make([]error, 0, len(validationResp.Errors))
		for _, msg := range validationResp.Errors {
			errs = append(errs, errors.New(msg))
		}

		return fmt.Errorf("validation service returned status %d: %w",
			resp.StatusCode,
			errors.Join(errs...))
	}

	if !validationResp.Valid {
		errs := make([]error, 0, len(validationResp.Errors))
		for _, msg := range validationResp.Errors {
			errs = append(errs, errors.New(msg))
		}

		return fmt.Errorf("validation failed: %w", errors.Join(errs...))
	}

	return nil
}
