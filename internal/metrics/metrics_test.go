package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRecordDrift(t *testing.T) {
	r := NewRecorder()
	t.Cleanup(func() { r.Delete("dc", "default") })

	r.RecordDrift("dc", "default", 2, 1, 3, 4)

	cases := map[string]float64{"changed": 2, "new": 1, "deleted": 3, "unchanged": 4}
	for status, want := range cases {
		if got := testutil.ToFloat64(driftedResources.WithLabelValues("dc", "default", status)); got != want {
			t.Errorf("status %q = %v, want %v", status, got, want)
		}
	}

	// A second record overwrites the gauge rather than accumulating.
	r.RecordDrift("dc", "default", 0, 0, 0, 5)
	if got := testutil.ToFloat64(driftedResources.WithLabelValues("dc", "default", "changed")); got != 0 {
		t.Errorf("after overwrite changed = %v, want 0", got)
	}
}

func TestDelete(t *testing.T) {
	r := NewRecorder()
	r.RecordDrift("gone", "ns", 1, 1, 1, 1)
	r.Delete("gone", "ns")

	// After delete the series is removed; re-reading recreates it at 0.
	if got := testutil.ToFloat64(driftedResources.WithLabelValues("gone", "ns", "changed")); got != 0 {
		t.Errorf("after delete changed = %v, want 0", got)
	}
	t.Cleanup(func() { r.Delete("gone", "ns") })
}

func TestNilRecorderIsNoOp(t *testing.T) {
	var r *Recorder // nil
	// Must not panic.
	r.RecordDrift("x", "y", 1, 2, 3, 4)
	r.Delete("x", "y")
}
