package gateway

import (
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/xueyulinn/cicd-system/internal/api"
	"github.com/xueyulinn/cicd-system/internal/config"
	"github.com/xueyulinn/cicd-system/internal/models"
	"github.com/xueyulinn/cicd-system/internal/observability"
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
		api.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", http.StatusText(http.StatusMethodNotAllowed))
		return
	}

	api.WriteJSON(w, http.StatusOK, api.StatusResponse{Status: "healthy"})
}

// handleServices returns status of all services
func (h *Handler) handleServices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", http.StatusText(http.StatusMethodNotAllowed))
		return
	}

	response := map[string]interface{}{
		"services": map[string]string{
			"validation":   h.client.validationURL,
			"orchestrator": h.client.orchestratorURL,
			"reporting":    h.client.reportURL,
			"gateway":      getGatewayPublicURL(),
		},
	}

	api.WriteJSON(w, http.StatusOK, response)
}

func (h *Handler) handleValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", http.StatusText(http.StatusMethodNotAllowed))
		return
	}
	defer func() { _ = r.Body.Close() }()

	log := observability.WithTraceContext(r.Context(), slog.Default())

	if err := h.client.forwardValidate(r.Context(), w, r); err != nil {
		log.Warn("gateway forward validate failed", "error", err)
		api.WriteError(w, http.StatusBadGateway, "upstream_unavailable", err.Error())
		return
	}
}

func (h *Handler) handleDryRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", http.StatusText(http.StatusMethodNotAllowed))
		return
	}
	defer func() { _ = r.Body.Close() }()

	log := observability.WithTraceContext(r.Context(), slog.Default())

	if err := h.client.forwardDryRun(r.Context(), w, r); err != nil {
		log.Warn("gateway forward dryrun failed", "error", err)
		api.WriteError(w, http.StatusBadGateway, "upstream_unavailable", err.Error())
		return
	}
}

func (h *Handler) handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", http.StatusText(http.StatusMethodNotAllowed))
		return
	}

	logger := observability.WithTraceContext(r.Context(), slog.Default())

	err := h.client.forwardRun(r.Context(), w, r)
	if err != nil {
		logger.Error("gateway forward run failed", "error", err)
		api.WriteError(w, http.StatusBadGateway, "upstream_unavailable", err.Error())
		return
	}
}

func (h *Handler) handleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", http.StatusText(http.StatusMethodNotAllowed))
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
			api.WriteError(w, http.StatusBadRequest, "invalid_argument", "run must be an integer: "+err.Error())
			return
		}
		query.Run = &runNo
	}

	if err := h.client.forwardReport(r.Context(), w, query); err != nil {
		api.WriteError(w, http.StatusBadGateway, "upstream_unavailable", err.Error())
		return
	}
}

func (h *Handler) handleReady(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", http.StatusText(http.StatusMethodNotAllowed))
		return
	}

	services := map[string]string{
		"validation":   "unknown",
		"reporting":    "unknown",
		"orchestrator": "unknown",
	}

	if resp, err := h.client.checkValidationReady(); err == nil {
		services["validation"] = resp
	}
	if resp, err := h.client.checkReportReady(); err == nil {
		services["reporting"] = resp
	}
	if resp, err := h.client.checkOrchestratorReady(); err == nil {
		services["orchestrator"] = resp
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

	response := struct {
		api.StatusResponse
		Services map[string]string `json:"services"`
	}{
		StatusResponse: api.StatusResponse{Status: overallStatus},
		Services:       services,
	}

	api.WriteJSON(w, statusCode, response)
}

func (c *Client) checkValidationReady() (string, error) {
	resp, err := c.validationClient.Get(c.validationURL + "/ready")
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
	resp, err := c.reportingClient.Get(c.reportURL + "/ready")
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

func (c *Client) checkOrchestratorReady() (string, error) {
	resp, err := c.orchestratorClient.Get(c.orchestratorURL + "/ready")
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
