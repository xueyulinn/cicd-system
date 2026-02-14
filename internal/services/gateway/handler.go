package gateway

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// Handler handles HTTP requests for API gateway
type Handler struct {
	client *Client
}

// NewHandler creates a new gateway handler
func NewHandler() *Handler {
	return &Handler{
		client: NewClient(),
	}
}

// RegisterRoutes registers gateway routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", h.handleHealth)
	mux.HandleFunc("/services", h.handleServices)
	mux.HandleFunc("/validate", h.handleValidate)
	mux.HandleFunc("/dryrun", h.handleDryRun)
}

// handleHealth returns gateway and service health status
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check validation service health
	validationResp, err := h.client.checkValidationHealth()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "unhealthy",
			"error":  err.Error(),
		})
		return
	}

	response := map[string]interface{}{
		"status": "healthy",
		"services": map[string]string{
			"validation": validationResp,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleServices returns status of all services
func (h *Handler) handleServices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := map[string]interface{}{
		"services": map[string]string{
			"validation": "http://localhost:8001",
			"gateway":   "http://localhost:8000",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleValidate forwards validation requests to validation service
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
	defer r.Body.Close()

	// Parse request
	var req map[string]string
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	yamlContent, ok := req["yaml_content"]
	if !ok {
		http.Error(w, "Missing yaml_content field", http.StatusBadRequest)
		return
	}

	// Forward to validation service
	response, err := h.client.ValidateRequest(yamlContent)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		// Extract the error message if it contains JSON, otherwise use as-is
		errorMsg := err.Error()
		if strings.Contains(errorMsg, "{") {
			// Try to extract just the error message from JSON
			if strings.Contains(errorMsg, "validation service returned status") {
				// Extract the clean error message
				start := strings.LastIndex(errorMsg, ": \"")
				if start != -1 {
					errorMsg = errorMsg[start+3:]
					if strings.HasSuffix(errorMsg, "\"}") {
						errorMsg = errorMsg[:len(errorMsg)-2]
					}
				}
			}
		}
		json.NewEncoder(w).Encode(map[string]string{"error": errorMsg})
		return
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	if response.Valid {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusBadRequest)
	}
	json.NewEncoder(w).Encode(response)
}

// handleDryRun forwards dry run requests to validation service
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
	defer r.Body.Close()

	// Parse request
	var req map[string]string
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	yamlContent, ok := req["yaml_content"]
	if !ok {
		http.Error(w, "Missing yaml_content field", http.StatusBadRequest)
		return
	}

	// Forward to validation service
	response, err := h.client.DryRunRequest(yamlContent)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		// Extract the error message if it contains JSON, otherwise use as-is
		errorMsg := err.Error()
		if strings.Contains(errorMsg, "{") {
			// Try to extract just the error message from JSON
			if strings.Contains(errorMsg, "validation service returned status") {
				// Extract clean error message
				start := strings.LastIndex(errorMsg, ": \"")
				if start != -1 {
					errorMsg = errorMsg[start+3:]
					if strings.HasSuffix(errorMsg, "\"}") {
						errorMsg = errorMsg[:len(errorMsg)-2]
					}
				}
			}
		}
		json.NewEncoder(w).Encode(map[string]string{"error": errorMsg})
		return
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	if response.Valid {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusBadRequest)
	}
	json.NewEncoder(w).Encode(response)
}

// checkValidationHealth checks validation service health
func (c *Client) checkValidationHealth() (string, error) {
	resp, err := c.httpClient.Get(c.validationURL + "/health")
	if err != nil {
		return "unhealthy", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return "healthy", nil
	}

	return "unhealthy", nil
}
