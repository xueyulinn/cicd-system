package config

import (
	"os"
	"strings"
)

// GetEnvOrDefault returns the trimmed env value, or fallback if unset or empty.
func GetEnvOrDefault(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

// GetEnvOrDefaultURL returns GetEnvOrDefault and trims trailing slashes (for base URLs).
func GetEnvOrDefaultURL(key, fallback string) string {
	return strings.TrimRight(GetEnvOrDefault(key, fallback), "/")
}

// Default ports (without colon prefix). Use with ":" + Port for listen address.
const (
	DefaultGatewayPort    = "8000"
	DefaultValidationPort = "8001"
	DefaultExecutionPort  = "8002"
	DefaultWorkerPort     = "8003"
	DefaultReportingPort  = "8004"
)

// Default base URLs for services (used when env vars are unset).
const (
	DefaultGatewayURL    = "http://localhost:8000"
	DefaultValidationURL = "http://localhost:8001"
	DefaultExecutionURL  = "http://localhost:8002"
	DefaultWorkerURL     = "http://localhost:8003"
	DefaultReportingURL  = "http://localhost:8004"
)

const DefaultPipelineConfigPath = ".pipelines/pipeline.yaml"
