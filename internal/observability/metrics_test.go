package observability

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestMain(m *testing.M) {
	// Global default registry; safe once per package test process.
	RegisterMetrics()
	os.Exit(m.Run())
}

func Test_normalizePath(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"/health", "/health"},
		{"/ready", "/ready"},
		{"/metrics", "/metrics"},
		{"/run", "/run"},
		{"/unknown/v1/foo", "/other"},
	}
	for _, tc := range tests {
		if got := normalizePath(tc.in); got != tc.want {
			t.Errorf("normalizePath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRecordExecutionReadyBatchSize_negativeBecomesZero(t *testing.T) {
	before := histogramSampleCount(t, "cicd_execution_ready_batch_size")
	RecordExecutionReadyBatchSize(-3)
	after := histogramSampleCount(t, "cicd_execution_ready_batch_size")
	if after != before+1 {
		t.Fatalf("expected one new histogram observation, before=%d after=%d", before, after)
	}
}

func TestRecordMQJobPublished_outcomes(t *testing.T) {
	q := "test-queue"
	beforeS := counterValue(t, "cicd_mq_jobs_published_total", map[string]string{"queue": q, "outcome": "success"})
	beforeF := counterValue(t, "cicd_mq_jobs_published_total", map[string]string{"queue": q, "outcome": "failure"})
	RecordMQJobPublished(q, true)
	RecordMQJobPublished(q, false)
	if got := counterValue(t, "cicd_mq_jobs_published_total", map[string]string{"queue": q, "outcome": "success"}); got != beforeS+1 {
		t.Errorf("success counter: want %f got %f", beforeS+1, got)
	}
	if got := counterValue(t, "cicd_mq_jobs_published_total", map[string]string{"queue": q, "outcome": "failure"}); got != beforeF+1 {
		t.Errorf("failure counter: want %f got %f", beforeF+1, got)
	}
}

func TestRecordMQDeliveryOutcome(t *testing.T) {
	q := "q2"
	out := "acked"
	before := counterValue(t, "cicd_mq_delivery_outcomes_total", map[string]string{"queue": q, "outcome": out})
	RecordMQDeliveryOutcome(q, out)
	if got := counterValue(t, "cicd_mq_delivery_outcomes_total", map[string]string{"queue": q, "outcome": out}); got != before+1 {
		t.Errorf("delivery counter: want %f got %f", before+1, got)
	}
}

func TestRecordExecutionJobEnqueued(t *testing.T) {
	labels := map[string]string{"pipeline": "p-test", "stage": "build"}
	before := counterValue(t, "cicd_execution_jobs_enqueued_total", labels)
	RecordExecutionJobEnqueued("p-test", "build")
	if got := counterValue(t, "cicd_execution_jobs_enqueued_total", labels); got != before+1 {
		t.Errorf("enqueued counter: want %f got %f", before+1, got)
	}
}

func TestHTTPMetricsMiddleware_recordsRequest(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	h := HTTPMetricsMiddleware(next)

	pathLabels := map[string]string{"method": "GET", "path": "/health", "code": http.StatusText(http.StatusTeapot)}
	before := counterValue(t, "http_requests_total", pathLabels)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTeapot {
		t.Fatalf("status %d", rec.Code)
	}

	if got := counterValue(t, "http_requests_total", pathLabels); got != before+1 {
		t.Errorf("http_requests_total: want %f got %f", before+1, got)
	}
}

func counterValue(t *testing.T, name string, wantLabels map[string]string) float64 {
	t.Helper()
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
		for _, m := range mf.GetMetric() {
			if labelsMatch(m.GetLabel(), wantLabels) {
				return m.GetCounter().GetValue()
			}
		}
	}
	return 0
}

func histogramSampleCount(t *testing.T, name string) uint64 {
	t.Helper()
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
		var total uint64
		for _, m := range mf.GetMetric() {
			total += m.GetHistogram().GetSampleCount()
		}
		return total
	}
	return 0
}

func labelsMatch(lps []*dto.LabelPair, want map[string]string) bool {
	if len(lps) != len(want) {
		return false
	}
	got := make(map[string]string, len(lps))
	for _, lp := range lps {
		got[lp.GetName()] = lp.GetValue()
	}
	for k, v := range want {
		if got[k] != v {
			return false
		}
	}
	return true
}

func TestMetricsHandler_servesPrometheusText(t *testing.T) {
	h := MetricsHandler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if len(body) < 100 {
		t.Fatalf("metrics body too short: %d", len(body))
	}
}
