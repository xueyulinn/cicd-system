package validation

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/CS7580-SEA-SP26/e-team/internal/api"
	"github.com/CS7580-SEA-SP26/e-team/internal/observability"
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
	mux.HandleFunc("/ready", h.handleReady)
}

// handleHealth returns health status
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteJSONError(w, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
		return
	}

	api.WriteJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

// handleValidate validates YAML pipeline
func (h *Handler) handleValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteJSONError(w, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
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

	log := observability.WithTraceContext(r.Context(), slog.Default())

	response := h.service.ValidateYAML(req.YAMLContent)

	if response.Valid {
		log.Info("validate ok")
		api.WriteJSON(w, http.StatusOK, response)
	} else {
		log.Info("validate rejected", "errors", response.Errors)
		api.WriteJSON(w, http.StatusBadRequest, response)
	}
}

// handleDryRun validates YAML and returns execution plan
func (h *Handler) handleDryRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteJSONError(w, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
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

	log := observability.WithTraceContext(r.Context(), slog.Default())

	response := h.service.DryRunYAML(req.YAMLContent)

	if response.Valid {
		log.Info("dryrun ok")
		api.WriteJSON(w, http.StatusOK, response)
	} else {
		log.Info("dryrun rejected", "errors", response.Errors)
		api.WriteJSON(w, http.StatusBadRequest, response)
	}
}

// handleReady returns readiness status
func (h *Handler) handleReady(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteJSONError(w, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
		return
	}

	api.WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
