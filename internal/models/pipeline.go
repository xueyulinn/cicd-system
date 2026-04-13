package models

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Pipeline represents a parsed CI/CD pipeline definition.
type Pipeline struct {
	Name   string  `yaml:"name,omitempty"`
	Stages []Stage `yaml:"stages"`
	Jobs   []Job   `yaml:"jobs"`
}

// Stage represents a declared pipeline stage.
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

// Job represents a pipeline job and its execution requirements.
type Job struct {
	Name     string   `yaml:"name"`
	Stage    string   `yaml:"stage"` // Reference to stage name
	Image    string   `yaml:"image,omitempty"`
	Script   []string `yaml:"script,omitempty"`
	Needs    []string `yaml:"needs,omitempty"`
	Failures bool     `json:"failures" yaml:"failures"`
}

// Location represents a position in a YAML document.
type Location struct {
	Line   int
	Column int
}

// ValidationError represents a validation error with file and location context.
type ValidationError struct {
	FilePath string
	Location Location
	Message  string
}

// Error renders the validation error in file:line:column: message format.
func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s:%d:%d: %s", e.FilePath, e.Location.Line, e.Location.Column, e.Message)
}
