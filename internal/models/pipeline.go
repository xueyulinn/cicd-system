package models

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Pipeline represents the entire CI/CD pipeline configuration (parallel structure)
type Pipeline struct {
	Name   string  `yaml:"name,omitempty"`
	Stages []Stage `yaml:"stages"`
	Jobs   []Job   `yaml:"jobs"`
}

// Stage represents a stage definition (name only)
type Stage struct {
	Name string `yaml:"name"`
}

// UnmarshalYAML allows stage entries to be defined as scalars or mappings.
func (s *Stage) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		s.Name = value.Value
		return nil
	case yaml.MappingNode:
		type stageAlias Stage
		var alias stageAlias
		if err := value.Decode(&alias); err != nil {
			return err
		}
		*s = Stage(alias)
		return nil
	default:
		return fmt.Errorf("stage must be a string or mapping, got %s", value.Tag)
	}
}

// Location represents a position in the YAML file
type Location struct {
	Line   int
	Column int
}

// ValidationError represents an error with location information
type ValidationError struct {
	FilePath string
	Location Location
	Message  string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s:%d:%d: %s", e.FilePath, e.Location.Line, e.Location.Column, e.Message)
}
