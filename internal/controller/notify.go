package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"

	driftv1alpha1 "github.com/somaz94/kube-drift/api/v1alpha1"
	"github.com/somaz94/kube-drift/internal/notify"
)

// notify delivers a drift notification to the configured webhooks when the
// drift set has changed since the last delivery. It is a no-op when no webhooks
// are configured.
//
// Delivery is best-effort with at-least-once semantics: a webhook that errors
// is logged and surfaced as an event, and LastNotifiedHash is left unchanged so
// the next reconcile retries. Because dedup is tracked by a single hash (not
// per-webhook), a retry re-sends to every webhook — including ones that already
// succeeded. Likewise, if the hash-persisting status write fails after a
// successful send, the next reconcile re-sends. Only a status-write failure is
// returned, so controller-runtime retries promptly instead of losing the
// fingerprint.
func (r *DriftCheckReconciler) notify(ctx context.Context, dc *driftv1alpha1.DriftCheck) error {
	if dc.Spec.Notify == nil || len(dc.Spec.Notify.Webhooks) == 0 {
		return nil
	}
	logger := log.FromContext(ctx)

	hash := driftHash(dc.Status.DriftedResources)
	if hash == dc.Status.LastNotifiedHash {
		// Drift state unchanged since the last notification.
		return nil
	}
	resolved := driftCount(dc.Status.Summary) == 0
	if resolved && dc.Status.LastNotifiedHash == "" {
		// A fresh, in-sync DriftCheck has never drifted: do not announce a
		// "resolved" non-event. LastNotifiedHash stays empty until real drift.
		return nil
	}

	ev := buildEvent(dc, resolved)
	notifier := r.Notifier
	if notifier == nil {
		notifier = notify.NewSender()
	}

	failed := false
	for i := range dc.Spec.Notify.Webhooks {
		wh := &dc.Spec.Notify.Webhooks[i]
		url, err := r.resolveWebhookURL(ctx, dc.Namespace, wh)
		if err != nil {
			failed = true
			logger.Error(err, "resolve webhook url", "type", wh.Type)
			r.event(dc, corev1.EventTypeWarning, "NotifyFailed", err.Error())
			continue
		}
		target := notify.Webhook{Type: notify.Type(wh.Type), URL: url}
		if err := notifier.Send(ctx, target, ev); err != nil {
			failed = true
			logger.Error(err, "send notification", "type", wh.Type)
			r.event(dc, corev1.EventTypeWarning, "NotifyFailed", err.Error())
			continue
		}
	}

	if failed {
		// Keep LastNotifiedHash so the next reconcile retries the delivery.
		return nil
	}

	dc.Status.LastNotifiedHash = hash
	return r.Status().Update(ctx, dc)
}

// resolveWebhookURL returns the webhook endpoint, dereferencing a Secret when
// URLSecretRef is set (it takes precedence over an inline URL).
func (r *DriftCheckReconciler) resolveWebhookURL(ctx context.Context, ns string, wh *driftv1alpha1.Webhook) (string, error) {
	if ref := wh.URLSecretRef; ref != nil {
		var sec corev1.Secret
		if err := r.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: ns}, &sec); err != nil {
			return "", fmt.Errorf("get Secret %s/%s: %w", ns, ref.Name, err)
		}
		v, ok := sec.Data[ref.Key]
		if !ok {
			return "", fmt.Errorf("key %q not found in Secret %s/%s", ref.Key, ns, ref.Name)
		}
		url := strings.TrimSpace(string(v))
		if url == "" {
			return "", fmt.Errorf("Secret %s/%s key %q is empty", ns, ref.Name, ref.Key)
		}
		return url, nil
	}
	if wh.URL == "" {
		return "", fmt.Errorf("webhook has neither url nor urlSecretRef")
	}
	return wh.URL, nil
}

// event records a Kubernetes event when a recorder is wired.
func (r *DriftCheckReconciler) event(dc *driftv1alpha1.DriftCheck, eventType, reason, message string) {
	if r.Recorder != nil {
		r.Recorder.Event(dc, eventType, reason, message)
	}
}

// buildEvent maps a DriftCheck's status into a notify.Event. Drifted resources
// are omitted when resolved.
func buildEvent(dc *driftv1alpha1.DriftCheck, resolved bool) notify.Event {
	ev := notify.Event{
		Name:      dc.Name,
		Namespace: dc.Namespace,
		Resolved:  resolved,
		Summary: notify.Summary{
			Changed:   dc.Status.Summary.Changed,
			New:       dc.Status.Summary.New,
			Deleted:   dc.Status.Summary.Deleted,
			Unchanged: dc.Status.Summary.Unchanged,
		},
	}
	if !resolved {
		ev.Drifted = make([]notify.Resource, 0, len(dc.Status.DriftedResources))
		for _, d := range dc.Status.DriftedResources {
			ev.Drifted = append(ev.Drifted, notify.Resource{
				APIVersion: d.APIVersion,
				Kind:       d.Kind,
				Name:       d.Name,
				Namespace:  d.Namespace,
				Status:     string(d.Status),
			})
		}
	}
	return ev
}

// driftCount returns the number of resources that differ from desired state.
func driftCount(s driftv1alpha1.DriftSummary) int {
	return s.Changed + s.New + s.Deleted
}

// driftHash fingerprints the drift set so notifications fire only when it
// changes. Only the drifted resources are hashed — the summary counts are fully
// derived from this list, so including them (in particular the unrelated
// Unchanged tally) would trigger spurious re-notifications when a matching
// resource is added or removed while the drift set is identical. Resources are
// sorted first so the fingerprint is independent of the order the comparison
// engine returns them in. An empty list hashes to a fixed non-empty digest, so
// it never collides with the empty LastNotifiedHash sentinel.
func driftHash(drifted []driftv1alpha1.DriftedResource) string {
	keys := make([]string, len(drifted))
	for i, d := range drifted {
		keys[i] = fmt.Sprintf("%s|%s|%s|%s|%s", d.APIVersion, d.Kind, d.Namespace, d.Name, d.Status)
	}
	sort.Strings(keys)

	h := sha256.New()
	for _, k := range keys {
		_, _ = io.WriteString(h, k)
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}
