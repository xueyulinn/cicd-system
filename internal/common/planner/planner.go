package planner

import (
	"github.com/CS7580-SEA-SP26/e-team/internal/common/scheduler"
	"github.com/CS7580-SEA-SP26/e-team/internal/models"
)

// GenerateExecutionPlan builds an ExecutionPlan from a pipeline (stage order + job dependency resolution).
// Reusable by CLI and Execution Service.
func GenerateExecutionPlan(pipeline *models.Pipeline) (*models.ExecutionPlan, error) {
	plan := &models.ExecutionPlan{
		Stages: make([]models.StageExecutionPlan, 0, len(pipeline.Stages)),
	}
	for _, stage := range pipeline.Stages {
		stagePlan := buildStagePlan(&stage, pipeline)
		plan.Stages = append(plan.Stages, stagePlan)
	}
	return plan, nil
}

func buildStagePlan(stage *models.Stage, pipeline *models.Pipeline) models.StageExecutionPlan {
	var stageJobs []models.Job
	for _, job := range pipeline.Jobs {
		if job.Stage == stage.Name {
			stageJobs = append(stageJobs, job)
		}
	}
	orderedJobs := scheduler.ScheduleJobs(stageJobs)
	jobs := make([]models.JobExecutionPlan, 0, len(orderedJobs))
	for _, job := range orderedJobs {
		jobs = append(jobs, models.JobExecutionPlan{
			Name:   job.Name,
			Image:  job.Image,
			Script: job.Script,
		})
	}
	return models.StageExecutionPlan{
		Name: stage.Name,
		Jobs: jobs,
	}
}
