package reporting

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

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
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "healthy"}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *Handler) handleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query, parseErr := parseReportQuery(r)
	if parseErr != nil {
		writeError(w, http.StatusBadRequest, parseErr.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	report, err := h.service.GetReport(ctx, query)
	if err != nil {
		writeError(w, err.StatusCode, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(report); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
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
			return models.ReportQuery{}, err
		}
		query.Run = &runNo
	}

	return query, nil
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": message}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
