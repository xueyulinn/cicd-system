package dryrun

import (
	"github.com/CS7580-SEA-SP26/e-team/internal/models"
	"gopkg.in/yaml.v3"
)

// DryRunOutput represents the dry-run execution order (stage -> job -> details).
type DryRunOutput map[string]map[string]JobOutput

// JobOutput holds the image and script for a job in the dry-run output.
type JobOutput struct {
	Image  string   `yaml:"image,omitempty"`
	Script []string `yaml:"script,omitempty"`
}

// BuildOutputStruct constructs a DryRunOutput from a validated pipeline.
// It groups jobs by stage and produces a nested structure suitable for YAML marshaling.
func BuildDryRunOutput(pipeline *models.Pipeline) DryRunOutput {
	dryRunOutput := DryRunOutput{}
	for _, stage := range pipeline.Stages {
		buildStageOutput(&stage, pipeline, dryRunOutput)
	}
	return dryRunOutput
}

// buildStageOutput populates dryRunOutput for a single stage by filtering jobs
// that belong to that stage, ordering them by dependencies (Needs), then adding
// them to the output structure.
func buildStageOutput(stage *models.Stage, pipeline *models.Pipeline, dryRunOutput DryRunOutput) {
	var stageJobs []models.Job
	for _, job := range pipeline.Jobs {
		if job.Stage == stage.Name {
			stageJobs = append(stageJobs, job)
		}
	}

	orderedJobs := ScheduleJobs(stageJobs)
	dryRunOutput[stage.Name] = make(map[string]JobOutput)
	for _, job := range orderedJobs {
		buildJobOutput(&job, dryRunOutput)
	}
}
// buildJobOutput adds a single job's image and script to the dry-run output
// under its stage and job name.
func buildJobOutput(job *models.Job, dryRunOutput DryRunOutput) {
	dryRunOutput[job.Stage][job.Name] = JobOutput{
		Image: job.Image,
		Script: job.Script,
	}
}

// MarshalOutputStruct marshals dryRunOutput to YAML with stages in declaration order.
func MarshalOutputStruct(output DryRunOutput, stages []models.Stage) ([]byte, error) {
	root := &yaml.Node{Kind: yaml.MappingNode}
	// Add the stages to the root node
	for _, stage := range stages {
		jobs, ok := output[stage.Name]
		if !ok || len(jobs) == 0 {
			root.Content = append(root.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: stage.Name},
				&yaml.Node{Kind: yaml.MappingNode},
			)
			continue
		}
		// Add the jobs to the stage node
		stageKey := &yaml.Node{Kind: yaml.ScalarNode, Value: stage.Name}
		stageVal := &yaml.Node{Kind: yaml.MappingNode}
		for jobName, jobOut := range jobs {
			jobKey := &yaml.Node{Kind: yaml.ScalarNode, Value: jobName}
			jobVal := &yaml.Node{Kind: yaml.MappingNode}
			jobVal.Content = append(jobVal.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: "image"},
				&yaml.Node{Kind: yaml.ScalarNode, Value: jobOut.Image},
				&yaml.Node{Kind: yaml.ScalarNode, Value: "script"},
				scriptToNode(jobOut.Script),
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
