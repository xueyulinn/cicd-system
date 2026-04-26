package worker

import (
	"context"
	"net/http"
	"time"

	"github.com/xueyulinn/cicd-system/internal/api"
)

// Handler exposes HTTP endpoints and lifecycle hooks for the worker service.
type Handler struct {
	service *Service
}

// NewHandler constructs a handler; dependencies are initialized lazily in Run.
func NewHandler() *Handler {
	srv := NewService(0)

	return &Handler{
		service: srv,
	}
}

// Close releases resources held by the underlying worker service.
func (h *Handler) Close() {
	if h == nil {
		return
	}
	if h.service != nil {
		_ = h.service.Close()
	}
}

// RegisterRoutes registers liveness and readiness endpoints on mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", h.handleHealth)
	mux.HandleFunc("/ready", h.handleReady)
}

// Run starts job consumption until ctx is canceled or startup fails.
func (h *Handler) Run(ctx context.Context) error {
	if h == nil {
		return nil
	}
	if h.service == nil {
		return nil
	}
	return h.service.Start(ctx)
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteJSONError(w, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
		return
	}

	api.WriteJSON(w, http.StatusOK, api.StatusResponse{Status: "healthy"})
}

func (h *Handler) handleReady(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteJSONError(w, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.service.Ready(ctx); err != nil {
		api.WriteJSONError(w, http.StatusServiceUnavailable, "worker service not ready: "+err.Error())
		return
	}

	api.WriteJSON(w, http.StatusOK, api.StatusResponse{Status: "ready"})
}
