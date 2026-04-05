package gateway

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/CS7580-SEA-SP26/e-team/internal/api"
	"github.com/CS7580-SEA-SP26/e-team/internal/config"
	"github.com/CS7580-SEA-SP26/e-team/internal/models"
	"github.com/CS7580-SEA-SP26/e-team/internal/observability"
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
	mux.HandleFunc("/ready", h.handleReady)
}

// handleHealth reports gateway liveness only.
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteJSONError(w, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
		return
	}

	response := map[string]interface{}{
		"status": "healthy",
	}

	api.WriteJSON(w, http.StatusOK, response)
}

// handleServices returns status of all services
func (h *Handler) handleServices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteJSONError(w, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
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

	api.WriteJSON(w, http.StatusOK, response)
}

// handleValidate forwards validation requests to validation service
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

	var req map[string]string
	if err := json.Unmarshal(body, &req); err != nil {
		api.WriteJSONError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	yamlContent, ok := req["yaml_content"]
	if !ok {
		api.WriteJSONError(w, http.StatusBadRequest, "missing yaml_content field")
		return
	}

	log := observability.WithTraceContext(r.Context(), slog.Default())

	response, err := h.client.ValidateRequest(yamlContent)
	if err != nil {
		log.Warn("validate proxy failed", "error", err)
		api.WriteJSONError(w, http.StatusBadGateway, err.Error())
		return
	}

	if response.Valid {
		log.Info("validate ok")
		api.WriteJSON(w, http.StatusOK, response)
	} else {
		log.Info("validate rejected", "errors", response.Errors)
		api.WriteJSON(w, http.StatusBadRequest, response)
	}
}

// handleDryRun forwards dry run requests to validation service
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

	var req map[string]string
	if err := json.Unmarshal(body, &req); err != nil {
		api.WriteJSONError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	yamlContent, ok := req["yaml_content"]
	if !ok {
		api.WriteJSONError(w, http.StatusBadRequest, "missing yaml_content field")
		return
	}

	log := observability.WithTraceContext(r.Context(), slog.Default())

	response, err := h.client.DryRunRequest(yamlContent)
	if err != nil {
		log.Warn("dryrun proxy failed", "error", err)
		api.WriteJSONError(w, http.StatusBadGateway, err.Error())
		return
	}

	if response.Valid {
		log.Info("dryrun ok")
		api.WriteJSON(w, http.StatusOK, response)
	} else {
		log.Info("dryrun rejected", "errors", response.Errors)
		api.WriteJSON(w, http.StatusBadRequest, response)
	}
}

// handleRun forwards run requests to execution service
func (h *Handler) handleRun(w http.ResponseWriter, r *http.Request) {
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

	var req api.RunRequest
	if err := json.Unmarshal(body, &req); err != nil {
		api.WriteJSONError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	log := observability.WithTraceContext(r.Context(), slog.Default())

	response, err := h.client.RunRequest(req)
	if err != nil {
		log.Error("run proxy failed", "error", err)
		api.WriteJSONError(w, http.StatusBadGateway, err.Error())
		return
	}

	if strings.EqualFold(response.Status, "failed") {
		log.Warn("run failed", "errors", response.Errors)
		api.WriteJSON(w, http.StatusBadRequest, response)
	} else {
		log.Info("run completed", "message", response.Message, "status", response.Status)
		api.WriteJSON(w, http.StatusOK, response)
	}
}

// handleReport forwards report requests to reporting service.
func (h *Handler) handleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteJSONError(w, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
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
			api.WriteJSONError(w, http.StatusBadRequest, "run must be an integer: "+err.Error())
			return
		}
		query.Run = &runNo
	}

	log := observability.WithTraceContext(r.Context(), slog.Default()).With(
		"pipeline", query.Pipeline,
	)

	response, statusCode, err := h.client.ReportRequest(query)
	if err != nil {
		if statusCode == 0 {
			statusCode = http.StatusBadGateway
		}
		log.Warn("report proxy failed", "error", err, "status", statusCode)
		api.WriteJSONError(w, statusCode, err.Error())
		return
	}

	log.Info("report ok")
	api.WriteJSON(w, http.StatusOK, response)
}

func (h *Handler) handleReady(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteJSONError(w, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
		return
	}

	services := map[string]string{
		"validation": "unknown",
		"reporting":  "unknown",
		"execution":  "unknown",
	}

	if resp, err := h.client.checkValidationReady(); err == nil {
		services["validation"] = resp
	}
	if resp, err := h.client.checkReportReady(); err == nil {
		services["reporting"] = resp
	}
	if resp, err := h.client.checkExecutionReady(); err == nil {
		services["execution"] = resp
	}

	overallStatus := "ready"
	statusCode := http.StatusOK
	for _, status := range services {
		if status != "ready" {
			overallStatus = "not ready"
			statusCode = http.StatusServiceUnavailable
			break
		}
	}

	response := map[string]interface{}{
		"status":   overallStatus,
		"services": services,
	}

	api.WriteJSON(w, statusCode, response)
}

func (c *Client) checkValidationReady() (string, error) {
	resp, err := c.httpClient.Get(c.validationURL + "/ready")
	if err != nil {
		return "not ready", err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusOK {
		return "ready", nil
	}

	return "not ready", nil
}

func (c *Client) checkReportReady() (string, error) {
	resp, err := c.httpClient.Get(c.reportURL + "/ready")
	if err != nil {
		return "not ready", err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusOK {
		return "ready", nil
	}
	return "not ready", nil
}

func (c *Client) checkExecutionReady() (string, error) {
	resp, err := c.httpClient.Get(c.executionURL + "/ready")
	if err != nil {
		return "not ready", err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusOK {
		return "ready", nil
	}
	return "not ready", nil
}

func getGatewayPublicURL() string {
	url := strings.TrimSpace(os.Getenv("GATEWAY_PUBLIC_URL"))
	if url == "" {
		return config.DefaultGatewayURL
	}
	return strings.TrimRight(url, "/")
}
