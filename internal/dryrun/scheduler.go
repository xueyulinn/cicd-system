package dryrun

import (

	"github.com/CS7580-SEA-SP26/e-team/internal/models"
)

// ScheduleJobs returns jobs in execution order (dependencies first).
// Uses BFS / Kahn's algorithm on the Needs graph.
func ScheduleJobs(jobs []models.Job) []models.Job {
	if len(jobs) == 0 {
		return nil
	}

	// Build the job map
	jobMap := make(map[string]models.Job)
	for _, job := range jobs {
		jobMap[job.Name] = job
	}
	
	// Build the dependents map
	dependents := make(map[string][]models.Job)
	for _, job := range jobs {
		for _, need := range job.Needs {
			dependents[need] = append(dependents[need], job)
		}
	}

	// Build the in-degree map
	inDegree := make(map[string]int)
	for _, job := range jobs {
		inDegree[job.Name] = len(job.Needs)
	}

	// Initialize the queue with jobs that have no dependencies
	var queue []string
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}

	// Process the jobs in the queue
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