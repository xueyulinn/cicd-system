package worker

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
	body := rec.Body.String()
	if body != `{"status":"ok"}` {
		t.Errorf("GET /health: body = %q, want %q", body, `{"status":"ok"}`)
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
	if srv.addr != defaultAddr {
		t.Errorf("NewServer(``, nil, 0): addr = %q, want %q", srv.addr, defaultAddr)
	}
}
