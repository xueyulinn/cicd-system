package gateway

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/CS7580-SEA-SP26/e-team/internal/api"
)

func newGatewayTestHandler(t *testing.T, fn http.HandlerFunc) (*Handler, func()) {
	t.Helper()
	srv := httptest.NewServer(fn)
	client := srv.Client()

	h := &Handler{
		client: &Client{
			validationURL:  srv.URL,
			executionURL:   srv.URL,
			reportURL:      srv.URL,
			httpValidation: client,
			httpExecution:  client,
			httpReporting:  client,
		},
	}

	return h, srv.Close
}

func doGatewayRequest(t *testing.T, mux *http.ServeMux, method, path string, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, body)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestNewHandlerAndRegisterRoutes(t *testing.T) {
	h := NewHandler()
	if h == nil || h.client == nil {
		t.Fatal("expected handler and client")
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	rec := doGatewayRequest(t, mux, http.MethodGet, "/health", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /health status=%d, want %d", rec.Code, http.StatusOK)
	}
}

func TestDecodeYAMLContentRequestReadError(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/validate", nil)
	req.Body = errReadCloser{}

	_, err := decodeYAMLContentRequest(req)
	if err == nil || !strings.Contains(err.Error(), "failed to read request body") {
		t.Fatalf("expected read error, got %v", err)
	}
}

func TestHandleHealthAndServices(t *testing.T) {
	h, closeFn := newGatewayTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})
	defer closeFn()

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	rec := doGatewayRequest(t, mux, http.MethodPost, "/health", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /health status=%d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}

	t.Setenv("GATEWAY_PUBLIC_URL", "http://gateway.example.com/")
	rec = doGatewayRequest(t, mux, http.MethodGet, "/services", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /services status=%d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "gateway.example.com") || !strings.Contains(body, `"validation"`) {
		t.Fatalf("unexpected services response: %q", body)
	}

	rec = doGatewayRequest(t, mux, http.MethodPost, "/services", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /services status=%d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleValidate(t *testing.T) {
	h, closeFn := newGatewayTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/validate" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "invalid") {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"valid":false,"errors":["invalid pipeline"]}`))
			return
		}
		_, _ = w.Write([]byte(`{"valid":true}`))
	})
	defer closeFn()

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	rec := doGatewayRequest(t, mux, http.MethodGet, "/validate", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /validate status=%d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}

	rec = doGatewayRequest(t, mux, http.MethodPost, "/validate", strings.NewReader(`{`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid JSON status=%d, want %d", rec.Code, http.StatusBadRequest)
	}

	rec = doGatewayRequest(t, mux, http.MethodPost, "/validate", strings.NewReader(`{"yaml_content":""}`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing yaml status=%d, want %d", rec.Code, http.StatusBadRequest)
	}

	rec = doGatewayRequest(t, mux, http.MethodPost, "/validate", strings.NewReader(`{"yaml_content":"invalid"}`))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("invalid pipeline proxy status=%d, want %d", rec.Code, http.StatusBadGateway)
	}

	rec = doGatewayRequest(t, mux, http.MethodPost, "/validate", strings.NewReader(`{"yaml_content":"ok"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("valid status=%d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandleDryRun(t *testing.T) {
	h, closeFn := newGatewayTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dryrun" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "bad") {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"valid":false,"errors":["bad yaml"]}`))
			return
		}
		_, _ = w.Write([]byte(`{"valid":true,"output":"plan"}`))
	})
	defer closeFn()

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	rec := doGatewayRequest(t, mux, http.MethodGet, "/dryrun", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /dryrun status=%d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}

	rec = doGatewayRequest(t, mux, http.MethodPost, "/dryrun", strings.NewReader(`{"yaml_content":"bad"}`))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("bad pipeline proxy status=%d, want %d", rec.Code, http.StatusBadGateway)
	}

	rec = doGatewayRequest(t, mux, http.MethodPost, "/dryrun", strings.NewReader(`{"yaml_content":"ok"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("valid dryrun status=%d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandleRun(t *testing.T) {
	h, closeFn := newGatewayTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/run" {
			http.NotFound(w, r)
			return
		}
		var req api.RunRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Branch == "bad" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"status":"failed","errors":["bad branch"]}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"running","message":"queued"}`))
	})
	defer closeFn()

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	rec := doGatewayRequest(t, mux, http.MethodGet, "/run", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /run status=%d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}

	rec = doGatewayRequest(t, mux, http.MethodPost, "/run", strings.NewReader(`{`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid JSON status=%d, want %d", rec.Code, http.StatusBadRequest)
	}

	rec = doGatewayRequest(t, mux, http.MethodPost, "/run", strings.NewReader(`{"yaml_content":"p","branch":"bad","commit":"1"}`))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("downstream failed status=%d, want %d", rec.Code, http.StatusBadGateway)
	}

	rec = doGatewayRequest(t, mux, http.MethodPost, "/run", strings.NewReader(`{"yaml_content":"p","branch":"main","commit":"1"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("run success status=%d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandleReport(t *testing.T) {
	h, closeFn := newGatewayTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/report" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("pipeline") == "missing" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"pipeline missing"}`))
			return
		}
		_, _ = w.Write([]byte(`{"runs":[]}`))
	})
	defer closeFn()

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	rec := doGatewayRequest(t, mux, http.MethodPost, "/report", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /report status=%d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}

	rec = doGatewayRequest(t, mux, http.MethodGet, "/report?pipeline=p&run=abc", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid run status=%d, want %d", rec.Code, http.StatusBadRequest)
	}

	rec = doGatewayRequest(t, mux, http.MethodGet, "/report?pipeline=missing", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("downstream error status=%d, want %d", rec.Code, http.StatusBadRequest)
	}

	rec = doGatewayRequest(t, mux, http.MethodGet, "/report?pipeline=p&run=1&stage=build&job=compile", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("report success status=%d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandleReady(t *testing.T) {
	h, closeFn := newGatewayTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ready":
			_, _ = w.Write([]byte(`{"status":"ready"}`))
		default:
			http.NotFound(w, r)
		}
	})
	defer closeFn()

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	rec := doGatewayRequest(t, mux, http.MethodPost, "/ready", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /ready status=%d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}

	rec = doGatewayRequest(t, mux, http.MethodGet, "/ready", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("all ready status=%d, want %d", rec.Code, http.StatusOK)
	}

	h.client.httpExecution = &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("execution down")
		}),
	}
	rec = doGatewayRequest(t, mux, http.MethodGet, "/ready", nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("partial readiness status=%d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestHandleRunReadBodyError(t *testing.T) {
	h, closeFn := newGatewayTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"running"}`))
	})
	defer closeFn()

	req := httptest.NewRequest(http.MethodPost, "/run", nil)
	req.Body = errReadCloser{}
	rec := httptest.NewRecorder()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("read body status=%d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleValidateProxyNetworkError(t *testing.T) {
	h := &Handler{
		client: &Client{
			validationURL: "http://validation.invalid",
			httpValidation: &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return nil, errors.New("network error")
				}),
			},
		},
	}
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	rec := doGatewayRequest(t, mux, http.MethodPost, "/validate", bytes.NewBufferString(`{"yaml_content":"ok"}`))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("proxy error status=%d, want %d", rec.Code, http.StatusBadGateway)
	}
}
