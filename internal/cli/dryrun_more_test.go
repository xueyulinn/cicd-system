package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func writeNonHeuristicPipeline(t *testing.T, content string) string {
	t.Helper()
	d, err := os.MkdirTemp("", "cli-dr-*")
	if err != nil {
		t.Fatalf("mkdirtemp: %v", err)
	}
	path := filepath.Join(d, "pipeline.yaml")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func TestRunDryRun_GatewayMode_Success(t *testing.T) {
	cfg := writeNonHeuristicPipeline(t, `pipeline:
  name: "p"
stages:
  - build
compile:
  - stage: build
  - image: alpine
  - script:
    - echo ok
`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dryrun" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"valid":true,"output":"build:\n  compile:\n"}`))
	}))
	defer srv.Close()
	t.Setenv("GATEWAY_URL", srv.URL)
	t.Setenv("CICD_TEST_MODE", "0")

	out, err := captureStdout(t, func() error { return runDryRun(&cobra.Command{}, []string{cfg}) })
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if !strings.Contains(out, "build:") {
		t.Fatalf("output=%q", out)
	}
}

func TestRunDryRun_GatewayMode_InvalidAndGatewayErrorAndReadError(t *testing.T) {
	cfg := writeNonHeuristicPipeline(t, `pipeline:
  name: "p"
stages:
  - build
compile:
  - stage: build
  - image: alpine
  - script:
    - echo ok
`)

	t.Run("invalid response", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"valid":false,"errors":["e1","e2"]}`))
		}))
		defer srv.Close()
		t.Setenv("GATEWAY_URL", srv.URL)
		t.Setenv("CICD_TEST_MODE", "0")

		err := runDryRun(&cobra.Command{}, []string{cfg})
		if err == nil || !strings.Contains(err.Error(), "dry run failed with 2 error(s)") {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("gateway error", func(t *testing.T) {
		t.Setenv("GATEWAY_URL", "http://127.0.0.1:1")
		t.Setenv("CICD_TEST_MODE", "0")
		err := runDryRun(&cobra.Command{}, []string{cfg})
		if err == nil || !strings.Contains(err.Error(), "dry run failed with 1 error(s)") {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("read file error", func(t *testing.T) {
		err := runDryRun(&cobra.Command{}, []string{filepath.Join(filepath.Dir(cfg), "missing.yaml")})
		if err == nil || !strings.Contains(err.Error(), "failed to read file") {
			t.Fatalf("err=%v", err)
		}
	})
}
