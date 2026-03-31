package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/CS7580-SEA-SP26/e-team/internal/api"
	"github.com/CS7580-SEA-SP26/e-team/internal/config"
	"github.com/CS7580-SEA-SP26/e-team/internal/models"
	"github.com/CS7580-SEA-SP26/e-team/internal/observability"
	"github.com/moby/moby/client"
)

const (
	defaultJobTimeout = 5 * time.Minute
)

// Server is the Worker Service HTTP server.
type Server struct {
	addr       string
	docker     *client.Client
	server     *http.Server
	jobTimeout time.Duration
}

// NewServer creates a new Worker Service server listening on addr.
func NewServer(addr string, docker *client.Client, jobTimeout time.Duration) *Server {
	if addr == "" {
		addr = ":" + config.DefaultWorkerPort
	}
	if jobTimeout == 0 {
		jobTimeout = defaultJobTimeout
	}
	s := &Server{addr: addr, docker: docker, jobTimeout: jobTimeout}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/execute", s.handleExecute)
	mux.HandleFunc("/ready", s.handleReady)
	mux.Handle("/metrics", observability.MetricsHandler())

	wrapped := observability.HTTPMetricsMiddleware(
		observability.TracingMiddleware("worker-service", mux))

	s.server = &http.Server{
		Addr:    addr,
		Handler: wrapped,
	}
	return s
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteJSONError(w, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := pingDocker(ctx, s.docker); err != nil {
		api.WriteJSONError(w, http.StatusServiceUnavailable, http.StatusText(http.StatusServiceUnavailable))
		return
	}

	api.WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// Start starts the HTTP server. It blocks until the server is stopped.
func (s *Server) Start() error {
	return s.server.ListenAndServe()
}

// ServeListener runs the server on the given listener (tests).
func (s *Server) ServeListener(l net.Listener) error {
	return s.server.Serve(l)
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// Handler returns the HTTP handler for use in tests.
func (s *Server) Handler() http.Handler {
	return s.server.Handler
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteJSONError(w, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
		return
	}
	api.WriteJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

// executeRequest is the JSON body for /execute.
type executeRequest struct {
	models.JobExecutionPlan
	RepoURL       string `json:"repo_url,omitempty"`
	Commit        string `json:"commit,omitempty"`
	WorkspacePath string `json:"workspace_path,omitempty"`
	Pipeline      string `json:"pipeline,omitempty"`
	RunNo         int    `json:"run_no,omitempty"`
	Stage         string `json:"stage,omitempty"`
}

func (s *Server) handleExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteJSONError(w, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
		return
	}
	if s.docker == nil {
		api.WriteJSONError(w, http.StatusServiceUnavailable, "docker client not available")
		return
	}

	var req executeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Warn("invalid execute request", "error", err)
		api.WriteJSONError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	job := &models.JobExecutionPlan{Name: req.Name, Image: req.Image, Script: req.Script}

	jobName := job.Name
	if jobName == "" {
		jobName = "unnamed"
	}

	// --- Job span (child of incoming trace from execution service) ---
	tracer := observability.Tracer("worker")
	ctx, jobSpan := tracer.Start(r.Context(), "job.run",
		trace.WithAttributes(
			attribute.String("pipeline", req.Pipeline),
			attribute.Int("run_no", req.RunNo),
			attribute.String("stage", req.Stage),
			attribute.String("job", jobName),
		),
	)
	defer jobSpan.End()

	log := observability.WithTraceContext(ctx, slog.Default()).With(
		"pipeline", req.Pipeline,
		"run_no", req.RunNo,
		"stage", req.Stage,
		"job", jobName,
	)

	timeout := s.jobTimeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	runNoLabel := strconv.Itoa(req.RunNo)
	log.Info("job started",
		"event", "job-run",
		"status", "running",
		"source", "service",
		"image", job.Image,
	)
	start := time.Now()
	logs, err := ExecuteJob(ctx, s.docker, job, req.RepoURL, req.Commit, req.WorkspacePath)
	elapsed := time.Since(start).Seconds()

	if err != nil {
		jobSpan.SetStatus(codes.Error, err.Error())
		observability.JobRunsTotal.WithLabelValues(req.Pipeline, runNoLabel, req.Stage, jobName, "failed").Inc()
		observability.JobDurationSeconds.WithLabelValues(req.Pipeline, runNoLabel, req.Stage, jobName).Observe(elapsed)
		log.Error("job failed",
			"event", "job-run",
			"status", "failed",
			"source", "service",
			"duration_s", elapsed,
			"error", err,
		)

		emitJobContainerLogs(log, logs)

		api.WriteJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jobSpan.SetStatus(codes.Ok, "")
	observability.JobRunsTotal.WithLabelValues(req.Pipeline, runNoLabel, req.Stage, jobName, "success").Inc()
	observability.JobDurationSeconds.WithLabelValues(req.Pipeline, runNoLabel, req.Stage, jobName).Observe(elapsed)
	log.Info("job completed",
		"event", "job-run",
		"status", "success",
		"source", "service",
		"duration_s", elapsed,
	)

	emitJobContainerLogs(log, logs)

	api.WriteJSON(w, http.StatusOK, map[string]string{"logs": logs})
}

// emitJobContainerLogs writes container stdout/stderr as structured log lines
// labeled with the parent logger's pipeline/run_no/stage/job context.
func emitJobContainerLogs(log *slog.Logger, rawLogs string) {
	if strings.TrimSpace(rawLogs) == "" {
		return
	}
	for _, line := range strings.Split(rawLogs, "\n") {
		if line == "" {
			continue
		}
		log.Info(line, "source", "job-container")
	}
}

// Run runs the Worker Service until ctx is cancelled.
func Run(ctx context.Context, addr string) error {
	dockerCli, err := NewDockerClient(ctx)
	if err != nil {
		return fmt.Errorf("docker client: %w", err)
	}
	defer func() { _ = dockerCli.Close() }()

	srv := NewServer(addr, dockerCli, 0)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	if err := srv.Start(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
