package planner

import (
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

// scheduleJobsInStage returns jobs in execution order within a stage (dependencies first).
// Uses Kahn's algorithm on the Needs graph.
func scheduleJobsInStage(jobs []models.Job) []models.Job {
	if len(jobs) == 0 {
		return nil
	}
	jobMap := make(map[string]models.Job)
	for _, job := range jobs {
		jobMap[job.Name] = job
	}
	dependents := make(map[string][]models.Job)
	for _, job := range jobs {
		for _, need := range job.Needs {
			dependents[need] = append(dependents[need], job)
		}
	}
	inDegree := make(map[string]int)
	for _, job := range jobs {
		inDegree[job.Name] = len(job.Needs)
	}
	var queue []string
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}
	var result []models.Job
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		result = append(result, jobMap[name])
		for _, dep := range dependents[name] {
			inDegree[dep.Name]--
			if inDegree[dep.Name] == 0 {
				queue = append(queue, dep.Name)
			}
		}
	}
	return result
}

func buildStagePlan(stage *models.Stage, pipeline *models.Pipeline) models.StageExecutionPlan {
	var stageJobs []models.Job
	for _, job := range pipeline.Jobs {
		if job.Stage == stage.Name {
			stageJobs = append(stageJobs, job)
		}
	}
	orderedJobs := scheduleJobsInStage(stageJobs)
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
