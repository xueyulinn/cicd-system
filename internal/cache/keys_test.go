package cache

import (
	"strings"
	"testing"
)

func TestValidateKeyAndDryRunKey_StableAndDifferentNamespaces(t *testing.T) {
	content := "pipeline:\n  name: demo\n"
	v1 := ValidateKey("cicd", content)
	v2 := ValidateKey("cicd", content)
	d1 := DryRunKey("cicd", content)

	if v1 != v2 {
		t.Fatalf("ValidateKey not stable: %q != %q", v1, v2)
	}
	if v1 == d1 {
		t.Fatalf("ValidateKey and DryRunKey should differ, both=%q", v1)
	}
	if !strings.HasPrefix(v1, "cicd:validate:v1:") {
		t.Fatalf("unexpected validate key prefix: %q", v1)
	}
	if !strings.HasPrefix(d1, "cicd:dryrun:v1:") {
		t.Fatalf("unexpected dryrun key prefix: %q", d1)
	}
}

func TestKeyWithoutPrefix(t *testing.T) {
	k := ValidateKey("", "x")
	if !strings.HasPrefix(k, "validate:v1:") {
		t.Fatalf("unexpected key: %q", k)
	}
}

