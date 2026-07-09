package controller

import (
	"context"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	myv1 "github.com/somaz94/kube-drift/api/v1alpha1"
	"github.com/somaz94/kube-drift/internal/notify"
)

// recordingNotifier captures the events and URLs it is asked to send.
type recordingNotifier struct {
	events []notify.Event
	urls   []string
	err    error
}

func (n *recordingNotifier) Send(_ context.Context, wh notify.Webhook, ev notify.Event) error {
	n.urls = append(n.urls, wh.URL)
	n.events = append(n.events, ev)
	return n.err
}

func notifyReconciler(scheme *runtime.Scheme, n notify.Notifier, rec record.EventRecorder, objs ...client.Object) *DriftCheckReconciler {
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(&myv1.DriftCheck{}).
		Build()
	return &DriftCheckReconciler{Client: cl, Scheme: scheme, Notifier: n, Recorder: rec}
}

func driftCheckWithNotify(webhooks ...myv1.Webhook) *myv1.DriftCheck {
	dc := newDriftCheck()
	dc.Spec.Notify = &myv1.NotifySpec{Webhooks: webhooks}
	return dc
}

func driftedStatus() myv1.DriftCheckStatus {
	return myv1.DriftCheckStatus{
		Summary: myv1.DriftSummary{Changed: 1, Unchanged: 2},
		DriftedResources: []myv1.DriftedResource{
			{APIVersion: "v1", Kind: "ConfigMap", Name: "app-config", Namespace: "default", Status: myv1.DriftChanged},
		},
	}
}

func TestNotify_NoWebhooks(t *testing.T) {
	scheme := newScheme(t)
	dc := newDriftCheck() // no notify block
	n := &recordingNotifier{}
	r := notifyReconciler(scheme, n, nil, dc)

	if err := r.notify(context.Background(), dc); err != nil {
		t.Fatalf("notify() error = %v", err)
	}
	if len(n.events) != 0 {
		t.Errorf("sent %d notifications, want 0", len(n.events))
	}
}

func TestNotify_SendsOnDriftAndDedups(t *testing.T) {
	scheme := newScheme(t)
	dc := driftCheckWithNotify(myv1.Webhook{Type: myv1.WebhookSlack, URL: "http://hook.example/1"})
	dc.Status = driftedStatus()
	n := &recordingNotifier{}
	r := notifyReconciler(scheme, n, nil, dc)

	// First call: drift is new → sends and stores the hash.
	if err := r.notify(context.Background(), dc); err != nil {
		t.Fatalf("notify() error = %v", err)
	}
	if len(n.events) != 1 {
		t.Fatalf("first notify sent %d, want 1", len(n.events))
	}
	if n.events[0].Resolved {
		t.Error("first event Resolved = true, want false")
	}
	if dc.Status.LastNotifiedHash == "" {
		t.Error("LastNotifiedHash not set after send")
	}

	// Second call with the same drift → deduped, no send.
	if err := r.notify(context.Background(), dc); err != nil {
		t.Fatalf("notify() second error = %v", err)
	}
	if len(n.events) != 1 {
		t.Errorf("second notify sent again (%d total), want dedup", len(n.events))
	}
}

func TestNotify_ReNotifiesWhenDriftSetChanges(t *testing.T) {
	scheme := newScheme(t)
	dc := driftCheckWithNotify(myv1.Webhook{Type: myv1.WebhookGeneric, URL: "http://hook.example/1"})
	dc.Status = driftedStatus()
	n := &recordingNotifier{}
	r := notifyReconciler(scheme, n, nil, dc)

	// First drift set → sends.
	if err := r.notify(context.Background(), dc); err != nil {
		t.Fatalf("notify() error = %v", err)
	}
	// Drift set changes to a different resource → must send again.
	dc.Status.DriftedResources = []myv1.DriftedResource{
		{APIVersion: "v1", Kind: "Service", Name: "web", Namespace: "default", Status: myv1.DriftNew},
	}
	dc.Status.Summary = myv1.DriftSummary{New: 1, Unchanged: 2}
	if err := r.notify(context.Background(), dc); err != nil {
		t.Fatalf("notify() second error = %v", err)
	}
	if len(n.events) != 2 {
		t.Errorf("sent %d, want 2 (drift set changed → re-notify)", len(n.events))
	}
}

func TestNotify_UnchangedTallyDoesNotReNotify(t *testing.T) {
	scheme := newScheme(t)
	dc := driftCheckWithNotify(myv1.Webhook{Type: myv1.WebhookGeneric, URL: "http://hook.example/1"})
	dc.Status = driftedStatus()
	n := &recordingNotifier{}
	r := notifyReconciler(scheme, n, nil, dc)

	if err := r.notify(context.Background(), dc); err != nil {
		t.Fatalf("notify() error = %v", err)
	}
	// Same drifted set, only the unchanged tally moves → must NOT re-notify.
	dc.Status.Summary.Unchanged += 5
	if err := r.notify(context.Background(), dc); err != nil {
		t.Fatalf("notify() second error = %v", err)
	}
	if len(n.events) != 1 {
		t.Errorf("sent %d, want 1 (unchanged tally must not trigger re-notify)", len(n.events))
	}
}

func TestDriftHash_OrderInvariantAndDistinct(t *testing.T) {
	a := myv1.DriftedResource{APIVersion: "v1", Kind: "ConfigMap", Name: "a", Namespace: "default", Status: myv1.DriftChanged}
	b := myv1.DriftedResource{APIVersion: "v1", Kind: "Service", Name: "b", Namespace: "prod", Status: myv1.DriftNew}

	if driftHash([]myv1.DriftedResource{a, b}) != driftHash([]myv1.DriftedResource{b, a}) {
		t.Error("driftHash is order-dependent; must be stable regardless of engine order")
	}
	if driftHash([]myv1.DriftedResource{a}) == driftHash([]myv1.DriftedResource{b}) {
		t.Error("distinct drift sets produced the same hash")
	}
	if empty := driftHash(nil); empty == "" {
		t.Error("empty drift set must hash to a non-empty digest to avoid the sentinel collision")
	}
}

func TestNotify_ResolvedTransition(t *testing.T) {
	scheme := newScheme(t)
	dc := driftCheckWithNotify(myv1.Webhook{Type: myv1.WebhookGeneric, URL: "http://hook.example/1"})
	// Previously drifted (non-empty hash), now clean.
	dc.Status = myv1.DriftCheckStatus{
		Summary:          myv1.DriftSummary{Unchanged: 3},
		LastNotifiedHash: "stale-drift-hash",
	}
	n := &recordingNotifier{}
	r := notifyReconciler(scheme, n, nil, dc)

	if err := r.notify(context.Background(), dc); err != nil {
		t.Fatalf("notify() error = %v", err)
	}
	if len(n.events) != 1 {
		t.Fatalf("sent %d, want 1 resolved notification", len(n.events))
	}
	if !n.events[0].Resolved {
		t.Error("event Resolved = false, want true")
	}
	if len(n.events[0].Drifted) != 0 {
		t.Errorf("resolved event carried %d drifted resources, want 0", len(n.events[0].Drifted))
	}
}

func TestNotify_FreshNoDriftSkips(t *testing.T) {
	scheme := newScheme(t)
	dc := driftCheckWithNotify(myv1.Webhook{Type: myv1.WebhookGeneric, URL: "http://hook.example/1"})
	// Never drifted (empty hash) and currently clean → no non-event announcement.
	dc.Status = myv1.DriftCheckStatus{Summary: myv1.DriftSummary{Unchanged: 3}}
	n := &recordingNotifier{}
	r := notifyReconciler(scheme, n, nil, dc)

	if err := r.notify(context.Background(), dc); err != nil {
		t.Fatalf("notify() error = %v", err)
	}
	if len(n.events) != 0 {
		t.Errorf("fresh in-sync DriftCheck sent %d notifications, want 0", len(n.events))
	}
	if dc.Status.LastNotifiedHash != "" {
		t.Errorf("LastNotifiedHash = %q, want empty", dc.Status.LastNotifiedHash)
	}
}

func TestNotify_SecretRefResolvesURL(t *testing.T) {
	scheme := newScheme(t)
	dc := driftCheckWithNotify(myv1.Webhook{
		Type:         myv1.WebhookSlack,
		URLSecretRef: &myv1.SecretKeyRef{Name: "slack", Key: "url"},
	})
	dc.Status = driftedStatus()
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "slack", Namespace: "default"},
		Data:       map[string][]byte{"url": []byte("  http://hook.example/from-secret  ")},
	}
	n := &recordingNotifier{}
	r := notifyReconciler(scheme, n, nil, dc, secret)

	if err := r.notify(context.Background(), dc); err != nil {
		t.Fatalf("notify() error = %v", err)
	}
	if len(n.urls) != 1 || n.urls[0] != "http://hook.example/from-secret" {
		t.Errorf("resolved urls = %v, want trimmed secret url", n.urls)
	}
}

func TestNotify_SecretMissing_RecordsEventKeepsHash(t *testing.T) {
	scheme := newScheme(t)
	dc := driftCheckWithNotify(myv1.Webhook{
		Type:         myv1.WebhookSlack,
		URLSecretRef: &myv1.SecretKeyRef{Name: "absent", Key: "url"},
	})
	dc.Status = driftedStatus()
	rec := record.NewFakeRecorder(4)
	n := &recordingNotifier{}
	r := notifyReconciler(scheme, n, rec, dc)

	if err := r.notify(context.Background(), dc); err != nil {
		t.Fatalf("notify() error = %v", err)
	}
	if len(n.events) != 0 {
		t.Errorf("sent %d despite unresolved url", len(n.events))
	}
	if dc.Status.LastNotifiedHash != "" {
		t.Error("LastNotifiedHash set despite delivery failure; next reconcile would not retry")
	}
	select {
	case ev := <-rec.Events:
		if ev == "" {
			t.Error("empty event recorded")
		}
	default:
		t.Error("expected a NotifyFailed event")
	}
}

func TestNotify_SendError_KeepsHashForRetry(t *testing.T) {
	scheme := newScheme(t)
	dc := driftCheckWithNotify(myv1.Webhook{Type: myv1.WebhookGeneric, URL: "http://hook.example/1"})
	dc.Status = driftedStatus()
	rec := record.NewFakeRecorder(4)
	n := &recordingNotifier{err: errors.New("boom")}
	r := notifyReconciler(scheme, n, rec, dc)

	if err := r.notify(context.Background(), dc); err != nil {
		t.Fatalf("notify() error = %v, want nil (best-effort)", err)
	}
	if dc.Status.LastNotifiedHash != "" {
		t.Error("LastNotifiedHash set despite send error; retry would be skipped")
	}
}

func TestReconcile_EndToEndNotifies(t *testing.T) {
	scheme := newScheme(t)
	dc := driftCheckWithNotify(myv1.Webhook{Type: myv1.WebhookSlack, URL: "http://hook.example/1"})
	desired := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "desired", Namespace: "default"},
		Data:       map[string]string{"m.yaml": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: brand-new\n  namespace: default\n"},
	}
	n := &recordingNotifier{}
	r := notifyReconciler(scheme, n, record.NewFakeRecorder(4), dc, desired)
	r.Fetcher = &fakeFetcher{} // empty → "new"

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "dc", Namespace: "default"}}
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if len(n.events) != 1 {
		t.Fatalf("Reconcile sent %d notifications, want 1", len(n.events))
	}

	var got myv1.DriftCheck
	if err := r.Get(context.Background(), types.NamespacedName{Name: "dc", Namespace: "default"}, &got); err != nil {
		t.Fatal(err)
	}
	if got.Status.LastNotifiedHash == "" {
		t.Error("LastNotifiedHash not persisted after Reconcile")
	}
}
