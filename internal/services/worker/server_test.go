package worker

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CS7580-SEA-SP26/e-team/internal/config"
)

func TestHandleHealth_GET_returnsOK(t *testing.T) {
	srv := NewServer("", nil, 0)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET /health: status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("GET /health: Content-Type = %q, want %q", ct, "application/json")
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("GET /health: decode body: %v", err)
	}
	if body["status"] != "healthy" {
		t.Errorf("GET /health: status body = %q, want %q", body["status"], "healthy")
	}
}

func TestHandleHealth_nonGET_returnsMethodNotAllowed(t *testing.T) {
	srv := NewServer("", nil, 0)
	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		req := httptest.NewRequest(method, "/health", nil)
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s /health: status = %d, want %d", method, rec.Code, http.StatusMethodNotAllowed)
		}
	}
}

func TestNewServer_emptyAddr_usesDefault(t *testing.T) {
	srv := NewServer("", nil, 0)
	want := ":" + config.DefaultWorkerPort
	if srv.addr != want {
		t.Errorf("NewServer(``, nil, 0): addr = %q, want %q", srv.addr, want)
	}
}
