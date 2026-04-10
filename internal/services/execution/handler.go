package execution

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/CS7580-SEA-SP26/e-team/internal/api"
)

// Handler exposes HTTP endpoints for execution service operations.
type Handler struct {
	service *Service
	initErr error
}

// NewHandler creates a Handler and initializes its underlying Service.
func NewHandler() *Handler {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	svc, err := NewService(ctx)
	return &Handler{
		service: svc,
		initErr: err,
	}
}

// Close releases resources held by the underlying execution service.
func (h *Handler) Close() {
	if h.service != nil {
		h.service.Close()
	}
}

// RegisterRoutes registers execution service HTTP routes on mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", h.handleHealth)
	mux.HandleFunc("/run", h.handleExecution)
	mux.HandleFunc("/ready", h.handleReady)
	mux.HandleFunc("/callbacks/job-started", h.handleJobStarted)
	mux.HandleFunc("/callbacks/job-finished", h.handleJobFinished)
}

func (h *Handler) handleReady(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteJSONError(w, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
		return
	}

	if h.initErr != nil {
		api.WriteJSONError(w, http.StatusServiceUnavailable, "execution service not ready: "+h.initErr.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.service.Ready(ctx); err != nil {
		api.WriteJSONError(w, http.StatusServiceUnavailable, "execution service not ready: "+err.Error())
		return
	}

	api.WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteJSONError(w, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
		return
	}

	api.WriteJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

func (h *Handler) handleExecution(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteJSONError(w, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
		return
	}

	if h.initErr != nil {
		api.WriteJSONError(w, http.StatusServiceUnavailable, "execution service not ready: "+h.initErr.Error())
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		api.WriteJSONError(w, http.StatusBadRequest, "failed to read request body: "+err.Error())
		return
	}
	defer func() { _ = r.Body.Close() }()

	var req api.RunRequest
	if err := json.Unmarshal(body, &req); err != nil {
		api.WriteJSONError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	resp, err := h.service.Run(r.Context(), req)
	if err != nil {
		api.WriteJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if len(resp.Errors) == 0 {
		api.WriteJSON(w, http.StatusOK, resp)
	} else {
		api.WriteJSON(w, http.StatusBadRequest, resp)
	}
}

// Callback for worker service to update job
func (h *Handler) handleJobStarted(w http.ResponseWriter, r *http.Request) {
	h.handleJobCallback(w, r, h.service.HandleJobStarted)
}

func (h *Handler) handleJobFinished(w http.ResponseWriter, r *http.Request) {
	h.handleJobCallback(w, r, h.service.HandleJobFinished)
}

func (h *Handler) handleJobCallback(w http.ResponseWriter, r *http.Request, fn func(context.Context, api.JobStatusCallbackRequest) error) {
	if r.Method != http.MethodPost {
		api.WriteJSONError(w, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
		return
	}

	if h.initErr != nil {
		api.WriteJSONError(w, http.StatusServiceUnavailable, "execution service not ready: "+h.initErr.Error())
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		api.WriteJSONError(w, http.StatusBadRequest, "failed to read request body: "+err.Error())
		return
	}
	defer func() { _ = r.Body.Close() }()

	var req api.JobStatusCallbackRequest
	if err := json.Unmarshal(body, &req); err != nil {
		api.WriteJSONError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if err := fn(r.Context(), req); err != nil {
		api.WriteJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	api.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
