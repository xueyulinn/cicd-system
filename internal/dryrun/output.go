package dryrun

import (
	"fmt"

	"github.com/CS7580-SEA-SP26/e-team/internal/models"
	"github.com/CS7580-SEA-SP26/e-team/internal/scheduler"
	"gopkg.in/yaml.v3"
)

// BuildOutputStruct constructs a DryRunOutput from a validated pipeline.
// It groups jobs by stage and produces a nested structure suitable for YAML marshaling.
func BuildDryRunOutput(pipeline *models.Pipeline) models.DryRunOutput {
	dryRunOutput := models.DryRunOutput{}
	for _, stage := range pipeline.Stages {
		buildStageOutput(&stage, pipeline, dryRunOutput)
	}
	return dryRunOutput
}

// buildStageOutput populates dryRunOutput for a single stage by filtering jobs
// that belong to that stage, ordering them by dependencies (Needs), then adding
// them to the output structure.
func buildStageOutput(stage *models.Stage, pipeline *models.Pipeline, dryRunOutput models.DryRunOutput) {
	var stageJobs []models.Job
	for _, job := range pipeline.Jobs {
		if job.Stage == stage.Name {
			stageJobs = append(stageJobs, job)
		}
	}

	orderedJobs := scheduler.ScheduleJobs(stageJobs)
	jobs := make([]models.NamedJobOutput, 0, len(orderedJobs))
	for _, job := range orderedJobs {
		jobs = append(jobs, models.NamedJobOutput{
			Name: job.Name,
			JobOutput: models.JobOutput{
				Image:  job.Image,
				Script: job.Script,
			},
		})
	}
	dryRunOutput[stage.Name] = jobs
}

// MarshalOutputStruct marshals dryRunOutput to YAML with stages in declaration order.
func MarshalOutputStruct(output models.DryRunOutput, stages []models.Stage) ([]byte, error) {
	root := &yaml.Node{Kind: yaml.MappingNode}
	for _, stage := range stages {
		jobs, ok := output[stage.Name]
		if !ok || len(jobs) == 0 {
			return nil, fmt.Errorf("stage '%s' has no jobs assigned to it", stage.Name)
		}
		stageKey := &yaml.Node{Kind: yaml.ScalarNode, Value: stage.Name}
		stageVal := &yaml.Node{Kind: yaml.MappingNode}
		for _, nj := range jobs {
			jobKey := &yaml.Node{Kind: yaml.ScalarNode, Value: nj.Name}
			jobVal := &yaml.Node{Kind: yaml.MappingNode}
			jobVal.Content = append(jobVal.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: "image"},
				&yaml.Node{Kind: yaml.ScalarNode, Value: nj.Image},
				&yaml.Node{Kind: yaml.ScalarNode, Value: "script"},
				scriptToNode(nj.Script),
			)
			stageVal.Content = append(stageVal.Content, jobKey, jobVal)
		}
		root.Content = append(root.Content, stageKey, stageVal)
	}
	return yaml.Marshal(root)
}

// scriptToNode converts a slice of strings to a YAML sequence node.
func scriptToNode(script []string) *yaml.Node {
	node := &yaml.Node{Kind: yaml.SequenceNode}
	for _, s := range script {
		node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: s})
	}
	return node
}
