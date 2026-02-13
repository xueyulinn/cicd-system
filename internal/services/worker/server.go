package worker

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/moby/moby/client"
)

const defaultAddr = ":8003"

// Server is the Worker Service HTTP server.
type Server struct {
	addr   string
	docker *client.Client
	server *http.Server
}

// NewServer creates a new Worker Service server listening on addr (e.g. ":8003").
// If addr is empty, defaultAddr (":8003") is used.
// docker may be nil (e.g. in tests); job execution will fail until a client is set.
func NewServer(addr string, docker *client.Client) *Server {
	if addr == "" {
		addr = defaultAddr
	}
	s := &Server{addr: addr, docker: docker}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	s.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	return s
}

// Start starts the HTTP server. It blocks until the server is stopped.
func (s *Server) Start() error {
	return s.server.ListenAndServe()
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
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, `{"status":"ok"}`)
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

	srv := NewServer(addr, dockerCli)
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
