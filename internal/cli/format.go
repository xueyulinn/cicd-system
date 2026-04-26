package cli

import (
	"encoding/json"
	"fmt"

	"github.com/xueyulinn/cicd-system/internal/models"
	"gopkg.in/yaml.v3"
)

// FormatExecutionPlanYAML formats an execution plan as YAML (stage -> job -> image/script).
// Preserves stage and job order for CLI display.
func FormatExecutionPlanYAML(plan *models.ExecutionPlan) ([]byte, error) {
	if plan == nil {
		return nil, fmt.Errorf("plan is nil")
	}
	root := &yaml.Node{Kind: yaml.MappingNode}
	for _, stage := range plan.Stages {
		stageKey := &yaml.Node{Kind: yaml.ScalarNode, Value: stage.Name}
		stageVal := &yaml.Node{Kind: yaml.MappingNode}
		for _, job := range stage.Jobs {
			jobKey := &yaml.Node{Kind: yaml.ScalarNode, Value: job.Name}
			jobVal := &yaml.Node{Kind: yaml.MappingNode}
			jobVal.Content = append(jobVal.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: "image"},
				&yaml.Node{Kind: yaml.ScalarNode, Value: job.Image},
				&yaml.Node{Kind: yaml.ScalarNode, Value: "script"},
				scriptToYAMLNode(job.Script),
			)
			stageVal.Content = append(stageVal.Content, jobKey, jobVal)
		}
		root.Content = append(root.Content, stageKey, stageVal)
	}
	return yaml.Marshal(root)
}

// FormatExecutionPlanJSON formats an execution plan as indented JSON.
func FormatExecutionPlanJSON(plan *models.ExecutionPlan) ([]byte, error) {
	if plan == nil {
		return nil, fmt.Errorf("plan is nil")
	}
	return json.MarshalIndent(plan, "", "  ")
}

func scriptToYAMLNode(script []string) *yaml.Node {
	node := &yaml.Node{Kind: yaml.SequenceNode}
	for _, s := range script {
		node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: s})
	}
	return node
}

// FormatReportYAML formats report output as YAML.
func FormatReportYAML(report *models.ReportResponse) ([]byte, error) {
	if report == nil {
		return nil, fmt.Errorf("report is nil")
	}
	return yaml.Marshal(report)
}

// FormatReportJSON formats report output as indented JSON.
func FormatReportJSON(report *models.ReportResponse) ([]byte, error) {
	if report == nil {
		return nil, fmt.Errorf("report is nil")
	}
	return json.MarshalIndent(report, "", "  ")
}
