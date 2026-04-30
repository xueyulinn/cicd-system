package validation

import (
	"context"
	"testing"
)

func TestValidateYAMLValidPipeline(t *testing.T) {
	disableValidationCache(t)

	svc, err := NewService()
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	resp := svc.ValidateYAML(context.Background(), validPipelineYAML)
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

	resp := svc.ValidateYAML(context.Background(), invalidPipelineYAML)
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

	resp := svc.DryRunYAML(context.Background(), validPipelineYAML)
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

	resp := svc.DryRunYAML(context.Background(), invalidPipelineYAML)
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

	resp := svc.ValidateYAML(context.Background(), ":::")
	if resp.Valid {
		t.Fatal("expected invalid response for malformed YAML")
	}
	if len(resp.Errors) == 0 {
		t.Fatal("expected parser errors")
	}
}
