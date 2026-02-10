package models

// DryRunOutput represents the dry-run execution order (stage -> ordered jobs).
type DryRunOutput map[string][]NamedJobOutput

// NamedJobOutput holds a job name and its output details; slice order preserves execution order.
type NamedJobOutput struct {
	Name string
	JobOutput
}

// JobOutput holds the image and script for a job in the dry-run output.
type JobOutput struct {
	Image  string   `yaml:"image,omitempty"`
	Script []string `yaml:"script,omitempty"`
}