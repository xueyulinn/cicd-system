package execution

import (
	"fmt"
	"testing"
	"time"

	"github.com/CS7580-SEA-SP26/e-team/internal/observability"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func uniqueLabel(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func TestRecordPipelineOutcomeZeroStartNoop(t *testing.T) {
	pipeline := uniqueLabel("pipe-zero")
	runNo := 901
	before := testutil.ToFloat64(observability.PipelineRunsTotal.WithLabelValues(pipeline, "901", "success"))

	recordPipelineOutcome(pipeline, runNo, "success", time.Time{})
	after := testutil.ToFloat64(observability.PipelineRunsTotal.WithLabelValues(pipeline, "901", "success"))
	if after != before {
		t.Fatalf("counter changed for zero start: before=%v after=%v", before, after)
	}
}

func TestRecordPipelineOutcomeIncrementsMetrics(t *testing.T) {
	pipeline := uniqueLabel("pipe")
	runNo := 902
	before := testutil.ToFloat64(observability.PipelineRunsTotal.WithLabelValues(pipeline, "902", "success"))

	recordPipelineOutcome(pipeline, runNo, "success", time.Now().Add(-2*time.Second))
	after := testutil.ToFloat64(observability.PipelineRunsTotal.WithLabelValues(pipeline, "902", "success"))
	if after != before+1 {
		t.Fatalf("counter delta=%v, want 1", after-before)
	}

	count := testutil.CollectAndCount(observability.PipelineDurationSeconds)
	if count == 0 {
		t.Fatal("expected pipeline duration histogram to contain at least one series")
	}
}

func TestRecordStageDurationZeroStartNoop(t *testing.T) {
	before := testutil.CollectAndCount(observability.StageDurationSeconds)
	recordStageDuration(uniqueLabel("pipe-stage-zero"), 903, "build", time.Time{})
	after := testutil.CollectAndCount(observability.StageDurationSeconds)
	if after != before {
		t.Fatalf("stage histogram series changed for zero start: before=%d after=%d", before, after)
	}
}

func TestRecordStageDurationIncrementsHistogram(t *testing.T) {
	before := testutil.CollectAndCount(observability.StageDurationSeconds)
	recordStageDuration(uniqueLabel("pipe-stage"), 904, "test", time.Now().Add(-time.Second))
	after := testutil.CollectAndCount(observability.StageDurationSeconds)
	if after <= before {
		t.Fatalf("stage histogram series did not increase: before=%d after=%d", before, after)
	}
}

func TestRecordJobOutcomeIncrementsCounterAndHistogram(t *testing.T) {
	pipeline := uniqueLabel("pipe-job")
	runNo := 905
	counterBefore := testutil.ToFloat64(observability.JobRunsTotal.WithLabelValues(pipeline, "905", "build", "compile", "success"))
	hBefore := testutil.CollectAndCount(observability.JobDurationSeconds)

	recordJobOutcome(pipeline, runNo, "build", "compile", "success", time.Now().Add(-1500*time.Millisecond))

	counterAfter := testutil.ToFloat64(observability.JobRunsTotal.WithLabelValues(pipeline, "905", "build", "compile", "success"))
	hAfter := testutil.CollectAndCount(observability.JobDurationSeconds)
	if counterAfter != counterBefore+1 {
		t.Fatalf("job counter delta=%v, want 1", counterAfter-counterBefore)
	}
	if hAfter <= hBefore {
		t.Fatalf("job histogram series did not increase: before=%d after=%d", hBefore, hAfter)
	}
}

func TestRecordJobOutcomeZeroStartStillIncrementsCounter(t *testing.T) {
	pipeline := uniqueLabel("pipe-job-zero")
	counterBefore := testutil.ToFloat64(observability.JobRunsTotal.WithLabelValues(pipeline, "906", "build", "lint", "failed"))
	hBefore := testutil.CollectAndCount(observability.JobDurationSeconds)

	recordJobOutcome(pipeline, 906, "build", "lint", "failed", time.Time{})

	counterAfter := testutil.ToFloat64(observability.JobRunsTotal.WithLabelValues(pipeline, "906", "build", "lint", "failed"))
	hAfter := testutil.CollectAndCount(observability.JobDurationSeconds)
	if counterAfter != counterBefore+1 {
		t.Fatalf("job counter delta=%v, want 1", counterAfter-counterBefore)
	}
	if hAfter != hBefore {
		t.Fatalf("job histogram series should not change for zero start: before=%d after=%d", hBefore, hAfter)
	}
}
