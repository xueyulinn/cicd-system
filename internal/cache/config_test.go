package cache

import (
	"testing"
	"time"
)

func TestLoadConfig_Defaults(t *testing.T) {
	t.Setenv("VALIDATION_CACHE_ENABLED", "")
	t.Setenv("VALIDATION_CACHE_REDIS_URL", "")
	t.Setenv("VALIDATION_CACHE_KEY_PREFIX", "")
	t.Setenv("VALIDATION_CACHE_VALIDATE_TTL", "")
	t.Setenv("VALIDATION_CACHE_DRYRUN_TTL", "")

	cfg := LoadConfig()
	if !cfg.Enabled {
		t.Fatal("Enabled = false, want true")
	}
	if cfg.RedisURL != defaultRedisURL {
		t.Fatalf("RedisURL = %q, want %q", cfg.RedisURL, defaultRedisURL)
	}
	if cfg.KeyPrefix != defaultKeyPrefix {
		t.Fatalf("KeyPrefix = %q, want %q", cfg.KeyPrefix, defaultKeyPrefix)
	}
	if cfg.ValidateTTL != defaultValidateTTL {
		t.Fatalf("ValidateTTL = %v, want %v", cfg.ValidateTTL, defaultValidateTTL)
	}
	if cfg.DryRunTTL != defaultDryRunTTL {
		t.Fatalf("DryRunTTL = %v, want %v", cfg.DryRunTTL, defaultDryRunTTL)
	}
}

func TestLoadConfig_ParsesEnv(t *testing.T) {
	t.Setenv("VALIDATION_CACHE_ENABLED", "false")
	t.Setenv("VALIDATION_CACHE_REDIS_URL", "redis://redis:6379/0")
	t.Setenv("VALIDATION_CACHE_KEY_PREFIX", "cicd-test")
	t.Setenv("VALIDATION_CACHE_VALIDATE_TTL", "45s")
	t.Setenv("VALIDATION_CACHE_DRYRUN_TTL", "30s")
	t.Setenv("VALIDATION_CACHE_DIAL_TIMEOUT", "3s")
	t.Setenv("VALIDATION_CACHE_READ_TIMEOUT", "800ms")
	t.Setenv("VALIDATION_CACHE_WRITE_TIMEOUT", "900ms")

	cfg := LoadConfig()
	if cfg.Enabled {
		t.Fatal("Enabled = true, want false")
	}
	if cfg.RedisURL != "redis://redis:6379/0" {
		t.Fatalf("RedisURL = %q, want redis://redis:6379/0", cfg.RedisURL)
	}
	if cfg.KeyPrefix != "cicd-test" {
		t.Fatalf("KeyPrefix = %q, want cicd-test", cfg.KeyPrefix)
	}
	if cfg.ValidateTTL != 45*time.Second {
		t.Fatalf("ValidateTTL = %v, want 45s", cfg.ValidateTTL)
	}
	if cfg.DryRunTTL != 30*time.Second {
		t.Fatalf("DryRunTTL = %v, want 30s", cfg.DryRunTTL)
	}
	if cfg.DialTimeout != 3*time.Second {
		t.Fatalf("DialTimeout = %v, want 3s", cfg.DialTimeout)
	}
	if cfg.ReadTimeout != 800*time.Millisecond {
		t.Fatalf("ReadTimeout = %v, want 800ms", cfg.ReadTimeout)
	}
	if cfg.WriteTimeout != 900*time.Millisecond {
		t.Fatalf("WriteTimeout = %v, want 900ms", cfg.WriteTimeout)
	}
}

func TestLoadConfig_InvalidDurationFallback(t *testing.T) {
	t.Setenv("VALIDATION_CACHE_VALIDATE_TTL", "bad")
	t.Setenv("VALIDATION_CACHE_DRYRUN_TTL", "-1s")
	t.Setenv("VALIDATION_CACHE_DIAL_TIMEOUT", "0s")

	cfg := LoadConfig()
	if cfg.ValidateTTL != defaultValidateTTL {
		t.Fatalf("ValidateTTL = %v, want %v", cfg.ValidateTTL, defaultValidateTTL)
	}
	if cfg.DryRunTTL != defaultDryRunTTL {
		t.Fatalf("DryRunTTL = %v, want %v", cfg.DryRunTTL, defaultDryRunTTL)
	}
	if cfg.DialTimeout != defaultRedisDialTimeout {
		t.Fatalf("DialTimeout = %v, want %v", cfg.DialTimeout, defaultRedisDialTimeout)
	}
}

