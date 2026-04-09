// Package parser reads YAML pipeline configuration files and builds the
// in-memory Pipeline model used by the verifier and planner.
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
	content  string
}

// NewParser creates a new parser for the given file path
func NewParser(filePath string) *Parser {
	return &Parser{
		filePath: filePath,
	}
}

// NewParserFromContent creates a parser from raw YAML content.
func NewParserFromContent(content string) *Parser {
	return &Parser{
		content: content,
	}
}

// Parse reads and parses the YAML file
func (p *Parser) Parse() (*models.Pipeline, *yaml.Node, error) {
	if p.content != "" {
		return parseYAMLData([]byte(p.content))
	}

	data, err := os.ReadFile(p.filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read file: %w", err)
	}

	return parseYAMLData(data)
}

func parseYAMLData(data []byte) (*models.Pipeline, *yaml.Node, error) {
	var rootNode yaml.Node
	if err := yaml.Unmarshal(data, &rootNode); err != nil {
		return nil, nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	pipeline, err := buildPipeline(&rootNode)
	if err != nil {
		return nil, nil, err
	}

	return pipeline, &rootNode, nil
}

func buildPipeline(root *yaml.Node) (*models.Pipeline, error) {
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
			return nil, fmt.Errorf("pipeline format does not allow top-level `%s` key (line %d)", key.Value, key.Line)
		}
	}

	pipeline := &models.Pipeline{}
	pipelineNode := findMappingNode(content, "pipeline")
	if pipelineNode != nil {
		if nameNode := findMappingNode(pipelineNode, "name"); nameNode != nil {
			pipeline.Name = nameNode.Value
		}
	}

	pipeline.Stages = parseStages(findMappingNode(content, "stages"))
	if len(pipeline.Stages) == 0 {
		// Set default stages if no stages are defined
		pipeline.Stages = getDefaultStages()
	}
	pipeline.Jobs = parseJobs(content)

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
			if nameNode := findMappingNode(entry, "name"); nameNode != nil {
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
		if IsReservedTopLevelKey(keyNode.Value) {
			continue
		}
		jobs = append(jobs, parseJob(keyNode.Value, valueNode))
	}
	return jobs
}

// IsReservedTopLevelKey reports whether key is a reserved top-level YAML key
// (i.e. not a job definition). Shared by parser and verifier.
func IsReservedTopLevelKey(key string) bool {
	switch key {
	case "name", "pipeline", "stages", "jobs":
		return true
	default:
		return false
	}
}

func parseJob(jobName string, node *yaml.Node) models.Job {
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
				scriptLines = appendStringValues(scriptLines, fieldValue)
			case "needs":
				job.Needs = appendStringValues(job.Needs, fieldValue)
			case "failures":
				if fieldValue.Kind == yaml.ScalarNode && fieldValue.Tag == "!!bool" {
					job.Failures = fieldValue.Value == "true"
				}
			}
		}
	}
	job.Script = scriptLines
	return job
}

// appendStringValues appends scalar string(s) from a YAML node (scalar or sequence)
// to an existing slice.
func appendStringValues(existing []string, node *yaml.Node) []string {
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

// getDefaultStages returns the default stages when none are defined
func getDefaultStages() []models.Stage {
	return []models.Stage{
		{Name: "build"},
		{Name: "test"},
		{Name: "docs"},
	}
}

// JobNode represents a job defined in format
type JobNode struct {
	Name  string
	Key   *yaml.Node
	Value *yaml.Node
}

// GetJobNodes extracts job nodes from the YAML structure
func (p *Parser) GetJobNodes(rootNode *yaml.Node) []JobNode {
	var jobNodes []JobNode

	if rootNode.Kind == yaml.DocumentNode && len(rootNode.Content) > 0 {
		content := rootNode.Content[0]
		if content.Kind == yaml.MappingNode {
			for i := 0; i < len(content.Content); i += 2 {
				key := content.Content[i]
				value := content.Content[i+1]

				if IsReservedTopLevelKey(key.Value) {
					continue
				}

				jobNodes = append(jobNodes, JobNode{
					Name:  key.Value,
					Key:   key,
					Value: value,
				})
			}
		}
	}

	return jobNodes
}
