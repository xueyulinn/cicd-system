package observability

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// CI/CD pipeline metrics (required by spec).
var (
	PipelineRunsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "cicd_pipeline_runs_total",
		Help: "Total pipeline executions.",
	}, []string{"pipeline", "run_no", "status"})

	PipelineDurationSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "cicd_pipeline_duration_seconds",
		Help:    "End-to-end pipeline execution time.",
		Buckets: []float64{5, 10, 30, 60, 120, 300, 600, 1800},
	}, []string{"pipeline", "run_no"})

	StageDurationSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "cicd_stage_duration_seconds",
		Help:    "Per-stage execution time.",
		Buckets: []float64{5, 10, 30, 60, 120, 300, 600},
	}, []string{"pipeline", "run_no", "stage"})

	JobDurationSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "cicd_job_duration_seconds",
		Help:    "Per-job execution time.",
		Buckets: []float64{1, 5, 10, 30, 60, 120, 300},
	}, []string{"pipeline", "run_no", "stage", "job"})

	JobRunsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "cicd_job_runs_total",
		Help: "Total job executions.",
	}, []string{"pipeline", "run_no", "stage", "job", "status"})
)

// httpRequestsTotal and httpRequestDuration are per-service HTTP metrics.
var (
	httpRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests by method, path, and status code.",
	}, []string{"method", "path", "code"})

	httpRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Inbound HTTP request latency (inbound, server handler).",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})

	// HTTPClientRequestDurationSeconds measures outbound HTTP from this process (client perspective).
	// Labels: client = calling service name, upstream = logical peer (e.g. validation, execution).
	httpClientRequestDurationSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_client_request_duration_seconds",
		Help:    "Outbound HTTP request latency (client perspective).",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 30, 60, 120, 300, 600, 1800},
	}, []string{"client", "upstream"})

	httpClientRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_client_requests_total",
		Help: "Outbound HTTP requests by client, upstream, and HTTP status (or error).",
	}, []string{"client", "upstream", "code"})
)

// Async execution / RabbitMQ metrics (parallel-ready batches and worker queue).
var (
	cicdMQJobsPublishedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "cicd_mq_jobs_published_total",
		Help: "Job messages published to RabbitMQ (success or failure).",
	}, []string{"queue", "outcome"})

	cicdMQDeliveryOutcomesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "cicd_mq_delivery_outcomes_total",
		Help: "RabbitMQ consumer outcomes per delivery (ack, nack with requeue, or ack error).",
	}, []string{"queue", "outcome"})

	cicdExecutionReadyBatchSize = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "cicd_execution_ready_batch_size",
		Help:    "Number of jobs dispatched together in one enqueue batch (parallel-ready within a stage).",
		Buckets: []float64{1, 2, 4, 8, 16, 32, 64},
	})

	cicdExecutionJobsEnqueuedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "cicd_execution_jobs_enqueued_total",
		Help: "Jobs successfully enqueued to the worker queue after publish.",
	}, []string{"pipeline", "stage"})
)

// RegisterMetrics registers all metrics with the default registry.
// Safe to call once at startup.
func RegisterMetrics() {
	prometheus.MustRegister(
		PipelineRunsTotal,
		PipelineDurationSeconds,
		StageDurationSeconds,
		JobDurationSeconds,
		JobRunsTotal,
		httpRequestsTotal,
		httpRequestDuration,
		httpClientRequestDurationSeconds,
		httpClientRequestsTotal,
		cicdMQJobsPublishedTotal,
		cicdMQDeliveryOutcomesTotal,
		cicdExecutionReadyBatchSize,
		cicdExecutionJobsEnqueuedTotal,
	)
}

// RecordMQJobPublished records publish success or failure for a queue.
func RecordMQJobPublished(queue string, success bool) {
	outcome := "failure"
	if success {
		outcome = "success"
	}
	cicdMQJobsPublishedTotal.WithLabelValues(queue, outcome).Inc()
}

// RecordMQDeliveryOutcome records how a single delivery was handled (acked, nack+requeue, or ack error).
func RecordMQDeliveryOutcome(queue, outcome string) {
	cicdMQDeliveryOutcomesTotal.WithLabelValues(queue, outcome).Inc()
}

// RecordExecutionReadyBatchSize records how many jobs were ready in one dispatch batch.
func RecordExecutionReadyBatchSize(n int) {
	if n < 0 {
		n = 0
	}
	cicdExecutionReadyBatchSize.Observe(float64(n))
}

// RecordExecutionJobEnqueued increments after a successful publish for one job.
func RecordExecutionJobEnqueued(pipeline, stage string) {
	cicdExecutionJobsEnqueuedTotal.WithLabelValues(pipeline, stage).Inc()
}

// MetricsHandler returns an http.Handler that serves Prometheus metrics.
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// normalizePath keeps known paths and collapses the rest to avoid high cardinality.
func normalizePath(p string) string {
	switch p {
	case "/health", "/ready", "/metrics",
		"/validate", "/dryrun", "/run", "/report",
		"/execute", "/services":
		return p
	default:
		return "/other"
	}
}
