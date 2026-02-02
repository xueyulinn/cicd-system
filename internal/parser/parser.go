package parser

import (
	"fmt"
	"os"

	"github.com/CS7580-SEA-SP26/e-team/internal/models"
	"gopkg.in/yaml.v3"
)

// Parser handles parsing of YAML pipeline configuration files
type Parser struct {
	filePath string
}

// NewParser creates a new parser for the given file path
func NewParser(filePath string) *Parser {
	return &Parser{
		filePath: filePath,
	}
}

// Parse reads and parses the YAML file
func (p *Parser) Parse() (*models.Pipeline, *yaml.Node, error) {
	data, err := os.ReadFile(p.filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read file: %w", err)
	}

	var rootNode yaml.Node
	if err := yaml.Unmarshal(data, &rootNode); err != nil {
		return nil, nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	pipeline, err := p.buildLegacyPipeline(&rootNode)
	if err != nil {
		return nil, nil, err
	}

	return pipeline, &rootNode, nil
}

func (p *Parser) buildLegacyPipeline(root *yaml.Node) (*models.Pipeline, error) {
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return nil, fmt.Errorf("invalid YAML document")
	}

	content := root.Content[0]
	if content.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("root must be a mapping")
	}

	for i := 0; i < len(content.Content); i += 2 {
		key := content.Content[i]
		if key.Value == "name" || key.Value == "jobs" {
			return nil, fmt.Errorf("legacy pipeline format does not allow top-level `%s` key (line %d)", key.Value, key.Line)
		}
	}

	pipeline := &models.Pipeline{}
	pipelineNode := findMappingNode(content, "pipeline")
	if pipelineNode != nil {
		if nameNode := findNameNode(pipelineNode); nameNode != nil {
			pipeline.Name = nameNode.Value
		}
	}

	pipeline.Stages = parseLegacyStages(findSequenceNode(content, "stages"))
	pipeline.Jobs = parseLegacyJobs(content)

	return pipeline, nil
}

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

func parseLegacyStages(node *yaml.Node) []models.Stage {
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

func parseLegacyJobs(root *yaml.Node) []models.Job {
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
		jobs = append(jobs, parseLegacyJob(keyNode.Value, valueNode))
	}
	return jobs
}

func isReservedTopLevelKey(key string) bool {
	switch key {
	case "pipeline", "stages":
		return true
	default:
		return false
	}
}

func parseLegacyJob(jobName string, node *yaml.Node) models.Job {
	job := models.Job{Name: jobName}
	var scriptLines []string
	if node == nil || node.Kind != yaml.SequenceNode {
		return job
	}
	for _, entry := range node.Content {
		if entry.Kind != yaml.MappingNode {
			continue
		}
		for j := 0; j < len(entry.Content); j += 2 {
			fieldKey := entry.Content[j].Value
			fieldValue := entry.Content[j+1]
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
				scriptLines = appendScript(scriptLines, fieldValue)
			case "needs":
				job.Needs = appendNeeds(job.Needs, fieldValue)
			}
		}
	}
	job.Script = scriptLines
	return job
}

func appendScript(existing []string, node *yaml.Node) []string {
	switch node.Kind {
	case yaml.ScalarNode:
		return append(existing, node.Value)
	case yaml.SequenceNode:
		for _, item := range node.Content {
			if item.Kind == yaml.ScalarNode {
				existing = append(existing, item.Value)
			}
		}
	}
	return existing
}

func appendNeeds(existing []string, node *yaml.Node) []string {
	switch node.Kind {
	case yaml.ScalarNode:
		return append(existing, node.Value)
	case yaml.SequenceNode:
		for _, item := range node.Content {
			if item.Kind == yaml.ScalarNode {
				existing = append(existing, item.Value)
			}
		}
	}
	return existing
}

// GetFilePath returns the file path being parsed
func (p *Parser) GetFilePath() string {
	return p.filePath
}
