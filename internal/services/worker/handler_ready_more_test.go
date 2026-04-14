package worker

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CS7580-SEA-SP26/e-team/internal/mq"
)

func TestHandlerClose_WithService(t *testing.T) {
	h := &Handler{service: &Service{jobConsumers: []mq.Consumer{&fakeConsumer{}}}}
	h.Close()
}

func TestHandleReady_ServiceUnavailableWhenServiceNil(t *testing.T) {
	h := &Handler{service: nil}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	h.handleReady(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestHandleReady_MethodNotAllowed(t *testing.T) {
	h := &Handler{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/ready", nil)
	h.handleReady(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleReady_InitErrorTakesPrecedence(t *testing.T) {
	h := &Handler{initErr: errors.New("init failed"), service: nil}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	h.handleReady(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}
