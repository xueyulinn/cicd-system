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
}

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

func (h *Handler) handleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(report); err != nil {
		api.WriteJSONError(w, http.StatusInternalServerError, "failed to encode response")
	}
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
