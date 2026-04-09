// Package planner builds execution plans from parsed pipelines by resolving
// stage ordering and intra-stage job dependencies via topological sort.
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

// BuildStagePlan builds the static dependency graph for a single stage.
func BuildStagePlan(stageName string, pipeline *models.Pipeline) models.StagePlan {
	stageJobs := jobsForStage(stageName, pipeline)
	orderedJobs := scheduleJobsInStage(stageJobs)

	plan := models.StagePlan{
		Name:       stageName,
		Jobs:       make([]models.JobExecutionPlan, 0, len(orderedJobs)),
		Needs:      make(map[string][]string, len(stageJobs)),
		Dependents: make(map[string][]string, len(stageJobs)),
		InDegree:   make(map[string]int, len(stageJobs)),
		JobByName:  make(map[string]models.JobExecutionPlan, len(stageJobs)),
	}

	for _, job := range stageJobs {
		plan.Needs[job.Name] = append([]string(nil), job.Needs...)
		plan.InDegree[job.Name] = len(job.Needs)
	}

	for _, job := range stageJobs {
		for _, need := range job.Needs {
			plan.Dependents[need] = append(plan.Dependents[need], job.Name)
		}
	}

	for _, job := range orderedJobs {
		jobPlan := models.JobExecutionPlan{
			Name:   job.Name,
			Image:  job.Image,
			Script: job.Script,
		}
		plan.Jobs = append(plan.Jobs, jobPlan)
		plan.JobByName[job.Name] = jobPlan
	}

	return plan
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

func jobsForStage(stageName string, pipeline *models.Pipeline) []models.Job {
	if pipeline == nil {
		return nil
	}

	var stageJobs []models.Job
	for _, job := range pipeline.Jobs {
		if job.Stage == stageName {
			stageJobs = append(stageJobs, job)
		}
	}
	return stageJobs
}

func buildStagePlan(stage *models.Stage, pipeline *models.Pipeline) models.StageExecutionPlan {
	orderedJobs := scheduleJobsInStage(jobsForStage(stage.Name, pipeline))
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
