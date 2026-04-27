package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

const (
	validateNamespace = "validate:v1"
	dryRunNamespace   = "dryrun:v1"
)

// ValidateKey returns a stable redis key for validate response cache.
func ValidateKey(prefix, yamlContent string) string {
	return buildKey(prefix, validateNamespace, yamlContent)
}

// DryRunKey returns a stable redis key for dryrun response cache.
func DryRunKey(prefix, yamlContent string) string {
	return buildKey(prefix, dryRunNamespace, yamlContent)
}

func buildKey(prefix, namespace, content string) string {
	sum := sha256.Sum256([]byte(content))
	hash := hex.EncodeToString(sum[:])

	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return namespace + ":" + hash
	}

	return prefix + ":" + namespace + ":" + hash
}

