package parser

import (
	"fmt"
	"os"

	"github.com/CS7580-SEA-SP26/e-team/internals/models"
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

	// Parse into yaml.Node to preserve location information
	var rootNode yaml.Node
	if err := yaml.Unmarshal(data, &rootNode); err != nil {
		return nil, nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Parse into Pipeline struct
	var pipeline models.Pipeline
	if err := yaml.Unmarshal(data, &pipeline); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal pipeline: %w", err)
	}

	return &pipeline, &rootNode, nil
}

// GetFilePath returns the file path being parsed
func (p *Parser) GetFilePath() string {
	return p.filePath
}
