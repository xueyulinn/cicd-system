package reporting

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/CS7580-SEA-SP26/e-team/internal/api"
	"github.com/CS7580-SEA-SP26/e-team/internal/models"
)

type Handler struct {
	service *Service
}

func NewHandler() (*Handler, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	svc, err := NewService(ctx)
	if err != nil {
		return nil, err
	}
	return &Handler{service: svc}, nil
}

func (h *Handler) Close() {
	if h.service != nil {
		h.service.Close()
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", h.handleHealth)
	mux.HandleFunc("/report", h.handleReport)
	mux.HandleFunc("/ready", h.handleReady)
}

// Check is DB is ready
func (h *Handler) handleReady(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteJSONError(w, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.service.Ping(ctx); err != nil {
		api.WriteJSONError(w, http.StatusServiceUnavailable, http.StatusText(http.StatusServiceUnavailable))
		return
	}

	api.WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteJSONError(w, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
		return
	}

	api.WriteJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

func (h *Handler) handleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteJSONError(w, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
		return
	}

	query, parseErr := parseReportQuery(r)
	if parseErr != nil {
		api.WriteJSONError(w, http.StatusBadRequest, parseErr.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	report, err := h.service.GetReport(ctx, query)
	if err != nil {
		api.WriteJSONError(w, err.StatusCode, err.Error())
		return
	}

	api.WriteJSON(w, http.StatusOK, report)
}

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
