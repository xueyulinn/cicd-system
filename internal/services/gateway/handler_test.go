package gateway

import (
	"testing"
)

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
