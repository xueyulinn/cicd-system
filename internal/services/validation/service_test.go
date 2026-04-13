package validation

import (
	"strings"
	"testing"
)

func TestValidateYAMLValidPipeline(t *testing.T) {
	svc := NewService()

	resp := svc.ValidateYAML(`
pipeline:
  name: "Demo"
stages:
  - build
compile:
  - stage: build
  - image: golang:1.25
  - script:
    - go test ./...
`)
	if !resp.Valid {
		t.Fatalf("ValidateYAML valid = false, errors = %+v", resp.Errors)
	}
}

func TestValidateYAMLInvalidPipeline(t *testing.T) {
	svc := NewService()

	resp := svc.ValidateYAML(`
pipeline:
  name: "Demo"
stages:
  - build
  - test
compile:
  - stage: build
  - needs: [missing-job]
  - image: golang:1.25
  - script:
    - go test ./...
`)
	if resp.Valid {
		t.Fatal("expected invalid response")
	}
	if len(resp.Errors) == 0 {
		t.Fatal("expected validation errors")
	}
}

func TestDryRunYAMLValidPipeline(t *testing.T) {
	svc := NewService()

	resp := svc.DryRunYAML(`
pipeline:
  name: "Demo"
stages:
  - build
compile:
  - stage: build
  - image: golang:1.25
  - script:
    - go test ./...
`)
	if !resp.Valid {
		t.Fatalf("DryRunYAML valid = false, errors = %+v", resp.Errors)
	}
	if !strings.Contains(resp.Output, "build:") || !strings.Contains(resp.Output, "compile:") {
		t.Fatalf("unexpected dryrun output: %q", resp.Output)
	}
}
