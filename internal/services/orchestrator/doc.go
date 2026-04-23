// Package orchestrator provides HTTP handlers and orchestration logic for pipeline execution.
//
// It validates incoming run requests, persists run/stage/job lifecycle state,
// dispatches executable jobs to worker infrastructure, and processes worker callbacks.
package orchestrator
