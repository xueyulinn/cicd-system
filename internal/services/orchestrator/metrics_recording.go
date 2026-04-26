package orchestrator

// func recordPipelineOutcome(pipeline string, runNo int, status string, start time.Time) {
// 	if start.IsZero() {
// 		return
// 	}
// 	runNoLabel := strconv.Itoa(runNo)
// 	observability.PipelineRunsTotal.WithLabelValues(pipeline, runNoLabel, status).Inc()
// 	observability.PipelineDurationSeconds.WithLabelValues(pipeline, runNoLabel).Observe(time.Since(start).Seconds())
// }

// func recordStageDuration(pipeline string, runNo int, stage string, start time.Time) {
// 	if start.IsZero() {
// 		return
// 	}
// 	observability.StageDurationSeconds.WithLabelValues(pipeline, strconv.Itoa(runNo), stage).Observe(time.Since(start).Seconds())
// }

// func recordJobOutcome(pipeline string, runNo int, stage, job, status string, runStart time.Time) {
// 	runNoLabel := strconv.Itoa(runNo)
// 	observability.JobRunsTotal.WithLabelValues(pipeline, runNoLabel, stage, job, status).Inc()
// 	if !runStart.IsZero() {
// 		observability.JobDurationSeconds.WithLabelValues(pipeline, runNoLabel, stage, job).Observe(time.Since(runStart).Seconds())
// 	}
// }
