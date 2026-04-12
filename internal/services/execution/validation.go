package execution

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/CS7580-SEA-SP26/e-team/internal/api"
	"github.com/CS7580-SEA-SP26/e-team/internal/common/parser"
	"github.com/CS7580-SEA-SP26/e-team/internal/common/planner"
)

// PrepareRun validates the incoming YAML and returns static execution plan and pipeline dto.
func (s *Service) prepareRun(req api.RunRequest) (*PreparedRun, *api.RunResponse, error) {
	if strings.TrimSpace(req.YAMLContent) == "" {
		return nil, &api.RunResponse{
			Pipeline: "",
			Status:   "failed",
			Errors:   []string{"yaml_content is required"},
		}, nil
	}

	validationResp, err := s.validatePipeline(req.YAMLContent)
	if err != nil {
		return nil, nil, fmt.Errorf("run pipeline: %w", err)
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
func (s *Service) validatePipeline(yamlContent string) (*api.ValidateResponse, error) {
	validateReq := map[string]string{
		"yaml_content": yamlContent,
	}

	jsonBody, err := json.Marshal(validateReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal validation request: %w", err)
	}

	resp, err := s.httpValidation.Post(s.validationURL+"/validate", "application/json", bytes.NewBuffer(jsonBody))
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
