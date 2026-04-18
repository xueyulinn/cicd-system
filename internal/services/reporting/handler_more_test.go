package reporting

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/xueyulinn/cicd-system/internal/store"
)

func TestNewHandler_FailsWithoutDBURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("REPORT_DB_URL", "")
	h, err := NewHandler()
	if err == nil {
		t.Fatal("expected error")
	}
	if h != nil {
		t.Fatalf("handler=%#v, want nil", h)
	}
}

func TestHandlerCloseAndRegisterRoutes(t *testing.T) {
	h := &Handler{}
	h.Close()

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleReady_MethodAndStoreUnavailable(t *testing.T) {
	h := &Handler{service: &Service{}}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/ready", nil)
	h.handleReady(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/ready", nil)
	h.handleReady(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d", rec.Code)
	}

	h = &Handler{service: &Service{store: &mockReportStore{}}}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/ready", nil)
	h.handleReady(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleHealth_Methods(t *testing.T) {
	h := &Handler{}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	h.handleHealth(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/health", nil)
	h.handleHealth(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleReport_MethodAndBadQueries(t *testing.T) {
	h := &Handler{service: &Service{}}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/report", nil)
	h.handleReport(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/report?run=abc", nil)
	h.handleReport(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/report", nil)
	h.handleReport(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestParseReportQuery_TrimsValuesAndOptionalRun(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/report?pipeline=%20demo%20&stage=%20build%20&job=%20lint%20", nil)
	q, err := parseReportQuery(req)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if q.Pipeline != "demo" || q.Stage != "build" || q.Job != "lint" {
		t.Fatalf("query=%+v", q)
	}
	if q.Run != nil {
		t.Fatalf("run=%v, want nil", q.Run)
	}
}

func TestHandleReport_ReturnsServiceValidationErrorStatus(t *testing.T) {
	h := &Handler{service: &Service{}}

	rec := httptest.NewRecorder()
	h.handleReport(rec, httptest.NewRequest(http.MethodGet, "/report?pipeline=p&job=j", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleReport_Success(t *testing.T) {
	now := time.Now().UTC()
	h := &Handler{
		service: &Service{
			store: &mockReportStore{
				runs: []store.Run{{Pipeline: "p", RunNo: 1, Status: store.StatusSuccess, StartTime: now}},
			},
		},
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/report?pipeline=p", nil)
	h.handleReport(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}
