package gateway

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeYAMLContentRequest(t *testing.T) {
	req := httptest.NewRequest("POST", "/validate", strings.NewReader(`{"yaml_content":"pipeline:\n  name: test"}`))

	got, err := decodeYAMLContentRequest(req)
	if err != nil {
		t.Fatalf("decodeYAMLContentRequest returned error: %v", err)
	}
	if got != "pipeline:\n  name: test" {
		t.Fatalf("decodeYAMLContentRequest = %q, want %q", got, "pipeline:\n  name: test")
	}
}

func TestDecodeYAMLContentRequestRejectsMissingField(t *testing.T) {
	req := httptest.NewRequest("POST", "/validate", strings.NewReader(`{"other":"value"}`))

	_, err := decodeYAMLContentRequest(req)
	if err == nil {
		t.Fatal("expected error for missing yaml_content")
	}
	if !strings.Contains(err.Error(), "missing yaml_content field") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetGatewayPublicURLDefaultsAndTrims(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		t.Setenv("GATEWAY_PUBLIC_URL", "")
		if got := getGatewayPublicURL(); got != "http://localhost:8000" {
			t.Fatalf("getGatewayPublicURL() = %q, want %q", got, "http://localhost:8000")
		}
	})

	t.Run("trim trailing slash", func(t *testing.T) {
		t.Setenv("GATEWAY_PUBLIC_URL", " http://example.com/ ")
		if got := getGatewayPublicURL(); got != "http://example.com" {
			t.Fatalf("getGatewayPublicURL() = %q, want %q", got, "http://example.com")
		}
	})
}
