package reporting

import (
	"net/http/httptest"
	"testing"
)

func TestParseReportQueryValid(t *testing.T) {
	req := httptest.NewRequest("GET", "/report?pipeline=demo&run=2&stage=build&job=lint", nil)

	got, err := parseReportQuery(req)
	if err != nil {
		t.Fatalf("parseReportQuery returned error: %v", err)
	}
	if got.Pipeline != "demo" {
		t.Fatalf("Pipeline = %q, want %q", got.Pipeline, "demo")
	}
	if got.Run == nil || *got.Run != 2 {
		t.Fatalf("Run = %+v, want 2", got.Run)
	}
	if got.Stage != "build" || got.Job != "lint" {
		t.Fatalf("unexpected query: %+v", got)
	}
}

func TestParseReportQueryRejectsInvalidRun(t *testing.T) {
	req := httptest.NewRequest("GET", "/report?pipeline=demo&run=abc", nil)

	_, err := parseReportQuery(req)
	if err == nil {
		t.Fatal("expected error for invalid run")
	}
}
