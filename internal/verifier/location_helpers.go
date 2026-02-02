package verifier

import (
	"github.com/CS7580-SEA-SP26/e-team/internal/models"
	"gopkg.in/yaml.v3"
)

// Helper functions to get locations from YAML nodes

func (v *PipelineVerifier) getStagesLocation() models.Location {
	if v.rootNode.Kind == yaml.DocumentNode && len(v.rootNode.Content) > 0 {
		content := v.rootNode.Content[0]
		if content.Kind == yaml.MappingNode {
			for i := 0; i < len(content.Content); i += 2 {
				if content.Content[i].Value == "stages" {
					return models.Location{
						Line:   content.Content[i].Line,
						Column: content.Content[i].Column,
					}
				}
			}
		}
	}
	return models.Location{Line: 1, Column: 1}
}

func (v *PipelineVerifier) getJobsLocation() models.Location {
	if v.rootNode.Kind == yaml.DocumentNode && len(v.rootNode.Content) > 0 {
		content := v.rootNode.Content[0]
		if content.Kind == yaml.MappingNode {
			// For format, find the first job (top-level key that's not reserved)
			for i := 0; i < len(content.Content); i += 2 {
				key := content.Content[i]
				if !v.isReservedTopLevelKey(key.Value) {
					return models.Location{
						Line:   key.Line,
						Column: key.Column,
					}
				}
			}
		}
	}
	return models.Location{Line: 1, Column: 1}
}

func (v *PipelineVerifier) getStageNameLocation(stageIdx int) models.Location {
	if v.rootNode.Kind == yaml.DocumentNode && len(v.rootNode.Content) > 0 {
		content := v.rootNode.Content[0]
		if content.Kind == yaml.MappingNode {
			for i := 0; i < len(content.Content); i += 2 {
				if content.Content[i].Value == "stages" {
					stagesNode := content.Content[i+1]
					if stagesNode.Kind == yaml.SequenceNode && stageIdx < len(stagesNode.Content) {
						stageNode := stagesNode.Content[stageIdx]
						if stageNode.Kind == yaml.MappingNode {
							// format with name field
							for j := 0; j < len(stageNode.Content); j += 2 {
								if stageNode.Content[j].Value == "name" {
									return models.Location{
										Line:   stageNode.Content[j+1].Line,
										Column: stageNode.Content[j+1].Column,
									}
								}
							}
						} else if stageNode.Kind == yaml.ScalarNode {
							// format with simple string
							return models.Location{
								Line:   stageNode.Line,
								Column: stageNode.Column,
							}
						}
					}
				}
			}
		}
	}
	return models.Location{Line: 1, Column: 1}
}

func (v *PipelineVerifier) getJobNameLocation(jobIdx int) models.Location {
	if v.rootNode.Kind == yaml.DocumentNode && len(v.rootNode.Content) > 0 {
		content := v.rootNode.Content[0]
		if content.Kind == yaml.MappingNode {
			// For format, jobs are top-level keys (excluding reserved keys)
			JobCount := 0
			for i := 0; i < len(content.Content); i += 2 {
				key := content.Content[i]

				if v.isReservedTopLevelKey(key.Value) {
					continue
				}

				if JobCount == jobIdx {
					return models.Location{
						Line:   key.Line,
						Column: key.Column,
					}
				}
				JobCount++
			}
		}
	}
	return models.Location{Line: 1, Column: 1}
}

func (v *PipelineVerifier) getJobStageLocation(jobIdx int) models.Location {
	if v.rootNode.Kind == yaml.DocumentNode && len(v.rootNode.Content) > 0 {
		content := v.rootNode.Content[0]
		if content.Kind == yaml.MappingNode {
			// For format, jobs are top-level keys
			JobCount := 0
			for i := 0; i < len(content.Content); i += 2 {
				key := content.Content[i]
				value := content.Content[i+1]

				if v.isReservedTopLevelKey(key.Value) {
					continue
				}

				if JobCount == jobIdx {
					// Find the stage field in the job's sequence
					if value.Kind == yaml.SequenceNode {
						for _, step := range value.Content {
							if step.Kind == yaml.MappingNode && len(step.Content) == 2 {
								if step.Content[0].Value == "stage" {
									return models.Location{
										Line:   step.Content[1].Line,
										Column: step.Content[1].Column,
									}
								}
							}
						}
					}
					// Fallback to job name location
					return models.Location{
						Line:   key.Line,
						Column: key.Column,
					}
				}
				JobCount++
			}
		}
	}
	return models.Location{Line: 1, Column: 1}
}

func (v *PipelineVerifier) getJobNeedsLocation(jobIdx int) models.Location {
	if v.rootNode.Kind == yaml.DocumentNode && len(v.rootNode.Content) > 0 {
		content := v.rootNode.Content[0]
		if content.Kind == yaml.MappingNode {
			// For format, jobs are top-level keys
			JobCount := 0
			for i := 0; i < len(content.Content); i += 2 {
				key := content.Content[i]
				value := content.Content[i+1]

				if v.isReservedTopLevelKey(key.Value) {
					continue
				}

				if JobCount == jobIdx {
					// Find the needs field in the job's sequence
					if value.Kind == yaml.SequenceNode {
						for _, step := range value.Content {
							if step.Kind == yaml.MappingNode && len(step.Content) == 2 {
								if step.Content[0].Value == "needs" {
									return models.Location{
										Line:   step.Content[0].Line,
										Column: step.Content[0].Column,
									}
								}
							}
						}
					}
					// Fallback to job name location
					return models.Location{
						Line:   key.Line,
						Column: key.Column,
					}
				}
				JobCount++
			}
		}
	}
	return models.Location{Line: 1, Column: 1}
}

func (v *PipelineVerifier) formatError(loc models.Location, message string) error {
	return &models.ValidationError{
		FilePath: v.filePath,
		Location: loc,
		Message:  message,
	}
}
