package verifier

import (
	"fmt"
	"strings"

	"github.com/CS7580-SEA-SP26/e-team/internals/models"
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

	// Check 1: At least 1 stage defined
	if err := v.checkAtLeastOneStage(); err != nil {
		errors = append(errors, err)
	}

	// Check 2: Stage names are unique
	if errs := v.checkUniqueStageNames(); len(errs) > 0 {
		errors = append(errors, errs...)
	}

	// Check 3: No empty stages
	if errs := v.checkNoEmptyStages(); len(errs) > 0 {
		errors = append(errors, errs...)
	}

	// Check 4: Job names are unique
	if errs := v.checkUniqueJobNames(); len(errs) > 0 {
		errors = append(errors, errs...)
	}

	// Check 5: All needs references are valid
	if errs := v.checkNeedsReferences(); len(errs) > 0 {
		errors = append(errors, errs...)
	}

	// Check 6: No cycles in dependency graph
	if err := v.checkNoCycles(); err != nil {
		errors = append(errors, err)
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

// checkNoEmptyStages verifies no stages are empty
func (v *Verifier) checkNoEmptyStages() []error {
	var errors []error

	for i, stage := range v.pipeline.Stages {
		if len(stage.Jobs) == 0 {
			loc := v.getStageNameLocation(i)
			errors = append(errors, v.formatError(loc,
				fmt.Sprintf("stage '%s' has no jobs defined", stage.Name)))
		}
	}

	return errors
}

// checkUniqueJobNames verifies job names are unique across all stages
func (v *Verifier) checkUniqueJobNames() []error {
	var errors []error
	seen := make(map[string]models.Location)

	for stageIdx, stage := range v.pipeline.Stages {
		for jobIdx, job := range stage.Jobs {
			if prevLoc, exists := seen[job.Name]; exists {
				loc := v.getJobNameLocation(stageIdx, jobIdx)
				errors = append(errors, v.formatError(loc,
					fmt.Sprintf("duplicate job name '%s' (previously defined at line %d)",
						job.Name, prevLoc.Line)))
			} else {
				seen[job.Name] = v.getJobNameLocation(stageIdx, jobIdx)
			}
		}
	}

	return errors
}

// checkNeedsReferences verifies all needs references point to valid jobs
func (v *Verifier) checkNeedsReferences() []error {
	var errors []error

	// Build set of all job names
	allJobs := make(map[string]bool)
	for _, stage := range v.pipeline.Stages {
		for _, job := range stage.Jobs {
			allJobs[job.Name] = true
		}
	}

	// Check each needs reference
	for stageIdx, stage := range v.pipeline.Stages {
		for jobIdx, job := range stage.Jobs {
			for _, need := range job.Needs {
				if !allJobs[need] {
					loc := v.getJobNeedsLocation(stageIdx, jobIdx)
					errors = append(errors, v.formatError(loc,
						fmt.Sprintf("job '%s' references undefined job '%s' in needs",
							job.Name, need)))
				}
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

	for stageIdx, stage := range v.pipeline.Stages {
		for jobIdx, job := range stage.Jobs {
			graph[job.Name] = job.Needs
			jobLocations[job.Name] = v.getJobNeedsLocation(stageIdx, jobIdx)
		}
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
	// Find the "stages" key in the root node
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

func (v *Verifier) getStageNameLocation(stageIdx int) models.Location {
	// Navigate to stages[stageIdx].name
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

func (v *Verifier) getJobNameLocation(stageIdx, jobIdx int) models.Location {
	// Navigate to stages[stageIdx].jobs[jobIdx].name
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
								if stageNode.Content[j].Value == "jobs" {
									jobsNode := stageNode.Content[j+1]
									if jobsNode.Kind == yaml.SequenceNode && jobIdx < len(jobsNode.Content) {
										jobNode := jobsNode.Content[jobIdx]
										if jobNode.Kind == yaml.MappingNode {
											for k := 0; k < len(jobNode.Content); k += 2 {
												if jobNode.Content[k].Value == "name" {
													return models.Location{
														Line:   jobNode.Content[k+1].Line,
														Column: jobNode.Content[k+1].Column,
													}
												}
											}
										}
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

func (v *Verifier) getJobNeedsLocation(stageIdx, jobIdx int) models.Location {
	// Navigate to stages[stageIdx].jobs[jobIdx].needs
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
								if stageNode.Content[j].Value == "jobs" {
									jobsNode := stageNode.Content[j+1]
									if jobsNode.Kind == yaml.SequenceNode && jobIdx < len(jobsNode.Content) {
										jobNode := jobsNode.Content[jobIdx]
										if jobNode.Kind == yaml.MappingNode {
											for k := 0; k < len(jobNode.Content); k += 2 {
												if jobNode.Content[k].Value == "needs" {
													return models.Location{
														Line:   jobNode.Content[k].Line,
														Column: jobNode.Content[k].Column,
													}
												}
											}
										}
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
