package objectstorage

import (
	"strings"
	"testing"
)

func TestConfigDefaults(t *testing.T) {
	t.Setenv("MINIO_ENDPOINT", "")
	t.Setenv("MINIO_ACCESS_KEY", "")
	config := LoadConfig()
	if config.Endpoint != defaultEndpoint {
		t.Fatalf("expected default endpoint: %q, got: %q", defaultEndpoint, config.Endpoint)
	}

	if config.AccessKeyID != "minioadmin" {
		t.Fatalf("expected default access key: %q, got: %q", defaultAccessKeyID, config.AccessKeyID)
	}
}

func TestConfigFromEnv(t *testing.T) {
	t.Setenv("MINIO_ENDPOINT", "localhost:9001")
	t.Setenv("MINIO_ACCESS_KEY", "minioguest")
	config := LoadConfig()
	if config.Endpoint != "localhost:9001" {
		t.Fatalf("expected default endpoint: %q, got: %q", defaultEndpoint, config.Endpoint)
	}

	if config.AccessKeyID != "minioguest" {
		t.Fatalf("expected default access key: %q, got: %q", defaultAccessKeyID, config.AccessKeyID)
	}
}

func TestConfigValidate(t *testing.T) {
	config := LoadConfig()
	if err := config.Validate(); err != nil {
		t.Fatalf("expected validated config got: %v", err)
	}

	config.Endpoint = ""
	if err := config.Validate(); err != nil && !strings.Contains(err.Error(), "minio endpoint") {
		t.Fatalf("expected invalid empty endpoint got: %v", err)
	}
}
