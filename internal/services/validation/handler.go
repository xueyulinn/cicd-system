package validation

import (
	"encoding/json"
	"io"
	"net/http"
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
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "healthy"}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// handleValidate validates YAML pipeline
func (h *Handler) handleValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer func() {
		_ = r.Body.Close() // Ignore close error as we're done with the body
	}()

	// Parse request
	var req ValidationRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate YAML
	response := h.service.ValidateYAML(req.YAMLContent)

	// Send response
	w.Header().Set("Content-Type", "application/json")
	if response.Valid {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusBadRequest)
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// handleDryRun validates YAML and returns execution plan
func (h *Handler) handleDryRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer func() {
		_ = r.Body.Close() // Ignore close error as we're done with the body
	}()

	// Parse request
	var req DryRunRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Process dry run
	response := h.service.DryRunYAML(req.YAMLContent)

	// Send response
	w.Header().Set("Content-Type", "application/json")
	if response.Valid {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusBadRequest)
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// ValidationRequest represents validation request
type ValidationRequest struct {
	YAMLContent string `json:"yaml_content"`
}

// DryRunRequest represents dry run request
type DryRunRequest struct {
	YAMLContent string `json:"yaml_content"`
}
