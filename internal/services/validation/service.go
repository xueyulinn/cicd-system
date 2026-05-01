package validation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/xueyulinn/cicd-system/internal/api"
	"github.com/xueyulinn/cicd-system/internal/cache"
	"github.com/xueyulinn/cicd-system/internal/common/parser"
	"github.com/xueyulinn/cicd-system/internal/common/planner"
	"github.com/xueyulinn/cicd-system/internal/common/verifier"
	"github.com/xueyulinn/cicd-system/internal/models"
)

// Service provides validation functionality
type Service struct {
	cacheStore cache.Store
	cacheCfg   cache.Config
}

// NewService creates a new validation service
func NewService() (*Service, error) {
	cfg := cache.LoadConfig()
	store, err := cache.NewStoreFromConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("initialize cache store: %w", err)
	}

	return &Service{
		cacheStore: store,
		cacheCfg:   cfg,
	}, nil
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
func (s *Service) ValidateYAML(ctx context.Context, req *api.ValidateRequest) api.ValidateResponse {
	key := cache.ValidateKey(s.cacheCfg.KeyPrefix, req.PipelinePath, req.Commit)

	val, err := s.cacheStore.Get(ctx, key)
	if err == nil {
		var response api.ValidateResponse
		unmarshalErr := json.Unmarshal(val, &response)
		if unmarshalErr == nil {
			return response
		}
		slog.Warn("validation cache payload invalid; fallback to compute", "error", unmarshalErr)
	}
	if err != nil && !errors.Is(err, cache.ErrCacheMiss) {
		slog.Warn("validation cache get failed; fallback to compute", "error", err)
	}

	_, errs := validatePipelineContent(req.YAMLContent)
	response := api.ValidateResponse{
		Valid:  len(errs) == 0,
		Errors: errs,
	}

	payload, marshalErr := json.Marshal(response)
	if marshalErr != nil {
		slog.Warn("validation cache marshal failed; skip cache write", "error", marshalErr)
		return response
	}

	if setErr := s.cacheStore.Set(ctx, key, payload, s.cacheCfg.ValidateTTL); setErr != nil {
		slog.Warn("validation cache set failed; response still served", "error", setErr)
	}

	return response
}

// DryRunYAML validates YAML and returns dry run output.
func (s *Service) DryRunYAML(ctx context.Context, req *api.ValidateRequest) api.DryRunResponse {
	key := cache.DryRunKey(s.cacheCfg.KeyPrefix, req.PipelinePath, req.Commit)

	val, err := s.cacheStore.Get(ctx, key)
	if err == nil {
		var response api.DryRunResponse
		unmarshalErr := json.Unmarshal(val, &response)
		if unmarshalErr == nil {
			return response
		}
		slog.Warn("dryrun cache payload invalid; fallback to compute", "error", unmarshalErr)
	}
	if err != nil && !errors.Is(err, cache.ErrCacheMiss) {
		slog.Warn("dryrun cache get failed; fallback to compute", "error", err)
	}

	pipeline, errors := validatePipelineContent(req.YAMLContent)
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

	response := api.DryRunResponse{
		Valid:         true,
		Errors:        []string{},
		ExecutionPlan: plan,
	}

	payload, marshalErr := json.Marshal(response)
	if marshalErr != nil {
		slog.Warn("dryrun cache marshal failed; skip cache write", "error", marshalErr)
		return response
	}

	if setErr := s.cacheStore.Set(ctx, key, payload, s.cacheCfg.DryRunTTL); setErr != nil {
		slog.Warn("dryrun cache set failed; response still served", "error", setErr)
	}

	return response
}