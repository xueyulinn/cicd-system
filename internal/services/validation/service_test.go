package validation

import (
	"context"
	"testing"

	"github.com/xueyulinn/cicd-system/internal/api"
)

func TestValidateYAMLValidPipeline(t *testing.T) {
	disableValidationCache(t)

	svc, err := NewService()
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	resp := svc.ValidateYAML(context.Background(), newValidateRequest(validPipelineYAML))
	if !resp.Valid {
		t.Fatalf("ValidateYAML valid = false, errors = %+v", resp.Errors)
	}
}

func TestValidateYAMLInvalidPipeline(t *testing.T) {
	disableValidationCache(t)

	svc, err := NewService()
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	resp := svc.ValidateYAML(context.Background(), newValidateRequest(invalidPipelineYAML))
	if resp.Valid {
		t.Fatal("expected invalid response")
	}
	if len(resp.Errors) == 0 {
		t.Fatal("expected validation errors")
	}
}

func TestDryRunYAMLValidPipeline(t *testing.T) {
	disableValidationCache(t)

	svc, err := NewService()
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	resp := svc.DryRunYAML(context.Background(), newValidateRequest(validPipelineYAML))
	if !resp.Valid {
		t.Fatalf("DryRunYAML valid = false, errors = %+v", resp.Errors)
	}
}

func TestDryRunYAMLInvalidPipeline(t *testing.T) {
	disableValidationCache(t)

	svc, err := NewService()
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	resp := svc.DryRunYAML(context.Background(), newValidateRequest(invalidPipelineYAML))
	if resp.Valid {
		t.Fatal("expected invalid response")
	}
	if len(resp.Errors) == 0 {
		t.Fatal("expected validation errors")
	}
}

func TestValidateYAMLParserError(t *testing.T) {
	disableValidationCache(t)

	svc, err := NewService()
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	resp := svc.ValidateYAML(context.Background(), newValidateRequest(":::"))
	if resp.Valid {
		t.Fatal("expected invalid response for malformed YAML")
	}
	if len(resp.Errors) == 0 {
		t.Fatal("expected parser errors")
	}
}

func newValidateRequest(yamlContent string) *api.ValidateRequest {
	return &api.ValidateRequest{
		YAMLContent:  yamlContent,
		Commit:       "abc123",
		PipelinePath: "build.yaml",
	}
}
