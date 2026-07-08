// Package metrics defines and registers the Prometheus metrics that the
// kube-drift controller exposes on the controller-runtime metrics endpoint.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

// driftedResources reports, per DriftCheck, how many compared resources fall
// into each drift status. It is a gauge (not a counter) because every reconcile
// overwrites the current tally, so it is intentionally named without a "_total"
// suffix. The status label carries the kube-diff values: changed, new, deleted,
// unchanged.
var driftedResources = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "kube_drift_resources",
		Help: "Number of resources per drift status for a DriftCheck (changed, new, deleted, unchanged).",
	},
	[]string{"driftcheck", "namespace", "status"},
)

func init() {
	// Register with the controller-runtime registry so the series are served on
	// the manager's metrics endpoint alongside the built-in controller metrics.
	ctrlmetrics.Registry.MustRegister(driftedResources)
}

// Recorder writes kube-drift metrics. A nil *Recorder is safe to use: every
// method is a no-op, so the controller can run without metrics wiring (e.g. in
// unit tests that do not care about metrics).
type Recorder struct{}

// NewRecorder returns a Recorder that writes to the global controller-runtime
// registry.
func NewRecorder() *Recorder { return &Recorder{} }

// RecordDrift sets the per-status resource gauge for a single DriftCheck. The
// "added" argument maps to the "new" status label (new is a Go keyword).
func (r *Recorder) RecordDrift(name, namespace string, changed, added, deleted, unchanged int) {
	if r == nil {
		return
	}
	driftedResources.WithLabelValues(name, namespace, "changed").Set(float64(changed))
	driftedResources.WithLabelValues(name, namespace, "new").Set(float64(added))
	driftedResources.WithLabelValues(name, namespace, "deleted").Set(float64(deleted))
	driftedResources.WithLabelValues(name, namespace, "unchanged").Set(float64(unchanged))
}

// Delete removes every metric series for a DriftCheck. Call it when the object
// is gone so stale series do not linger in the registry.
func (r *Recorder) Delete(name, namespace string) {
	if r == nil {
		return
	}
	driftedResources.DeletePartialMatch(prometheus.Labels{"driftcheck": name, "namespace": namespace})
}
