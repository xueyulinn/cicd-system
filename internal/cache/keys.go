package cache

import (
	"strings"
)

const (
	validateNamespace = "validate:v2"
	dryRunNamespace   = "dryrun:v2"
)

// ValidateKey returns a stable redis key for validate response cache.
func ValidateKey(prefix, path, commit string) string {
	return buildKey(prefix, validateNamespace, path, commit)
}

// DryRunKey returns a stable redis key for dryrun response cache.
func DryRunKey(prefix, path, commit string) string {
	return buildKey(prefix, dryRunNamespace, path, commit)
}

func buildKey(prefix, namespace, path, commit string) string {
	path = strings.TrimSpace(path)
	commit = strings.TrimSpace(commit)

	val := commit + ":" + path

	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return namespace + ":" + val
	}

	return prefix + ":" + namespace + ":" + val
}
