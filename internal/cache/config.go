package cache

import (
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultRedisURL          = "redis://localhost:6379/0"
	defaultKeyPrefix         = "cicd"
	defaultValidateTTL       = 10 * time.Minute
	defaultDryRunTTL         = 5 * time.Minute
	defaultRedisDialTimeout  = 2 * time.Second
	defaultRedisReadTimeout  = 500 * time.Millisecond
	defaultRedisWriteTimeout = 500 * time.Millisecond
)

// Config holds cache/redis settings for validation-service level response caching.
type Config struct {
	Enabled        bool
	RedisURL       string
	KeyPrefix      string
	ValidateTTL    time.Duration
	DryRunTTL      time.Duration
	DialTimeout    time.Duration
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
}

// LoadConfig reads cache settings from env and fills safe defaults.
func LoadConfig() Config {
	return Config{
		Enabled:      loadBool("VALIDATION_CACHE_ENABLED", true),
		RedisURL:     loadString("VALIDATION_CACHE_REDIS_URL", defaultRedisURL),
		KeyPrefix:    loadString("VALIDATION_CACHE_KEY_PREFIX", defaultKeyPrefix),
		ValidateTTL:  loadDuration("VALIDATION_CACHE_VALIDATE_TTL", defaultValidateTTL),
		DryRunTTL:    loadDuration("VALIDATION_CACHE_DRYRUN_TTL", defaultDryRunTTL),
		DialTimeout:  loadDuration("VALIDATION_CACHE_DIAL_TIMEOUT", defaultRedisDialTimeout),
		ReadTimeout:  loadDuration("VALIDATION_CACHE_READ_TIMEOUT", defaultRedisReadTimeout),
		WriteTimeout: loadDuration("VALIDATION_CACHE_WRITE_TIMEOUT", defaultRedisWriteTimeout),
	}
}

func loadString(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

func loadDuration(key string, fallback time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}

	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}

func loadBool(key string, fallback bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}

	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

