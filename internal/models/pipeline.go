package models

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

// Job represents a job with stage reference
type Job struct {
	Name   string   `yaml:"name"`
	Stage  string   `yaml:"stage"` // Reference to stage name
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
