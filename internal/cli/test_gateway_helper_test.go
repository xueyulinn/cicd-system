package cli

import (
	"net/http"
	"net/http/httptest"
	"testing"

	validationservice "github.com/xueyulinn/cicd-system/internal/services/validation"
)

func startValidationGatewayServer(t *testing.T) *httptest.Server {
	t.Helper()

	t.Setenv("VALIDATION_CACHE_ENABLED", "false")

	handler, err := validationservice.NewHandler()
	if err != nil {
		t.Fatalf("validation NewHandler() error = %v", err)
	}

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	t.Setenv("GATEWAY_URL", srv.URL)
	t.Cleanup(srv.Close)

	return srv
}
