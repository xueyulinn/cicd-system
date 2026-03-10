package execution

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/CS7580-SEA-SP26/e-team/internal/api"
)

type Handler struct {
	service *Service
	initErr error
}

func NewHandler() *Handler {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	svc, err := NewService(ctx)
	return &Handler{
		service: svc,
		initErr: err,
	}
}

func (h *Handler) Close() {
	if h.service != nil {
		h.service.Close()
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", h.handleHealth)
	mux.HandleFunc("/run", h.handleExecution)
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "healthy"}); err != nil {
		api.WriteJSONError(w, http.StatusInternalServerError, "failed to encode response")
	}
}

func (h *Handler) handleExecution(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
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

	w.Header().Set("Content-Type", "application/json")
	if resp.Success {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusBadRequest)
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		api.WriteJSONError(w, http.StatusInternalServerError, "failed to encode response")
	}
}
