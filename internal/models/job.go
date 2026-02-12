package models

// Job represents a job with stage reference
type Job struct {
	Name   string   `yaml:"name"`
	Stage  string   `yaml:"stage"` // Reference to stage name
	Image  string   `yaml:"image,omitempty"`
	Script []string `yaml:"script,omitempty"`
	Needs  []string `yaml:"needs,omitempty"`
}
