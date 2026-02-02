package verifier

import (
	"fmt"
	"strings"

	"github.com/CS7580-SEA-SP26/e-team/internal/models"
	"gopkg.in/yaml.v3"
)

// checkAtLeastOneStage verifies at least one stage is defined
func (v *PipelineVerifier) checkAtLeastOneStage() error {
	if len(v.pipeline.Stages) == 0 {
		loc := v.getStagesLocation()
		return v.formatError(loc, "pipeline must have at least one stage defined")
	}
	return nil
}

// checkUniqueStageNames verifies stage names are unique
func (v *PipelineVerifier) checkUniqueStageNames() []error {
	var errors []error
	seen := make(map[string]models.Location)

	for i, stage := range v.pipeline.Stages {
		if prevLoc, exists := seen[stage.Name]; exists {
			loc := v.getStageNameLocation(i)
			errors = append(errors, v.formatError(loc,
				fmt.Sprintf("duplicate stage name '%s' (previously defined at line %d)", stage.Name, prevLoc.Line)))
		} else {
			seen[stage.Name] = v.getStageNameLocation(i)
		}
	}

	return errors
}

// checkAtLeastOneJob verifies at least one job is defined
func (v *PipelineVerifier) checkAtLeastOneJob() error {
	if len(v.pipeline.Jobs) == 0 {
		loc := v.getJobsLocation()
		return v.formatError(loc, "pipeline must have at least one job defined")
	}
	return nil
}

// checkUniqueJobNames verifies job names are unique
func (v *PipelineVerifier) checkUniqueJobNames() []error {
	var errors []error
	seen := make(map[string]models.Location)

	for i, job := range v.pipeline.Jobs {
		if prevLoc, exists := seen[job.Name]; exists {
			loc := v.getJobNameLocation(i)
			errors = append(errors, v.formatError(loc,
				fmt.Sprintf("duplicate job name '%s' (previously defined at line %d)", job.Name, prevLoc.Line)))
		} else {
			seen[job.Name] = v.getJobNameLocation(i)
		}
	}

	return errors
}

// checkJobStagesExist verifies each job references a valid stage
func (v *PipelineVerifier) checkJobStagesExist() []error {
	var errors []error

	// Build set of valid stage names
	validStages := make(map[string]bool)
	for _, stage := range v.pipeline.Stages {
		validStages[stage.Name] = true
	}

	// Check each job's stage
	for i, job := range v.pipeline.Jobs {
		if !validStages[job.Stage] {
			loc := v.getJobStageLocation(i)
			errors = append(errors, v.formatError(loc,
				fmt.Sprintf("job '%s' references undefined stage '%s'", job.Name, job.Stage)))
		}
	}

	return errors
}

// checkNoEmptyStages verifies no stages are empty (have no jobs assigned)
func (v *PipelineVerifier) checkNoEmptyStages() []error {
	var errors []error

	// Count jobs per stage
	jobsPerStage := make(map[string]int)
	for _, job := range v.pipeline.Jobs {
		jobsPerStage[job.Stage]++
	}

	// Check for empty stages
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
func (v *PipelineVerifier) checkNeedsReferences() []error {
	var errors []error

	// Build set of all job names
	jobNames := make(map[string]bool)
	for _, job := range v.pipeline.Jobs {
		jobNames[job.Name] = true
	}

	// Check each job's needs
	for i, job := range v.pipeline.Jobs {
		for _, need := range job.Needs {
			if !jobNames[need] {
				loc := v.getJobNeedsLocation(i)
				errors = append(errors, v.formatError(loc,
					fmt.Sprintf("job '%s' references undefined job '%s' in needs", job.Name, need)))
			}
		}
	}

	return errors
}

// checkNoCycles detects cycles in the job dependency graph
func (v *PipelineVerifier) checkNoCycles() error {
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

// checkYAMLTypes validates that YAML fields have correct types
func (v *PipelineVerifier) checkYAMLTypes() []error {
	var errors []error

	if v.rootNode.Kind != yaml.DocumentNode || len(v.rootNode.Content) == 0 {
		errors = append(errors, v.formatError(models.Location{Line: 1, Column: 1},
			"invalid YAML document"))
		return errors
	}

	root := v.rootNode.Content[0]
	if root.Kind != yaml.MappingNode {
		errors = append(errors, v.formatError(models.Location{Line: root.Line, Column: root.Column},
			"root must be a mapping"))
		return errors
	}

	// Check for pipeline section
	pipelineNode := v.getTopLevelNode("pipeline")
	if pipelineNode == nil {
		// Find the root mapping location to report where pipeline should be
		root := v.getRootMappingNode()
		if root != nil {
			errors = append(errors, v.formatError(models.Location{Line: root.Line, Column: root.Column},
				"pipeline must have a `pipeline` key"))
		} else {
			errors = append(errors, v.formatError(models.Location{Line: 1, Column: 1},
				"pipeline must have a `pipeline` key"))
		}
		return errors
	}

	if pipelineNode.Kind != yaml.MappingNode {
		errors = append(errors, v.formatError(models.Location{Line: pipelineNode.Line, Column: pipelineNode.Column},
			"pipeline must be a mapping"))
		return errors
	}

	// Check pipeline name
	if pipelineNode.Kind == yaml.MappingNode {
		foundName := false
		for i := 0; i < len(pipelineNode.Content); i += 2 {
			if pipelineNode.Content[i].Value == "name" {
				foundName = true
				pipelineNameNode := pipelineNode.Content[i+1]
				if pipelineNameNode.Kind != yaml.ScalarNode || pipelineNameNode.Tag != "!!str" {
					errors = append(errors, v.formatError(models.Location{Line: pipelineNameNode.Line, Column: pipelineNameNode.Column},
						"wrong type for `pipeline.name`. Expected string, got "+pipelineNameNode.Tag))
				}
				break
			}
		}
		if !foundName {
			errors = append(errors, v.formatError(models.Location{Line: pipelineNode.Line, Column: pipelineNode.Column},
				"pipeline must have a `name` field"))
		}
	}

	// Check stages section
	stagesNode := v.getTopLevelNode("stages")
	if stagesNode == nil {
		// Stages are optional - will be extracted from jobs if missing
		// Don't report an error here, let the stage extraction handle it
	} else if stagesNode.Kind != yaml.SequenceNode {
		errors = append(errors, v.formatError(models.Location{Line: stagesNode.Line, Column: stagesNode.Column},
			"stages must be a sequence"))
	} else {
		// Validate each stage
		for i, stageNode := range stagesNode.Content {
			if stageNode.Kind == yaml.MappingNode {
				nameNode := v.findNameInMapping(stageNode)
				if nameNode == "" {
					loc := models.Location{Line: stageNode.Line, Column: stageNode.Column}
					errors = append(errors, v.formatError(loc,
						"stage object must have a `name` field"))
				}
			} else if stageNode.Kind != yaml.ScalarNode {
				loc := models.Location{Line: stageNode.Line, Column: stageNode.Column}
				errors = append(errors, v.formatError(loc,
					fmt.Sprintf("wrong type for stage at index %d. Expected string, got %s", i, stageNode.Tag)))
			}
		}
	}

	// Check for jobs in legacy format
	legacyJobs := v.getLegacyJobNodes()
	if len(legacyJobs) == 0 {
		// Check for structured jobs format
		jobsNode := v.getTopLevelNode("jobs")
		if jobsNode != nil {
			if jobsNode.Kind != yaml.SequenceNode {
				errors = append(errors, v.formatError(models.Location{Line: jobsNode.Line, Column: jobsNode.Column},
					"jobs must be a sequence"))
			}
		}
	} else {
		// Validate legacy jobs
		errors = append(errors, v.validateLegacyJobs(legacyJobs)...)
	}

	return errors
}

// hasCriticalErrors checks if there are errors that prevent further validation
func (v *PipelineVerifier) hasCriticalErrors(errors []error) bool {
	// Only consider it critical if stages or jobs keys are completely missing
	// or if the document structure is invalid
	for _, err := range errors {
		msg := err.Error()
		if strings.Contains(msg, "invalid YAML document") ||
			strings.Contains(msg, "root must be a mapping") ||
			strings.Contains(msg, "pipeline must have a `pipeline` key") ||
			strings.Contains(msg, "pipeline must be a mapping") ||
			strings.Contains(msg, "stages must be a sequence") {
			return true
		}
	}
	return false
}
