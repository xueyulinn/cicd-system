package verifier

import (
	"github.com/xueyulinn/cicd-system/internal/common/parser"
	"github.com/xueyulinn/cicd-system/internal/models"
	"gopkg.in/yaml.v3"
)

var fallbackLocation = models.Location{Line: 1, Column: 1}

func (v *PipelineVerifier) getRootMappingNode() *yaml.Node {
	if v.rootNode.Kind == yaml.DocumentNode && len(v.rootNode.Content) > 0 {
		content := v.rootNode.Content[0]
		if content.Kind == yaml.MappingNode {
			return content
		}
	}
	return nil
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

// getJobNodePair returns the YAML key and value nodes for the job at the given
// index (skipping reserved top-level keys). Returns nil, nil when not found.
func (v *PipelineVerifier) getJobNodePair(jobIdx int) (key, value *yaml.Node) {
	root := v.getRootMappingNode()
	if root == nil {
		return nil, nil
	}
	jobCount := 0
	for i := 0; i < len(root.Content); i += 2 {
		k := root.Content[i]
		if parser.IsReservedTopLevelKey(k.Value) {
			continue
		}
		if jobCount == jobIdx {
			return k, root.Content[i+1]
		}
		jobCount++
	}
	return nil, nil
}

func (v *PipelineVerifier) getStagesLocation() models.Location {
	root := v.getRootMappingNode()
	if root == nil {
		return fallbackLocation
	}
	for i := 0; i < len(root.Content); i += 2 {
		if root.Content[i].Value == "stages" {
			return models.Location{Line: root.Content[i].Line, Column: root.Content[i].Column}
		}
	}
	return fallbackLocation
}

func (v *PipelineVerifier) getJobsLocation() models.Location {
	key, _ := v.getJobNodePair(0)
	if key != nil {
		return models.Location{Line: key.Line, Column: key.Column}
	}
	return fallbackLocation
}

func (v *PipelineVerifier) getStageNameLocation(stageIdx int) models.Location {
	root := v.getRootMappingNode()
	if root == nil {
		return fallbackLocation
	}
	for i := 0; i < len(root.Content); i += 2 {
		if root.Content[i].Value == "stages" {
			stagesNode := root.Content[i+1]
			if stagesNode.Kind == yaml.SequenceNode && stageIdx < len(stagesNode.Content) {
				stageNode := stagesNode.Content[stageIdx]
				switch stageNode.Kind {
				case yaml.MappingNode:
					for j := 0; j < len(stageNode.Content); j += 2 {
						if stageNode.Content[j].Value == "name" {
							return models.Location{
								Line:   stageNode.Content[j+1].Line,
								Column: stageNode.Content[j+1].Column,
							}
						}
					}
				case yaml.ScalarNode:
					return models.Location{Line: stageNode.Line, Column: stageNode.Column}
				}
			}
		}
	}
	return fallbackLocation
}

func (v *PipelineVerifier) getJobNameLocation(jobIdx int) models.Location {
	key, _ := v.getJobNodePair(jobIdx)
	if key != nil {
		return models.Location{Line: key.Line, Column: key.Column}
	}
	return fallbackLocation
}

// findJobField searches for a named field inside a job's step sequence and
// returns the field key node. Returns nil when not found.
func findJobField(value *yaml.Node, fieldName string) *yaml.Node {
	if value == nil || value.Kind != yaml.SequenceNode {
		return nil
	}
	for _, step := range value.Content {
		if step.Kind == yaml.MappingNode && len(step.Content) == 2 {
			if step.Content[0].Value == fieldName {
				return step.Content[0]
			}
		}
	}
	return nil
}

func (v *PipelineVerifier) getJobStageLocation(jobIdx int) models.Location {
	key, value := v.getJobNodePair(jobIdx)
	if key == nil {
		return fallbackLocation
	}
	if value != nil && value.Kind == yaml.SequenceNode {
		for _, step := range value.Content {
			if step.Kind == yaml.MappingNode && len(step.Content) == 2 {
				if step.Content[0].Value == "stage" {
					return models.Location{Line: step.Content[1].Line, Column: step.Content[1].Column}
				}
			}
		}
	}
	return models.Location{Line: key.Line, Column: key.Column}
}

func (v *PipelineVerifier) getJobNeedsLocation(jobIdx int) models.Location {
	key, value := v.getJobNodePair(jobIdx)
	if key == nil {
		return fallbackLocation
	}
	if fieldKey := findJobField(value, "needs"); fieldKey != nil {
		return models.Location{Line: fieldKey.Line, Column: fieldKey.Column}
	}
	return models.Location{Line: key.Line, Column: key.Column}
}

func (v *PipelineVerifier) formatError(loc models.Location, message string) error {
	return &models.ValidationError{
		FilePath: v.filePath,
		Location: loc,
		Message:  message,
	}
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
