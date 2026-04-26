package validation

import (
	"fmt"

	"github.com/xueyulinn/cicd-system/internal/api"
	"github.com/xueyulinn/cicd-system/internal/common/parser"
	"github.com/xueyulinn/cicd-system/internal/common/planner"
	"github.com/xueyulinn/cicd-system/internal/common/verifier"
	"github.com/xueyulinn/cicd-system/internal/models"
)

// Service provides validation functionality
type Service struct{}

// NewService creates a new validation service
func NewService() *Service {
	return &Service{}
}

func validatePipelineContent(yamlContent string) (*models.Pipeline, []string) {
	pipeline, rootNode, err := parser.NewParserFromContent(yamlContent).Parse()
	if err != nil {
		return nil, []string{err.Error()}
	}

	v := verifier.NewPipelineVerifier("content", pipeline, rootNode)
	errors := v.Verify()
	if len(errors) > 0 {
		errorStrings := make([]string, len(errors))
		for i, err := range errors {
			errorStrings[i] = err.Error()
		}
		return nil, errorStrings
	}

	return pipeline, nil
}

// ValidateYAML validates YAML pipeline content.
func (s *Service) ValidateYAML(yamlContent string) api.ValidateResponse {
	_, errors := validatePipelineContent(yamlContent)
	if len(errors) > 0 {
		return api.ValidateResponse{
			Valid:  false,
			Errors: errors,
		}
	}

	return api.ValidateResponse{
		Valid:  true,
		Errors: []string{},
	}
}

// DryRunYAML validates YAML and returns dry run output.
func (s *Service) DryRunYAML(yamlContent string) api.DryRunResponse {
	pipeline, errors := validatePipelineContent(yamlContent)
	if len(errors) > 0 {
		return api.DryRunResponse{
			Valid:  false,
			Errors: errors,
		}
	}

	// Build execution plan for dry run output
	plan, err := planner.GenerateExecutionPlan(pipeline)
	if err != nil {
		return api.DryRunResponse{
			Valid:  false,
			Errors: []string{fmt.Sprintf("failed to generate execution plan: %v", err)},
		}
	}

	return api.DryRunResponse{
		Valid:  true,
		Errors: []string{},
		ExecutionPlan: plan,
	}
}

// marshalExecutionPlan converts an execution plan into the dry-run YAML view.
func marshalExecutionPlan(plan *models.ExecutionPlan) (string, error) {
	result := ""
	for _, stage := range plan.Stages {
		result += stage.Name + ":\n"
		for _, job := range stage.Jobs {
			result += fmt.Sprintf("  %s:\n", job.Name)
			result += fmt.Sprintf("    image: %s\n", job.Image)
			result += "    script:\n"
			for _, script := range job.Script {
				result += fmt.Sprintf("      - %s\n", script)
			}
		}
	}
	return result, nil
}
