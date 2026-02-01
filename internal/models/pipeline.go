package models

// Pipeline represents the entire CI/CD pipeline configuration
type Pipeline struct {
	Stages []Stage `yaml:"stages"`
}

// Stage represents a stage in the pipeline
type Stage struct {
	Name string `yaml:"name"`
	Jobs []Job  `yaml:"jobs"`
}

// Job represents a job within a stage
type Job struct {
	Name   string   `yaml:"name"`
	Image  string   `yaml:"image,omitempty"`
	Script []string `yaml:"script,omitempty"`
	Needs  []string `yaml:"needs,omitempty"`
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
	return e.Message
}
