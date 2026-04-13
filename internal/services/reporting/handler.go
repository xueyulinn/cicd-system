package reporting

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/CS7580-SEA-SP26/e-team/internal/api"
	"github.com/CS7580-SEA-SP26/e-team/internal/models"
	"github.com/CS7580-SEA-SP26/e-team/internal/observability"
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
		api.WriteJSONError(w, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
		return
	}
	log := observability.WithTraceContext(r.Context(), slog.Default())

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.service.Ping(ctx); err != nil {
		log.Warn("reporting readiness failed", "error", err)
		api.WriteJSONError(w, http.StatusServiceUnavailable, http.StatusText(http.StatusServiceUnavailable))
		return
	}

	log.Debug("reporting ready")
	api.WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// handleHealth reports reporting-service liveness only.
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteJSONError(w, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
		return
	}

	api.WriteJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

// handleReport parses report filters and returns the requested report view.
func (h *Handler) handleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteJSONError(w, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
		return
	}
	log := observability.WithTraceContext(r.Context(), slog.Default())

	query, parseErr := parseReportQuery(r)
	if parseErr != nil {
		log.Warn("invalid report query", "error", parseErr)
		api.WriteJSONError(w, http.StatusBadRequest, parseErr.Error())
		return
	}
	log = log.With("pipeline", query.Pipeline)
	if query.Run != nil {
		log = log.With("run_no", *query.Run)
	}
	if query.Stage != "" {
		log = log.With("stage", query.Stage)
	}
	if query.Job != "" {
		log = log.With("job", query.Job)
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	report, err := h.service.GetReport(ctx, query)
	if err != nil {
		log.Warn("report request failed", "status_code", err.StatusCode, "error", err.Message)
		api.WriteJSONError(w, err.StatusCode, err.Error())
		return
	}

	log.Info("report returned")
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
