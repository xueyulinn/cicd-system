package config

import "testing"

func TestGetEnvOrDefaultReturnsFallbackForBlankValue(t *testing.T) {
	t.Setenv("TEST_EMPTY_ENV", "   ")

	got := GetEnvOrDefault("TEST_EMPTY_ENV", "fallback")
	if got != "fallback" {
		t.Fatalf("GetEnvOrDefault() = %q, want %q", got, "fallback")
	}
}

func TestGetEnvOrDefaultURLTrimsTrailingSlash(t *testing.T) {
	t.Setenv("TEST_URL_ENV", " http://localhost:8000/ ")

	got := GetEnvOrDefaultURL("TEST_URL_ENV", "http://fallback/")
	if got != "http://localhost:8000" {
		t.Fatalf("GetEnvOrDefaultURL() = %q, want %q", got, "http://localhost:8000")
	}
}
