package cache

import (
	"strings"
	"testing"
)

func TestValidateKeyAndDryRunKey_StableAndDifferentNamespaces(t *testing.T) {
	path := "build.yaml"
	commit := "abc123"
	v1 := ValidateKey("cicd", path, commit)
	v2 := ValidateKey("cicd", path, commit)
	d1 := DryRunKey("cicd", path, commit)

	if v1 != v2 {
		t.Fatalf("ValidateKey not stable: %q != %q", v1, v2)
	}
	if v1 == d1 {
		t.Fatalf("ValidateKey and DryRunKey should differ, both=%q", v1)
	}
	if !strings.HasPrefix(v1, "cicd:validate:v2:") {
		t.Fatalf("unexpected validate key prefix: %q", v1)
	}
	if !strings.HasPrefix(d1, "cicd:dryrun:v2:") {
		t.Fatalf("unexpected dryrun key prefix: %q", d1)
	}
	if !strings.HasSuffix(v1, "abc123:build.yaml") {
		t.Fatalf("unexpected validate key suffix: %q", v1)
	}
}

func TestKeyWithoutPrefix(t *testing.T) {
	k := ValidateKey("", "build.yaml", "abc123")
	if !strings.HasPrefix(k, "validate:v2:") {
		t.Fatalf("unexpected key: %q", k)
	}
}
