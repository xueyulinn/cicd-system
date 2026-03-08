package gateway

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/CS7580-SEA-SP26/e-team/internal/api"
	"github.com/CS7580-SEA-SP26/e-team/internal/models"
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
	mux.HandleFunc("/run", h.handleRun)
	mux.HandleFunc("/report", h.handleReport)
}

// handleHealth returns gateway and service health status
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check validation service health
	validationResp := "unknown"
	if resp, err := h.client.checkValidationHealth(); err == nil {
		validationResp = resp
	}
	reportResp := "unknown"
	if resp, err := h.client.checkReportHealth(); err == nil {
		reportResp = resp
	}

	response := map[string]interface{}{
		"status": "healthy",
		"services": map[string]string{
			"validation": validationResp,
			"reporting":  reportResp,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// handleServices returns status of all services
func (h *Handler) handleServices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := map[string]interface{}{
		"services": map[string]string{
			"validation": h.client.validationURL,
			"execution":  h.client.executionURL,
			"reporting":  h.client.reportURL,
			"gateway":    getGatewayPublicURL(),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
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
	defer func() {
		_ = r.Body.Close() // Ignore close error as we're done with the body
	}()

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
					errorMsg = strings.TrimSuffix(errorMsg, "\"}")
				}
			}
		}
		if err := json.NewEncoder(w).Encode(map[string]string{"error": errorMsg}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
		return
	}

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
	defer func() {
		_ = r.Body.Close() // Ignore close error as we're done with the body
	}()

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
					errorMsg = strings.TrimSuffix(errorMsg, "\"}")
				}
			}
		}
		if err := json.NewEncoder(w).Encode(map[string]string{"error": errorMsg}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
		return
	}

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

// handleRun forwards run requests to execution service
func (h *Handler) handleRun(w http.ResponseWriter, r *http.Request) {
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
	var req api.RunRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Forward to execution service
	response, err := h.client.RunRequest(req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		// Extract error message if it contains JSON, otherwise use as-is
		errorMsg := err.Error()
		if strings.Contains(errorMsg, "{") {
			// Try to extract just the error message from JSON
			if strings.Contains(errorMsg, "execution service returned status") {
				// Extract clean error message
				start := strings.LastIndex(errorMsg, ": \"")
				if start != -1 {
					errorMsg = errorMsg[start+3:]
					errorMsg = strings.TrimSuffix(errorMsg, "\"}")
				}
			}
		}
		if err := json.NewEncoder(w).Encode(map[string]string{"error": errorMsg}); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
		return
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	if response.Success {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusBadRequest)
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// handleReport forwards report requests to reporting service.
func (h *Handler) handleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := models.ReportQuery{
		Pipeline: strings.TrimSpace(r.URL.Query().Get("pipeline")),
		Stage:    strings.TrimSpace(r.URL.Query().Get("stage")),
		Job:      strings.TrimSpace(r.URL.Query().Get("job")),
	}
	if runParam := strings.TrimSpace(r.URL.Query().Get("run")); runParam != "" {
		runNo, err := strconv.Atoi(runParam)
		if err != nil {
			writeGatewayError(w, http.StatusBadRequest, "run must be an integer")
			return
		}
		query.Run = &runNo
	}

	response, statusCode, err := h.client.ReportRequest(query)
	if err != nil {
		writeGatewayError(w, statusCode, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// checkValidationHealth checks validation service health
func (c *Client) checkValidationHealth() (string, error) {
	resp, err := c.httpClient.Get(c.validationURL + "/health")
	if err != nil {
		return "unhealthy", err
	}
	defer func() {
		_ = resp.Body.Close() // Ignore close error as we're done with the body
	}()

	if resp.StatusCode == http.StatusOK {
		return "healthy", nil
	}

	return "unhealthy", nil
}

func (c *Client) checkReportHealth() (string, error) {
	resp, err := c.httpClient.Get(c.reportURL + "/health")
	if err != nil {
		return "unhealthy", err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusOK {
		return "healthy", nil
	}
	return "unhealthy", nil
}

func writeGatewayError(w http.ResponseWriter, statusCode int, message string) {
	if statusCode == 0 {
		statusCode = http.StatusBadGateway
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": message}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func getGatewayPublicURL() string {
	url := strings.TrimSpace(os.Getenv("GATEWAY_PUBLIC_URL"))
	if url == "" {
		return "http://localhost:8000"
	}
	return strings.TrimRight(url, "/")
}
