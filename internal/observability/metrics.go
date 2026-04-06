package observability

import (
	"net/http"
	"time"

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
		Help:    "HTTP request latency (inbound, server handler).",
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

// RegisterMetrics registers all CI/CD and HTTP metrics with the default registry.
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
	)
}

// MetricsHandler returns an http.Handler that serves Prometheus metrics.
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// HTTPMetricsMiddleware wraps an http.Handler and records request count + latency.
func HTTPMetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, code: http.StatusOK}
		next.ServeHTTP(rec, r)
		elapsed := time.Since(start).Seconds()

		path := normalizePath(r.URL.Path)
		code := http.StatusText(rec.code)
		httpRequestsTotal.WithLabelValues(r.Method, path, code).Inc()
		httpRequestDuration.WithLabelValues(r.Method, path).Observe(elapsed)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	code int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.code = code
	r.ResponseWriter.WriteHeader(code)
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
