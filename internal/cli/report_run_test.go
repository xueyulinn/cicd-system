package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xueyulinn/cicd-system/internal/models"
	"github.com/spf13/cobra"
)

func TestRunReport_InvalidFormat(t *testing.T) {
	resetReportFlags()
	reportRun = 1
	cmd := &cobra.Command{}
	cmd.Flags().StringP("format", "f", formatYAML, "")
	_ = cmd.Flags().Set("format", "xml")

	err := runReport(cmd, []string{"demo"})
	if err == nil || !strings.Contains(err.Error(), "invalid format") {
		t.Fatalf("err=%v", err)
	}
}

func TestRunReport_SuccessJSON(t *testing.T) {
	resetReportFlags()
	reportRun = 1

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/report" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		resp := models.ReportResponse{Pipeline: models.ReportPipeline{Name: "demo"}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()
	t.Setenv("GATEWAY_URL", srv.URL)

	cmd := &cobra.Command{}
	cmd.Flags().StringP("format", "f", formatYAML, "")
	_ = cmd.Flags().Set("format", formatJSON)

	out, err := captureStdout(t, func() error { return runReport(cmd, []string{"demo"}) })
	if err != nil {
		t.Fatalf("runReport err=%v", err)
	}
	if !strings.Contains(out, "pipeline") || !strings.Contains(out, "demo") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestRunReport_GatewayError(t *testing.T) {
	resetReportFlags()
	reportRun = 1

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"report unavailable"}`))
	}))
	defer srv.Close()
	t.Setenv("GATEWAY_URL", srv.URL)

	cmd := &cobra.Command{}
	cmd.Flags().StringP("format", "f", formatYAML, "")
	_ = cmd.Flags().Set("format", formatYAML)

	err := runReport(cmd, []string{"demo"})
	if err == nil || !strings.Contains(err.Error(), "report unavailable") {
		t.Fatalf("err=%v", err)
	}
}
