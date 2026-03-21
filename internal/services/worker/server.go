package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/CS7580-SEA-SP26/e-team/internal/api"
	"github.com/CS7580-SEA-SP26/e-team/internal/config"
	"github.com/CS7580-SEA-SP26/e-team/internal/models"
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
	jobTimeout time.Duration // max duration for each ExecuteJob; 0 means defaultJobTimeout
}

// NewServer creates a new Worker Service server listening on addr (e.g. ":"+config.DefaultWorkerPort).
// If addr is empty, ":"+config.DefaultWorkerPort is used.
// jobTimeout is the max duration for a single job execution; if 0, defaultJobTimeout (5m) is used.
// docker may be nil (e.g. in tests); job execution will fail until a client is set.
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
	s.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	return s
}

// handleReady reports whether the worker can reach the Docker daemon.
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

// ServeListener runs the server on the given listener. Used for tests (e.g. E2E with a random port).
func (s *Server) ServeListener(l net.Listener) error {
	return s.server.Serve(l)
}

// Shutdown gracefully shuts down the server with the given context.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// Handler returns the HTTP handler for use in tests (e.g. httptest with ServeHTTP).
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

// executeRequest is the JSON body for /execute (job fields + optional workspace_path).
type executeRequest struct {
	models.JobExecutionPlan
	WorkspacePath string `json:"workspace_path,omitempty"`
}

// handleExecute runs a single job from a JSON body (JobExecutionPlan) and returns logs or error.
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
		log.Printf("[execute] invalid JSON: %v", err)
		api.WriteJSONError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	job := &models.JobExecutionPlan{Name: req.Name, Image: req.Image, Script: req.Script}

	jobName := job.Name
	if jobName == "" {
		jobName = "unnamed"
	}

	timeout := s.jobTimeout
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	start := time.Now()
	logs, err := ExecuteJob(ctx, s.docker, job, req.WorkspacePath)
	duration := time.Since(start)

	if err != nil {
		log.Printf("[execute] job=%s duration=%v error=%v", jobName, duration, err)
		api.WriteJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	log.Printf("[execute] job=%s duration=%v ok", jobName, duration)

	api.WriteJSON(w, http.StatusOK, map[string]string{"logs": logs})
}

// Run runs the Worker Service until ctx is cancelled or the server errors.
// It creates a Docker client from env (DOCKER_HOST etc.) and closes it on shutdown.
// On shutdown it gives the server up to 5 seconds to drain.
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
