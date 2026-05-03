package objectstorage

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	defaultEndpoint        = "localhost:9000"
	defaultAccessKeyID     = "minioadmin"
	defaultSecretAccessKey = "minioadmin"
	defaultUseSSL          = false
	defaultWorkspaceBucket = "workspace"
)

type Config struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
	WorkspaceBucket string
}

func LoadConfig() Config {
	return Config{
		Endpoint:        loadString("MINIO_ENDPOINT", defaultEndpoint),
		AccessKeyID:     loadString("MINIO_ACCESS_KEY", defaultAccessKeyID),
		SecretAccessKey: loadString("MINIO_SECRET_KEY", defaultSecretAccessKey),
		UseSSL:          loadBool("MINIO_USE_SSL", defaultUseSSL),
		WorkspaceBucket:          loadString("MINIO_WORKSPACE_BUCKET", defaultWorkspaceBucket),
	}
}

func loadString(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
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

func (config *Config) Validate() error {
	if strings.TrimSpace(config.Endpoint) == "" {
		return fmt.Errorf("minio endpoint is required")
	}
	if strings.TrimSpace(config.AccessKeyID) == "" {
		return fmt.Errorf("minio access key id is required")
	}
	if strings.TrimSpace(config.SecretAccessKey) == "" {
		return fmt.Errorf("minio secret access key is required")
	}
	if strings.TrimSpace(config.WorkspaceBucket) == "" {
		return fmt.Errorf("minio workspace bucket is required")
	}
	return nil
}
