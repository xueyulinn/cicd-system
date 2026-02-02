package verifier

import (
	"fmt"

	"github.com/CS7580-SEA-SP26/e-team/internal/models"
	"gopkg.in/yaml.v3"
)

func (v *PipelineVerifier) populateLegacyPipelineData() {
	if v.pipeline == nil {
		return
	}

	// Only populate from legacy format if jobs are not already populated
	if len(v.pipeline.Jobs) > 0 {
		// Jobs already populated, check if stages need to be extracted
		if len(v.pipeline.Stages) == 0 {
			v.SetDefaultStages()
		}
		return
	}

	// Parse jobs from legacy format
	legacyJobs := v.getLegacyJobNodes()
	for _, legacyJob := range legacyJobs {
		job := v.parseLegacyJob(legacyJob.name, legacyJob.value)
		v.pipeline.Jobs = append(v.pipeline.Jobs, job)
	}

	// Parse stages
	stagesNode := v.getTopLevelNode("stages")
	if stagesNode != nil {
		// Use explicitly defined stages
		v.pipeline.Stages = v.parseStagesFromNode(stagesNode)
	} else {
		// Extract stages from jobs and sort alphabetically
		v.SetDefaultStages()
	}
}

func (v *PipelineVerifier) SetDefaultStages() {
	// Return fixed default stages when stages key is missing
	v.pipeline.Stages = []models.Stage{
		{Name: "build"},
		{Name: "test"},
		{Name: "docs"},
	}
}

func (v *PipelineVerifier) getTopLevelNode(key string) *yaml.Node {
	root := v.getRootMappingNode()
	if root == nil {
		return nil
	}

	for i := 0; i < len(root.Content); i += 2 {
		if root.Content[i].Value == key {
			return root.Content[i+1]
		}
	}
	return nil
}

func (v *PipelineVerifier) getRootMappingNode() *yaml.Node {
	if v.rootNode.Kind == yaml.DocumentNode && len(v.rootNode.Content) > 0 {
		content := v.rootNode.Content[0]
		if content.Kind == yaml.MappingNode {
			return content
		}
	}
	return nil
}

func (v *PipelineVerifier) findNameInMapping(node *yaml.Node) string {
	if node == nil || node.Kind != yaml.MappingNode {
		return ""
	}

	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == "name" && node.Content[i+1].Kind == yaml.ScalarNode {
			return node.Content[i+1].Value
		}
	}
	return ""
}

func (v *PipelineVerifier) parseStagesFromNode(node *yaml.Node) []models.Stage {
	if node == nil || node.Kind != yaml.SequenceNode {
		return nil
	}

	var stages []models.Stage
	for _, stageNode := range node.Content {
		if stageNode.Kind == yaml.ScalarNode {
			// Simple string stage name
			stages = append(stages, models.Stage{Name: stageNode.Value})
		} else if stageNode.Kind == yaml.MappingNode {
			// Object with name field
			name := v.findNameInMapping(stageNode)
			if name != "" {
				stages = append(stages, models.Stage{Name: name})
			}
		}
	}
	return stages
}

func (v *PipelineVerifier) getLegacyJobNodes() []legacyJobNode {
	if v.legacyJobsCached {
		return v.legacyJobNodes
	}

	var nodes []legacyJobNode
	root := v.getRootMappingNode()
	if root == nil {
		return nodes
	}

	for i := 0; i < len(root.Content); i += 2 {
		key := root.Content[i]
		value := root.Content[i+1]

		if !v.isReservedTopLevelKey(key.Value) && v.isLegacyJobCandidate(value) {
			nodes = append(nodes, legacyJobNode{
				name:  key.Value,
				key:   key,
				value: value,
			})
		}
	}

	v.legacyJobNodes = nodes
	v.legacyJobsCached = true
	return nodes
}

func (v *PipelineVerifier) isReservedTopLevelKey(key string) bool {
	switch key {
	case "name", "pipeline", "stages", "jobs":
		return true
	default:
		return false
	}
}

func (v *PipelineVerifier) isLegacyJobCandidate(node *yaml.Node) bool {
	if node.Kind != yaml.SequenceNode || len(node.Content) == 0 {
		return false
	}

	// Check if this looks like a legacy job (sequence of mappings)
	for _, item := range node.Content {
		if item.Kind != yaml.MappingNode || len(item.Content) != 2 {
			return false
		}
	}

	return true
}

func (v *PipelineVerifier) validateLegacyJobs(nodes []legacyJobNode) []error {
	var errors []error
	for _, jobNode := range nodes {
		errors = append(errors, v.validateLegacyJobStructure(jobNode.name, jobNode.key, jobNode.value)...)
	}
	return errors
}

func (v *PipelineVerifier) validateLegacyJobStructure(jobName string, keyNode, valueNode *yaml.Node) []error {
	var errors []error
	hasStage := false
	hasImage := false
	hasScript := false

	// Validate job name is a string
	if keyNode.Kind != yaml.ScalarNode || keyNode.Tag != "!!str" {
		errors = append(errors, v.formatError(
			models.Location{Line: keyNode.Line, Column: keyNode.Column},
			fmt.Sprintf("wrong type for job name. Expected string, got %s", keyNode.Tag)))
	}

	// Check that value is a sequence
	if valueNode.Kind != yaml.SequenceNode {
		errors = append(errors, v.formatError(
			models.Location{Line: keyNode.Line, Column: keyNode.Column},
			fmt.Sprintf("wrong type for job '%s'. Expected sequence, got %s", jobName, valueNode.Tag)))
		return errors
	}

	// Validate each item in the sequence
	for i, step := range valueNode.Content {
		if step.Kind != yaml.MappingNode {
			errors = append(errors, v.formatError(
				models.Location{Line: step.Line, Column: step.Column},
				fmt.Sprintf("wrong type for item %d in job '%s'. Expected mapping, got %s", i+1, jobName, step.Tag)))
			continue
		}

		// Each mapping should have exactly one key-value pair
		if len(step.Content) != 2 {
			errors = append(errors, v.formatError(
				models.Location{Line: step.Line, Column: step.Column},
				fmt.Sprintf("item %d in job '%s' should have exactly one field, got %d", i+1, jobName, len(step.Content)/2)))
			continue
		}

		fieldKey := step.Content[0].Value
		fieldValue := step.Content[1]

		switch fieldKey {
		case "stage":
			hasStage = true
			if msg := v.validateStringScalar(fieldValue, fmt.Sprintf("stage in job '%s'", jobName)); msg != "" {
				errors = append(errors, v.formatError(
					models.Location{Line: fieldValue.Line, Column: fieldValue.Column}, msg))
			}
		case "image":
			hasImage = true
			if msg := v.validateStringScalar(fieldValue, fmt.Sprintf("image in job '%s'", jobName)); msg != "" {
				errors = append(errors, v.formatError(
					models.Location{Line: fieldValue.Line, Column: fieldValue.Column}, msg))
			}
		case "needs":
			if fieldValue.Kind != yaml.SequenceNode {
				errors = append(errors, v.formatError(
					models.Location{Line: fieldValue.Line, Column: fieldValue.Column},
					fmt.Sprintf("wrong type for needs in job '%s'. Expected sequence, got %s", jobName, fieldValue.Tag)))
			}
		case "script":
			hasScript = true
			if fieldValue.Kind != yaml.ScalarNode && fieldValue.Kind != yaml.SequenceNode {
				errors = append(errors, v.formatError(
					models.Location{Line: fieldValue.Line, Column: fieldValue.Column},
					fmt.Sprintf("wrong type for script in job '%s'. Expected string or sequence, got %s", jobName, fieldValue.Tag)))
			}
		default:
			errors = append(errors, v.formatError(
				models.Location{Line: step.Line, Column: step.Column},
				fmt.Sprintf("unexpected field '%s' in job '%s'", fieldKey, jobName)))
		}
	}

	// Check for missing required fields
	jobLocation := models.Location{Line: keyNode.Line, Column: keyNode.Column}
	if !hasStage {
		errors = append(errors, v.formatError(jobLocation,
			fmt.Sprintf("job '%s' must have a `stage` field", jobName)))
	}
	if !hasImage {
		errors = append(errors, v.formatError(jobLocation,
			fmt.Sprintf("job '%s' must have an `image` field", jobName)))
	}
	if !hasScript {
		errors = append(errors, v.formatError(jobLocation,
			fmt.Sprintf("job '%s' must have a `script` field", jobName)))
	}

	return errors
}

func (v *PipelineVerifier) parseLegacyJob(jobName string, jobNode *yaml.Node) models.Job {
	job := models.Job{Name: jobName}
	var scriptLines []string

	// Parse the format where each field is a separate mapping in the sequence
	for _, step := range jobNode.Content {
		if step.Kind != yaml.MappingNode || len(step.Content) != 2 {
			continue
		}

		fieldKey := step.Content[0].Value
		fieldValue := step.Content[1]

		switch fieldKey {
		case "stage":
			if fieldValue.Kind == yaml.ScalarNode {
				job.Stage = fieldValue.Value
			}
		case "image":
			if fieldValue.Kind == yaml.ScalarNode {
				job.Image = fieldValue.Value
			}
		case "script":
			switch fieldValue.Kind {
			case yaml.ScalarNode:
				scriptLines = append(scriptLines, fieldValue.Value)
			case yaml.SequenceNode:
				for _, scriptItem := range fieldValue.Content {
					if scriptItem.Kind == yaml.ScalarNode {
						scriptLines = append(scriptLines, scriptItem.Value)
					}
				}
			}
		case "needs":
			switch fieldValue.Kind {
			case yaml.SequenceNode:
				for _, needItem := range fieldValue.Content {
					if needItem.Kind == yaml.ScalarNode {
						job.Needs = append(job.Needs, needItem.Value)
					}
				}
			case yaml.ScalarNode:
				job.Needs = append(job.Needs, fieldValue.Value)
			}
		}
	}

	job.Script = scriptLines
	return job
}

// validateStringScalar validates that a YAML node is a string scalar
func (v *PipelineVerifier) validateStringScalar(node *yaml.Node, fieldName string) string {
	if node.Kind != yaml.ScalarNode {
		return fmt.Sprintf("wrong type for %s. Expected string, got %s", fieldName, node.Tag)
	}
	return ""
}

// validateLegacyJob validates a job definition
func (v *PipelineVerifier) validateLegacyJob(keyNode, valueNode *yaml.Node, errors *[]error) {
	if valueNode.Kind != yaml.SequenceNode || len(valueNode.Content) == 0 {
		*errors = append(*errors, v.formatError(
			models.Location{Line: keyNode.Line, Column: keyNode.Column},
			fmt.Sprintf("wrong type for job '%s'. Expected sequence, got %s", keyNode.Value, valueNode.Tag)))
		return
	}

	// Merge all mappings in the sequence to form one complete job
	hasStage := false
	hasImage := false
	hasScript := false

	for i, step := range valueNode.Content {
		if step.Kind != yaml.MappingNode {
			*errors = append(*errors, v.formatError(
				models.Location{Line: step.Line, Column: step.Column},
				fmt.Sprintf("wrong type for item %d in job '%s'. Expected mapping, got %s", i+1, keyNode.Value, step.Tag)))
			continue
		}

		// Each mapping should have exactly one key-value pair
		if len(step.Content) != 2 {
			*errors = append(*errors, v.formatError(
				models.Location{Line: step.Line, Column: step.Column},
				fmt.Sprintf("item %d in job '%s' should have exactly one field, got %d", i+1, keyNode.Value, len(step.Content)/2)))
			continue
		}

		fieldKey := step.Content[0].Value
		fieldValue := step.Content[1]

		switch fieldKey {
		case "stage":
			hasStage = true
			if msg := v.validateStringScalar(fieldValue, fmt.Sprintf("stage in job '%s'", keyNode.Value)); msg != "" {
				*errors = append(*errors, v.formatError(
					models.Location{Line: fieldValue.Line, Column: fieldValue.Column}, msg))
			}
		case "image":
			hasImage = true
			if msg := v.validateStringScalar(fieldValue, fmt.Sprintf("image in job '%s'", keyNode.Value)); msg != "" {
				*errors = append(*errors, v.formatError(
					models.Location{Line: fieldValue.Line, Column: fieldValue.Column}, msg))
			}
		case "needs":
			if fieldValue.Kind != yaml.ScalarNode && fieldValue.Kind != yaml.SequenceNode {
				*errors = append(*errors, v.formatError(
					models.Location{Line: fieldValue.Line, Column: fieldValue.Column},
					fmt.Sprintf("wrong type for needs in job '%s'. Expected string or sequence, got %s", keyNode.Value, fieldValue.Tag)))
			}
		case "script":
			hasScript = true
			if fieldValue.Kind != yaml.ScalarNode && fieldValue.Kind != yaml.SequenceNode {
				*errors = append(*errors, v.formatError(
					models.Location{Line: fieldValue.Line, Column: fieldValue.Column},
					fmt.Sprintf("wrong type for script in job '%s'. Expected string or sequence, got %s", keyNode.Value, fieldValue.Tag)))
			}
		default:
			*errors = append(*errors, v.formatError(
				models.Location{Line: step.Line, Column: step.Column},
				fmt.Sprintf("unexpected field '%s' in job '%s'", fieldKey, keyNode.Value)))
		}
	}

	// Check for missing required fields for the entire job
	jobLocation := models.Location{Line: keyNode.Line, Column: keyNode.Column}
	if !hasStage {
		*errors = append(*errors, v.formatError(jobLocation,
			fmt.Sprintf("job '%s' must have a `stage` field", keyNode.Value)))
	}
	if !hasImage {
		*errors = append(*errors, v.formatError(jobLocation,
			fmt.Sprintf("job '%s' must have an `image` field", keyNode.Value)))
	}
	if !hasScript {
		*errors = append(*errors, v.formatError(jobLocation,
			fmt.Sprintf("job '%s' must have a `script` field", keyNode.Value)))
	}
}
