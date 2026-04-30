package validation

import "testing"

func disableValidationCache(t *testing.T) {
	t.Helper()
	t.Setenv("VALIDATION_CACHE_ENABLED", "false")
}
