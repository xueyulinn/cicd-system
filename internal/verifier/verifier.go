package verifier

import (
	"fmt"
	"strings"

	"github.com/CS7580-SEA-SP26/e-team/internal/models"
	"gopkg.in/yaml.v3"
)

// Verifier validates pipeline configurations
type Verifier struct {
	filePath string
	pipeline *models.Pipeline
	rootNode *yaml.Node
}

// NewVerifier creates a new verifier
func NewVerifier(filePath string, pipeline *models.Pipeline, rootNode *yaml.Node) *Verifier {
	return &Verifier{
		filePath: filePath,
		pipeline: pipeline,
		rootNode: rootNode,
	}
}

// Verify runs all validation checks
func (v *Verifier) Verify() []error {
	var errors []error

	// Check 0: Validate YAML types and structure
	typeErrors := v.checkYAMLTypes()
	if len(typeErrors) > 0 {
		errors = append(errors, typeErrors...)
		// Only return early if there are critical parsing errors
		// that would prevent other checks from working
		if v.hasCriticalErrors(typeErrors) {
			return errors
		}
	}

	// Check 1: At least 1 stage defined
	if err := v.checkAtLeastOneStage(); err != nil {
		errors = append(errors, err)
	}

	// Check 2: Stage names are unique
	if errs := v.checkUniqueStageNames(); len(errs) > 0 {
		errors = append(errors, errs...)
	}

	// Check 3: At least one job defined
	if err := v.checkAtLeastOneJob(); err != nil {
		errors = append(errors, err)
	}

	// Check 4: Job names are unique
	if errs := v.checkUniqueJobNames(); len(errs) > 0 {
		errors = append(errors, errs...)
	}

	// Check 5: Each job references a valid stage
	if errs := v.checkJobStagesExist(); len(errs) > 0 {
		errors = append(errors, errs...)
	}

	// Check 6: No empty stages (stages with no jobs assigned)
	if errs := v.checkNoEmptyStages(); len(errs) > 0 {
		errors = append(errors, errs...)
	}

	// Check 7: All needs references are valid
	if errs := v.checkNeedsReferences(); len(errs) > 0 {
		errors = append(errors, errs...)
	}

	// Check 8: No cycles in dependency graph (includes self-dependency)
	if err := v.checkNoCycles(); err != nil {
		errors = append(errors, err)
	}

	return errors
}

// hasCriticalErrors checks if there are errors that prevent further validation
func (v *Verifier) hasCriticalErrors(errors []error) bool {
	// Only consider it critical if stages or jobs keys are completely missing
	// or if the document structure is invalid
	for _, err := range errors {
		msg := err.Error()
		if strings.Contains(msg, "invalid YAML document") ||
			strings.Contains(msg, "root must be a mapping") {
			return true
		}
	}
	return false
}

// checkYAMLTypes validates that YAML fields have correct types
func (v *Verifier) checkYAMLTypes() []error {
	var errors []error

	if v.rootNode.Kind != yaml.DocumentNode || len(v.rootNode.Content) == 0 {
		return []error{v.formatError(models.Location{Line: 1, Column: 1}, "invalid YAML document")}
	}

	content := v.rootNode.Content[0]

	// Handle empty document
	if content.Kind == yaml.ScalarNode && content.Value == "" {
		return []error{v.formatError(models.Location{Line: 1, Column: 1}, "pipeline must have a `stages` key")}
	}

	if content.Kind != yaml.MappingNode {
		return []error{v.formatError(models.Location{Line: 1, Column: 1}, "root must be a mapping")}
	}

	// Check for required top-level keys
	var stagesNode *yaml.Node
	var jobsNode *yaml.Node
	hasPipelineName := false

	for i := 0; i < len(content.Content); i += 2 {
		key := content.Content[i]
		value := content.Content[i+1]

		switch key.Value {
		case "name":
			hasPipelineName = true
			// Check if value is actually a string (not a number)
			if value.Tag == "!!int" || value.Tag == "!!float" {
				errors = append(errors, v.formatError(
					models.Location{Line: value.Line, Column: value.Column},
					fmt.Sprintf("wrong type of value given for `name` key. Expected value of type String, given %s", value.Value)))
			} else if value.Kind != yaml.ScalarNode || value.Tag != "!!str" {
				errors = append(errors, v.formatError(
					models.Location{Line: value.Line, Column: value.Column},
					"wrong type of value given for `name` key. Expected value of type String, given "+value.Value))
			} else if value.Value == "" {
				errors = append(errors, v.formatError(
					models.Location{Line: value.Line, Column: value.Column},
					"pipeline.name cannot be empty"))
			}

		case "stages":
			stagesNode = value
			if value.Kind != yaml.SequenceNode {
				errors = append(errors, v.formatError(
					models.Location{Line: value.Line, Column: value.Column},
					"wrong type for `stages` key. Expected sequence (list), got "+value.Tag))
			} else {
				// Validate each stage
				for j, stageNode := range value.Content {
					if stageNode.Kind != yaml.MappingNode {
						errors = append(errors, v.formatError(
							models.Location{Line: stageNode.Line, Column: stageNode.Column},
							fmt.Sprintf("wrong type for stage at index %d. Expected mapping, got "+stageNode.Tag, j)))
						continue
					}
					// Check stage has name field and it's a string
					hasName := false
					for k := 0; k < len(stageNode.Content); k += 2 {
						if stageNode.Content[k].Value == "name" {
							hasName = true
							nameValue := stageNode.Content[k+1]
							if nameValue.Tag == "!!int" || nameValue.Tag == "!!float" {
								errors = append(errors, v.formatError(
									models.Location{Line: nameValue.Line, Column: nameValue.Column},
									fmt.Sprintf("wrong type of value given for `name` key. Expected value of type String, given %s", nameValue.Value)))
							} else if nameValue.Kind != yaml.ScalarNode || nameValue.Tag != "!!str" {
								errors = append(errors, v.formatError(
									models.Location{Line: nameValue.Line, Column: nameValue.Column},
									fmt.Sprintf("wrong type of value given for `name` key. Expected value of type String, given %s", nameValue.Value)))
							} else if nameValue.Value == "" {
								errors = append(errors, v.formatError(
									models.Location{Line: nameValue.Line, Column: nameValue.Column},
									"stage name cannot be empty"))
							}
						}
					}
					if !hasName {
						errors = append(errors, v.formatError(
							models.Location{Line: stageNode.Line, Column: stageNode.Column},
							"stage must have a `name` field"))
					}
				}
			}

		case "jobs":
			jobsNode = value
			if value.Kind != yaml.SequenceNode {
				errors = append(errors, v.formatError(
					models.Location{Line: value.Line, Column: value.Column},
					"wrong type for `jobs` key. Expected sequence (list), got "+value.Tag))
			} else {
				// Validate each job
				for j, jobNode := range value.Content {
					if jobNode.Kind != yaml.MappingNode {
						errors = append(errors, v.formatError(
							models.Location{Line: jobNode.Line, Column: jobNode.Column},
							fmt.Sprintf("wrong type for job at index %d. Expected mapping, got "+jobNode.Tag, j)))
						continue
					}

					// Track required fields for this job
					hasName := false
					hasStage := false
					hasImage := false
					hasScript := false

					for k := 0; k < len(jobNode.Content); k += 2 {
						fieldKey := jobNode.Content[k].Value
						fieldValue := jobNode.Content[k+1]

						switch fieldKey {
						case "name":
							hasName = true
							if fieldValue.Tag == "!!int" || fieldValue.Tag == "!!float" {
								errors = append(errors, v.formatError(
									models.Location{Line: fieldValue.Line, Column: fieldValue.Column},
									fmt.Sprintf("wrong type of value given for `name` key. Expected value of type String, given %s", fieldValue.Value)))
							} else if fieldValue.Kind != yaml.ScalarNode || fieldValue.Tag != "!!str" {
								errors = append(errors, v.formatError(
									models.Location{Line: fieldValue.Line, Column: fieldValue.Column},
									fmt.Sprintf("wrong type of value given for `name` key. Expected value of type String, given %s", fieldValue.Value)))
							} else if fieldValue.Value == "" {
								errors = append(errors, v.formatError(
									models.Location{Line: fieldValue.Line, Column: fieldValue.Column},
									"job name cannot be empty"))
							}

						case "stage":
							hasStage = true
							if fieldValue.Tag == "!!int" || fieldValue.Tag == "!!float" {
								errors = append(errors, v.formatError(
									models.Location{Line: fieldValue.Line, Column: fieldValue.Column},
									fmt.Sprintf("wrong type of value given for `stage` key. Expected value of type String, given %s", fieldValue.Value)))
							} else if fieldValue.Kind != yaml.ScalarNode || fieldValue.Tag != "!!str" {
								errors = append(errors, v.formatError(
									models.Location{Line: fieldValue.Line, Column: fieldValue.Column},
									fmt.Sprintf("wrong type of value given for `stage` key. Expected value of type String, given %s", fieldValue.Value)))
							} else if fieldValue.Value == "" {
								errors = append(errors, v.formatError(
									models.Location{Line: fieldValue.Line, Column: fieldValue.Column},
									"job stage cannot be empty"))
							}

						case "image":
							hasImage = true
							if fieldValue.Tag == "!!int" || fieldValue.Tag == "!!float" {
								errors = append(errors, v.formatError(
									models.Location{Line: fieldValue.Line, Column: fieldValue.Column},
									fmt.Sprintf("wrong type of value given for `image` key. Expected value of type String, given %s", fieldValue.Value)))
							} else if fieldValue.Kind != yaml.ScalarNode || fieldValue.Tag != "!!str" {
								errors = append(errors, v.formatError(
									models.Location{Line: fieldValue.Line, Column: fieldValue.Column},
									fmt.Sprintf("wrong type of value given for `image` key. Expected value of type String, given %s", fieldValue.Value)))
							} else if fieldValue.Value == "" {
								errors = append(errors, v.formatError(
									models.Location{Line: fieldValue.Line, Column: fieldValue.Column},
									"job image cannot be empty"))
							}

						case "script":
							hasScript = true
							if fieldValue.Kind != yaml.SequenceNode {
								errors = append(errors, v.formatError(
									models.Location{Line: fieldValue.Line, Column: fieldValue.Column},
									"wrong type of value given for `script` key. Expected sequence (list), got "+fieldValue.Tag))
							} else {
								// Check script is not empty and all items are strings
								if len(fieldValue.Content) == 0 {
									errors = append(errors, v.formatError(
										models.Location{Line: fieldValue.Line, Column: fieldValue.Column},
										"script list cannot be empty"))
								}
								for _, scriptItem := range fieldValue.Content {
									if scriptItem.Kind != yaml.ScalarNode || scriptItem.Tag != "!!str" {
										errors = append(errors, v.formatError(
											models.Location{Line: scriptItem.Line, Column: scriptItem.Column},
											"script items must be strings"))
									}
								}
							}

						case "needs":
							if fieldValue.Kind != yaml.SequenceNode {
								errors = append(errors, v.formatError(
									models.Location{Line: fieldValue.Line, Column: fieldValue.Column},
									"wrong type of value given for `needs` key. Expected sequence (list), got "+fieldValue.Tag))
							} else {
								// Check each needs item is a string
								for _, needItem := range fieldValue.Content {
									if needItem.Kind != yaml.ScalarNode || needItem.Tag != "!!str" {
										errors = append(errors, v.formatError(
											models.Location{Line: needItem.Line, Column: needItem.Column},
											"needs items must be strings"))
									}
								}
							}
						}
					}

					// Only check required fields if we're doing full validation
					// Don't report missing required fields if there are already type errors
					// as the file may be incomplete for testing purposes
					jobLocation := models.Location{Line: jobNode.Line, Column: jobNode.Column}
					if !hasName {
						errors = append(errors, v.formatError(jobLocation,
							"job must have a `name` field"))
					}
					if !hasStage {
						errors = append(errors, v.formatError(jobLocation,
							"job must have a `stage` field"))
					}
					if !hasImage {
						errors = append(errors, v.formatError(jobLocation,
							"job must have an `image` field"))
					}
					if !hasScript {
						errors = append(errors, v.formatError(jobLocation,
							"job must have a `script` field"))
					}
				}
			}
		}
	}

	// Check that required top-level keys exist
	if !hasPipelineName {
		errors = append(errors, v.formatError(
			models.Location{Line: 1, Column: 1},
			"pipeline must have a `name` field"))
	}
	if stagesNode == nil {
		errors = append(errors, v.formatError(
			models.Location{Line: 1, Column: 1},
			"pipeline must have a `stages` key"))
	}
	if jobsNode == nil {
		errors = append(errors, v.formatError(
			models.Location{Line: 1, Column: 1},
			"pipeline must have a `jobs` key"))
	}

	return errors
}

// checkAtLeastOneStage verifies at least one stage is defined
func (v *Verifier) checkAtLeastOneStage() error {
	if len(v.pipeline.Stages) == 0 {
		loc := v.getStagesLocation()
		return v.formatError(loc, "pipeline must have at least one stage defined")
	}
	return nil
}

// checkUniqueStageNames verifies stage names are unique
func (v *Verifier) checkUniqueStageNames() []error {
	var errors []error
	seen := make(map[string]models.Location)

	for i, stage := range v.pipeline.Stages {
		if prevLoc, exists := seen[stage.Name]; exists {
			loc := v.getStageNameLocation(i)
			errors = append(errors, v.formatError(loc,
				fmt.Sprintf("duplicate stage name '%s' (previously defined at line %d)",
					stage.Name, prevLoc.Line)))
		} else {
			seen[stage.Name] = v.getStageNameLocation(i)
		}
	}

	return errors
}

// checkAtLeastOneJob verifies at least one job is defined
func (v *Verifier) checkAtLeastOneJob() error {
	if len(v.pipeline.Jobs) == 0 {
		loc := v.getJobsLocation()
		return v.formatError(loc, "pipeline must have at least one job defined")
	}
	return nil
}

// checkUniqueJobNames verifies job names are unique
func (v *Verifier) checkUniqueJobNames() []error {
	var errors []error
	seen := make(map[string]models.Location)

	for i, job := range v.pipeline.Jobs {
		if prevLoc, exists := seen[job.Name]; exists {
			loc := v.getJobNameLocation(i)
			errors = append(errors, v.formatError(loc,
				fmt.Sprintf("duplicate job name '%s' (previously defined at line %d)",
					job.Name, prevLoc.Line)))
		} else {
			seen[job.Name] = v.getJobNameLocation(i)
		}
	}

	return errors
}

// checkJobStagesExist verifies each job references a valid stage
func (v *Verifier) checkJobStagesExist() []error {
	var errors []error

	// Build set of valid stage names
	stageNames := make(map[string]bool)
	for _, stage := range v.pipeline.Stages {
		stageNames[stage.Name] = true
	}

	// Check each job's stage reference
	for i, job := range v.pipeline.Jobs {
		if !stageNames[job.Stage] {
			loc := v.getJobStageLocation(i)
			errors = append(errors, v.formatError(loc,
				fmt.Sprintf("job '%s' references undefined stage '%s'",
					job.Name, job.Stage)))
		}
	}

	return errors
}

// checkNoEmptyStages verifies no stages are empty (have no jobs assigned)
func (v *Verifier) checkNoEmptyStages() []error {
	var errors []error

	// Count jobs per stage
	jobsPerStage := make(map[string]int)
	for _, job := range v.pipeline.Jobs {
		jobsPerStage[job.Stage]++
	}

	// Check each stage has at least one job
	for i, stage := range v.pipeline.Stages {
		if jobsPerStage[stage.Name] == 0 {
			loc := v.getStageNameLocation(i)
			errors = append(errors, v.formatError(loc,
				fmt.Sprintf("stage '%s' has no jobs assigned to it", stage.Name)))
		}
	}

	return errors
}

// checkNeedsReferences verifies all needs references point to valid jobs
func (v *Verifier) checkNeedsReferences() []error {
	var errors []error

	// Build set of all job names
	allJobs := make(map[string]bool)
	for _, job := range v.pipeline.Jobs {
		allJobs[job.Name] = true
	}

	// Check each needs reference
	for i, job := range v.pipeline.Jobs {
		for _, need := range job.Needs {
			// Check for self-dependency
			if need == job.Name {
				loc := v.getJobNeedsLocation(i)
				errors = append(errors, v.formatError(loc,
					fmt.Sprintf("job '%s' cannot depend on itself", job.Name)))
			} else if !allJobs[need] {
				loc := v.getJobNeedsLocation(i)
				errors = append(errors, v.formatError(loc,
					fmt.Sprintf("job '%s' references undefined job '%s' in needs",
						job.Name, need)))
			}
		}
	}

	return errors
}

// checkNoCycles detects cycles in the job dependency graph
func (v *Verifier) checkNoCycles() error {
	// Build adjacency list
	graph := make(map[string][]string)
	jobLocations := make(map[string]models.Location)

	for i, job := range v.pipeline.Jobs {
		graph[job.Name] = job.Needs
		jobLocations[job.Name] = v.getJobNeedsLocation(i)
	}

	// DFS to detect cycles
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var dfs func(string, []string) ([]string, bool)
	dfs = func(job string, path []string) ([]string, bool) {
		visited[job] = true
		recStack[job] = true
		path = append(path, job)

		for _, neighbor := range graph[job] {
			if !visited[neighbor] {
				if cycle, found := dfs(neighbor, path); found {
					return cycle, true
				}
			} else if recStack[neighbor] {
				// Found a cycle, extract it
				cycleStart := 0
				for i, j := range path {
					if j == neighbor {
						cycleStart = i
						break
					}
				}
				return path[cycleStart:], true
			}
		}

		recStack[job] = false
		return nil, false
	}

	for job := range graph {
		if !visited[job] {
			if cycle, found := dfs(job, nil); found {
				// Format cycle
				cycleStr := strings.Join(cycle, " -> ")
				cycleStr += " -> " + cycle[0]

				loc := jobLocations[cycle[0]]
				return v.formatError(loc,
					fmt.Sprintf("cycle detected in `needs` requirements. %s", cycleStr))
			}
		}
	}

	return nil
}

// Helper functions to get locations from YAML nodes

func (v *Verifier) getStagesLocation() models.Location {
	if v.rootNode.Kind == yaml.DocumentNode && len(v.rootNode.Content) > 0 {
		content := v.rootNode.Content[0]
		if content.Kind == yaml.MappingNode {
			for i := 0; i < len(content.Content); i += 2 {
				if content.Content[i].Value == "stages" {
					return models.Location{
						Line:   content.Content[i].Line,
						Column: content.Content[i].Column,
					}
				}
			}
		}
	}
	return models.Location{Line: 1, Column: 1}
}

func (v *Verifier) getJobsLocation() models.Location {
	if v.rootNode.Kind == yaml.DocumentNode && len(v.rootNode.Content) > 0 {
		content := v.rootNode.Content[0]
		if content.Kind == yaml.MappingNode {
			for i := 0; i < len(content.Content); i += 2 {
				if content.Content[i].Value == "jobs" {
					return models.Location{
						Line:   content.Content[i].Line,
						Column: content.Content[i].Column,
					}
				}
			}
		}
	}
	return models.Location{Line: 1, Column: 1}
}

func (v *Verifier) getStageNameLocation(stageIdx int) models.Location {
	if v.rootNode.Kind == yaml.DocumentNode && len(v.rootNode.Content) > 0 {
		content := v.rootNode.Content[0]
		if content.Kind == yaml.MappingNode {
			for i := 0; i < len(content.Content); i += 2 {
				if content.Content[i].Value == "stages" {
					stagesNode := content.Content[i+1]
					if stagesNode.Kind == yaml.SequenceNode && stageIdx < len(stagesNode.Content) {
						stageNode := stagesNode.Content[stageIdx]
						if stageNode.Kind == yaml.MappingNode {
							for j := 0; j < len(stageNode.Content); j += 2 {
								if stageNode.Content[j].Value == "name" {
									return models.Location{
										Line:   stageNode.Content[j+1].Line,
										Column: stageNode.Content[j+1].Column,
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return models.Location{Line: 1, Column: 1}
}

func (v *Verifier) getJobNameLocation(jobIdx int) models.Location {
	if v.rootNode.Kind == yaml.DocumentNode && len(v.rootNode.Content) > 0 {
		content := v.rootNode.Content[0]
		if content.Kind == yaml.MappingNode {
			for i := 0; i < len(content.Content); i += 2 {
				if content.Content[i].Value == "jobs" {
					jobsNode := content.Content[i+1]
					if jobsNode.Kind == yaml.SequenceNode && jobIdx < len(jobsNode.Content) {
						jobNode := jobsNode.Content[jobIdx]
						if jobNode.Kind == yaml.MappingNode {
							for j := 0; j < len(jobNode.Content); j += 2 {
								if jobNode.Content[j].Value == "name" {
									return models.Location{
										Line:   jobNode.Content[j+1].Line,
										Column: jobNode.Content[j+1].Column,
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return models.Location{Line: 1, Column: 1}
}

func (v *Verifier) getJobStageLocation(jobIdx int) models.Location {
	if v.rootNode.Kind == yaml.DocumentNode && len(v.rootNode.Content) > 0 {
		content := v.rootNode.Content[0]
		if content.Kind == yaml.MappingNode {
			for i := 0; i < len(content.Content); i += 2 {
				if content.Content[i].Value == "jobs" {
					jobsNode := content.Content[i+1]
					if jobsNode.Kind == yaml.SequenceNode && jobIdx < len(jobsNode.Content) {
						jobNode := jobsNode.Content[jobIdx]
						if jobNode.Kind == yaml.MappingNode {
							for j := 0; j < len(jobNode.Content); j += 2 {
								if jobNode.Content[j].Value == "stage" {
									return models.Location{
										Line:   jobNode.Content[j+1].Line,
										Column: jobNode.Content[j+1].Column,
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return models.Location{Line: 1, Column: 1}
}

func (v *Verifier) getJobNeedsLocation(jobIdx int) models.Location {
	if v.rootNode.Kind == yaml.DocumentNode && len(v.rootNode.Content) > 0 {
		content := v.rootNode.Content[0]
		if content.Kind == yaml.MappingNode {
			for i := 0; i < len(content.Content); i += 2 {
				if content.Content[i].Value == "jobs" {
					jobsNode := content.Content[i+1]
					if jobsNode.Kind == yaml.SequenceNode && jobIdx < len(jobsNode.Content) {
						jobNode := jobsNode.Content[jobIdx]
						if jobNode.Kind == yaml.MappingNode {
							for j := 0; j < len(jobNode.Content); j += 2 {
								if jobNode.Content[j].Value == "needs" {
									return models.Location{
										Line:   jobNode.Content[j].Line,
										Column: jobNode.Content[j].Column,
									}
								}
							}
							// If no needs field, return job name location
							for j := 0; j < len(jobNode.Content); j += 2 {
								if jobNode.Content[j].Value == "name" {
									return models.Location{
										Line:   jobNode.Content[j].Line,
										Column: jobNode.Content[j].Column,
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return models.Location{Line: 1, Column: 1}
}

func (v *Verifier) formatError(loc models.Location, message string) error {
	return &models.ValidationError{
		FilePath: v.filePath,
		Location: loc,
		Message:  fmt.Sprintf("%s:%d:%d: %s", v.filePath, loc.Line, loc.Column, message),
	}
}
