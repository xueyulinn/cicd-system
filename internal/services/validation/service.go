package validation

import (
	"fmt"

	"github.com/CS7580-SEA-SP26/e-team/internal/common/planner"
	"github.com/CS7580-SEA-SP26/e-team/internal/common/verifier"
	"github.com/CS7580-SEA-SP26/e-team/internal/models"
	"gopkg.in/yaml.v3"
)

// Service provides validation functionality
type Service struct{}

// NewService creates a new validation service
func NewService() *Service {
	return &Service{}
}

// ValidateYAML validates YAML pipeline content
func (s *Service) ValidateYAML(yamlContent string) ValidationResponse {
	// Parse YAML content
	pipeline, rootNode, err := parseYAMLContent(yamlContent)
	if err != nil {
		return ValidationResponse{
			Valid:  false,
			Errors: []string{err.Error()},
		}
	}

	// Create verifier and validate
	v := verifier.NewPipelineVerifier("content", pipeline, rootNode)
	errors := v.Verify()

	if len(errors) > 0 {
		errorStrings := make([]string, len(errors))
		for i, err := range errors {
			errorStrings[i] = err.Error()
		}
		return ValidationResponse{
			Valid:  false,
			Errors: errorStrings,
		}
	}

	return ValidationResponse{
		Valid:  true,
		Errors: []string{},
	}
}

// DryRunYAML validates YAML and returns dry run output
func (s *Service) DryRunYAML(yamlContent string) DryRunResponse {
	// Parse YAML content
	pipeline, rootNode, err := parseYAMLContent(yamlContent)
	if err != nil {
		return DryRunResponse{
			Valid:  false,
			Errors: []string{err.Error()},
		}
	}

	// Create verifier and validate
	v := verifier.NewPipelineVerifier("content", pipeline, rootNode)
	errors := v.Verify()

	if len(errors) > 0 {
		errorStrings := make([]string, len(errors))
		for i, err := range errors {
			errorStrings[i] = err.Error()
		}
		return DryRunResponse{
			Valid:  false,
			Errors: errorStrings,
		}
	}

	// Build execution plan for dry run output
	plan, err := planner.GenerateExecutionPlan(pipeline)
	if err != nil {
		return DryRunResponse{
			Valid:  false,
			Errors: []string{fmt.Sprintf("failed to generate execution plan: %v", err)},
		}
	}

	// Convert execution plan to YAML output
	output, err := marshalExecutionPlan(plan)
	if err != nil {
		return DryRunResponse{
			Valid:  false,
			Errors: []string{fmt.Sprintf("failed to marshal execution plan: %v", err)},
		}
	}

	return DryRunResponse{
		Valid:  true,
		Errors: []string{},
		Output: output,
	}
}

// parseYAMLContent parses YAML content string
func parseYAMLContent(content string) (*models.Pipeline, *yaml.Node, error) {
	data := []byte(content)
	
	var rootNode yaml.Node
	if err := yaml.Unmarshal(data, &rootNode); err != nil {
		return nil, nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Parse the YAML content directly
	pipeline, err := parsePipelineFromNode(&rootNode)
	if err != nil {
		return nil, nil, err
	}

	return pipeline, &rootNode, nil
}

// parsePipelineFromNode extracts pipeline from YAML node
func parsePipelineFromNode(root *yaml.Node) (*models.Pipeline, error) {
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return nil, fmt.Errorf("invalid YAML document")
	}

	content := root.Content[0]
	if content.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("root must be a mapping")
	}

	pipeline := &models.Pipeline{}
	
	// Extract pipeline name
	pipelineNode := findMappingNode(content, "pipeline")
	if pipelineNode != nil {
		if nameNode := findNameNode(pipelineNode); nameNode != nil {
			pipeline.Name = nameNode.Value
		}
	}

	// Extract stages
	pipeline.Stages = parseStages(findSequenceNode(content, "stages"))
	if len(pipeline.Stages) == 0 {
		// Set default stages if no stages are defined
		pipeline.Stages = getDefaultStages()
	}
	
	// Extract jobs
	pipeline.Jobs = parseJobs(content)

	return pipeline, nil
}

// Helper functions for YAML parsing
func findMappingNode(root *yaml.Node, key string) *yaml.Node {
	if root == nil || root.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(root.Content); i += 2 {
		if root.Content[i].Value == key {
			return root.Content[i+1]
		}
	}
	return nil
}

func findSequenceNode(root *yaml.Node, key string) *yaml.Node {
	return findMappingNode(root, key)
}

func findNameNode(node *yaml.Node) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == "name" {
			return node.Content[i+1]
		}
	}
	return nil
}

func parseStages(node *yaml.Node) []models.Stage {
	if node == nil || node.Kind != yaml.SequenceNode {
		return nil
	}
	var stages []models.Stage
	for _, entry := range node.Content {
		switch entry.Kind {
		case yaml.ScalarNode:
			stages = append(stages, models.Stage{Name: entry.Value})
		case yaml.MappingNode:
			if nameNode := findNameNode(entry); nameNode != nil {
				stages = append(stages, models.Stage{Name: nameNode.Value})
			}
		}
	}
	return stages
}

func parseJobs(root *yaml.Node) []models.Job {
	var jobs []models.Job
	if root == nil || root.Kind != yaml.MappingNode {
		return jobs
	}
	for i := 0; i < len(root.Content); i += 2 {
		keyNode := root.Content[i]
		valueNode := root.Content[i+1]
		if isReservedTopLevelKey(keyNode.Value) {
			continue
		}
		jobs = append(jobs, parseJob(keyNode.Value, valueNode))
	}
	return jobs
}

func isReservedTopLevelKey(key string) bool {
	return key == "pipeline" || key == "stages"
}

func parseJob(name string, node *yaml.Node) models.Job {
	job := models.Job{Name: name}
	
	if node.Kind != yaml.SequenceNode {
		return job
	}

	// Parse job properties from sequence
	for _, item := range node.Content {
		if item.Kind != yaml.MappingNode {
			continue
		}
		
		for i := 0; i < len(item.Content); i += 2 {
			key := item.Content[i]
			value := item.Content[i+1]
			
			switch key.Value {
			case "stage":
				if value.Kind == yaml.ScalarNode {
					job.Stage = value.Value
				}
			case "needs":
				if value.Kind == yaml.SequenceNode {
					for _, need := range value.Content {
						if need.Kind == yaml.ScalarNode {
							job.Needs = append(job.Needs, need.Value)
						}
					}
				}
			case "image":
				if value.Kind == yaml.ScalarNode {
					job.Image = value.Value
				}
			case "script":
				if value.Kind == yaml.SequenceNode {
					for _, script := range value.Content {
						if script.Kind == yaml.ScalarNode {
							job.Script = append(job.Script, script.Value)
						}
					}
				}
			}
		}
	}
	
	return job
}

func getDefaultStages() []models.Stage {
	return []models.Stage{
		{Name: "build"},
		{Name: "test"},
		{Name: "deploy"},
	}
}

// marshalExecutionPlan converts execution plan to YAML string
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

// ValidationResponse represents validation result
type ValidationResponse struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

// DryRunResponse represents dry run result
type DryRunResponse struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
	Output string   `json:"output,omitempty"`
}
