package validation

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/CS7580-SEA-SP26/e-team/internal/api"
)

// Handler handles HTTP requests for validation service
type Handler struct {
	service *Service
}

// NewHandler creates a new validation handler
func NewHandler() *Handler {
	return &Handler{
		service: NewService(),
	}
}

// RegisterRoutes registers validation service routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", h.handleHealth)
	mux.HandleFunc("/validate", h.handleValidate)
	mux.HandleFunc("/dryrun", h.handleDryRun)
}

// handleHealth returns health status
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

// handleValidate validates YAML pipeline
func (h *Handler) handleValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		api.WriteJSONError(w, http.StatusBadRequest, "failed to read request body: "+err.Error())
		return
	}
	defer func() { _ = r.Body.Close() }()

	var req api.ValidateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		api.WriteJSONError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	response := h.service.ValidateYAML(req.YAMLContent)

	w.Header().Set("Content-Type", "application/json")
	if response.Valid {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusBadRequest)
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		api.WriteJSONError(w, http.StatusInternalServerError, "failed to encode response")
	}
}

// handleDryRun validates YAML and returns execution plan
func (h *Handler) handleDryRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		api.WriteJSONError(w, http.StatusBadRequest, "failed to read request body: "+err.Error())
		return
	}
	defer func() { _ = r.Body.Close() }()

	var req api.ValidateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		api.WriteJSONError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	response := h.service.DryRunYAML(req.YAMLContent)

	w.Header().Set("Content-Type", "application/json")
	if response.Valid {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusBadRequest)
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		api.WriteJSONError(w, http.StatusInternalServerError, "failed to encode response")
	}
}
