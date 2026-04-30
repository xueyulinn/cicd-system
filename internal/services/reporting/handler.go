package reporting

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/xueyulinn/cicd-system/internal/api"
	"github.com/xueyulinn/cicd-system/internal/models"
	"github.com/xueyulinn/cicd-system/internal/observability"
)

// Handler serves reporting HTTP endpoints.
type Handler struct {
	service *Service
}

// NewHandler constructs a reporting handler and initializes its backing service.
func NewHandler() (*Handler, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	svc, err := NewService(ctx)
	if err != nil {
		return nil, err
	}
	return &Handler{service: svc}, nil
}

// Close releases resources owned by the underlying reporting service.
func (h *Handler) Close() {
	if h.service != nil {
		h.service.Close()
	}
}

// RegisterRoutes registers the reporting service HTTP routes.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", h.handleHealth)
	mux.HandleFunc("/report", h.handleReport)
	mux.HandleFunc("/ready", h.handleReady)
}

// handleReady reports reporting-service readiness based on report-store reachability.
func (h *Handler) handleReady(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", http.StatusText(http.StatusMethodNotAllowed))
		return
	}
	log := observability.WithTraceContext(r.Context(), slog.Default())

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.service.Ping(ctx); err != nil {
		log.Warn("reporting readiness failed", "error", err)
		api.WriteError(w, http.StatusServiceUnavailable, "service_unready", http.StatusText(http.StatusServiceUnavailable))
		return
	}

	log.Debug("reporting ready")
	api.WriteJSON(w, http.StatusOK, api.StatusResponse{Status: "ready"})
}

// handleHealth reports reporting-service liveness only.
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", http.StatusText(http.StatusMethodNotAllowed))
		return
	}

	api.WriteJSON(w, http.StatusOK, api.StatusResponse{Status: "healthy"})
}

// handleReport parses report filters and returns the requested report view.
func (h *Handler) handleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", http.StatusText(http.StatusMethodNotAllowed))
		return
	}
	log := observability.WithTraceContext(r.Context(), slog.Default())

	query, parseErr := parseReportQuery(r)
	if parseErr != nil {
		log.Error("invalid report query", "error", parseErr)
		api.WriteError(w, http.StatusBadRequest, "invalid_argument", parseErr.Error())
		return
	}

	log = observability.WithReportQueryContext(log, query)

	ctx := r.Context()
	report, err := h.service.GetReport(ctx, query)
	if err != nil {
		status, code, message := classifyError(err)
		api.WriteError(w, status, code, message)
		return
	}

	api.WriteJSON(w, http.StatusOK, report)
}

// parseReportQuery parses supported report filters from the incoming HTTP request.
func parseReportQuery(r *http.Request) (models.ReportQuery, error) {
	query := models.ReportQuery{
		Pipeline: strings.TrimSpace(r.URL.Query().Get("pipeline")),
		Stage:    strings.TrimSpace(r.URL.Query().Get("stage")),
		Job:      strings.TrimSpace(r.URL.Query().Get("job")),
	}

	runParam := strings.TrimSpace(r.URL.Query().Get("run"))
	if runParam != "" {
		runNo, err := strconv.Atoi(runParam)
		if err != nil {
			return models.ReportQuery{}, fmt.Errorf("parse report query run parameter: %w", err)
		}
		query.Run = &runNo
	}

	return query, nil
}
