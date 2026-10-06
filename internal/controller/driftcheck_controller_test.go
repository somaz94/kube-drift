package controller

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	myv1 "github.com/somaz94/kube-drift/api/v1alpha1"
	"github.com/somaz94/kube-drift/internal/metrics"
	driftsource "github.com/somaz94/kube-drift/internal/source"
)

// reasonSourceError is the Ready-condition Reason the reconciler records when
// source resolution fails permanently (see permanentFail in
// driftcheck_controller.go).
const reasonSourceError = "SourceError"

// fakeFetcher implements kube-diff's cluster.ResourceFetcher. A resource keyed
// by name is returned as-is; err, when set, is returned for every lookup;
// anything else is NotFound (→ "new").
type fakeFetcher struct {
	objs map[string]*unstructured.Unstructured
	err  error
}

func (f *fakeFetcher) Get(_ context.Context, _, kind, _, name string) (*unstructured.Unstructured, error) {
	if f.err != nil {
		return nil, f.err
	}
	if obj, ok := f.objs[name]; ok {
		return obj, nil
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{Resource: kind}, name)
}

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("clientgoscheme: %v", err)
	}
	if err := myv1.AddToScheme(scheme); err != nil {
		t.Fatalf("myv1 scheme: %v", err)
	}
	return scheme
}

func TestReconcile_NotFound(t *testing.T) {
	scheme := newScheme(t)
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	r := &DriftCheckReconciler{Client: cl, Scheme: scheme}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "nonexistent", Namespace: nsDefault},
	})
	if err != nil {
		t.Errorf("Reconcile() error = %v, want nil for not found", err)
	}
}

func newDriftCheck() *myv1.DriftCheck {
	return &myv1.DriftCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "dc", Namespace: nsDefault},
		Spec: myv1.DriftCheckSpec{
			Source: myv1.Source{
				Type:      myv1.SourceTypeConfigMap,
				ConfigMap: &myv1.ConfigMapSource{Name: nameDesired},
			},
			Interval: metav1.Duration{Duration: 5 * time.Minute},
		},
	}
}

func reconcilerFor(scheme *runtime.Scheme, fetcher *fakeFetcher, objs ...client.Object) *DriftCheckReconciler {
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(&myv1.DriftCheck{}).
		Build()
	return &DriftCheckReconciler{Client: cl, Scheme: scheme, Fetcher: fetcher, Metrics: metrics.NewRecorder()}
}

func TestReconcile_MissingConfigMap_SetsNotReady(t *testing.T) {
	scheme := newScheme(t)
	dc := newDriftCheck()
	r := reconcilerFor(scheme, &fakeFetcher{}, dc)

	res, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dc", Namespace: nsDefault},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v, want nil", err)
	}
	if res.RequeueAfter != 5*time.Minute {
		t.Errorf("RequeueAfter = %v, want 5m", res.RequeueAfter)
	}

	var got myv1.DriftCheck
	if err := r.Get(context.Background(), types.NamespacedName{Name: "dc", Namespace: nsDefault}, &got); err != nil {
		t.Fatal(err)
	}
	cond := got.Status.Conditions
	if len(cond) != 1 || cond[0].Status != metav1.ConditionFalse || cond[0].Reason != reasonSourceError {
		t.Errorf("expected a False/SourceError condition, got %+v", cond)
	}
}

func TestReconcile_DetectsDrift(t *testing.T) {
	scheme := newScheme(t)
	dc := newDriftCheck()

	// Desired manifests in the ConfigMap: one existing (will differ) + one absent (new).
	desired := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: nameDesired, Namespace: nsDefault},
		Data: map[string]string{
			"manifests.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
  namespace: default
data:
  key: desired
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: brand-new
  namespace: default
`,
		},
	}

	// Live cluster: app-config exists with a different value → changed; brand-new absent → new.
	live := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": kindConfigMap,
		"metadata": map[string]interface{}{"name": nameAppConfig, "namespace": nsDefault},
		"data":     map[string]interface{}{"key": "live"},
	}}
	fetcher := &fakeFetcher{objs: map[string]*unstructured.Unstructured{nameAppConfig: live}}

	r := reconcilerFor(scheme, fetcher, dc, desired)

	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dc", Namespace: nsDefault},
	}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var got myv1.DriftCheck
	if err := r.Get(context.Background(), types.NamespacedName{Name: "dc", Namespace: nsDefault}, &got); err != nil {
		t.Fatal(err)
	}
	if got.Status.Summary.Changed != 1 || got.Status.Summary.New != 1 {
		t.Errorf("summary = %+v, want changed=1 new=1", got.Status.Summary)
	}
	if len(got.Status.DriftedResources) != 2 {
		t.Errorf("driftedResources = %d, want 2: %+v", len(got.Status.DriftedResources), got.Status.DriftedResources)
	}
	if got.Status.LastCheckedAt == nil {
		t.Error("LastCheckedAt not set")
	}
	if got.Status.ObservedGeneration != got.Generation {
		t.Errorf("ObservedGeneration = %d, want %d", got.Status.ObservedGeneration, got.Generation)
	}
}

func TestReconcile_GitSourceDetectsDrift(t *testing.T) {
	scheme := newScheme(t)
	dc := &myv1.DriftCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "dc", Namespace: nsDefault},
		Spec: myv1.DriftCheckSpec{
			Source: myv1.Source{
				Type: myv1.SourceTypeGit,
				Git:  &myv1.GitSource{URL: testRepoURL, Ref: "main", Path: "manifests"},
			},
		},
	}

	// Live cluster is empty, so the desired ConfigMap surfaces as "new".
	r := reconcilerFor(scheme, &fakeFetcher{}, dc)
	r.GitCloner = func(_ context.Context, dir, _, _ string, _ *driftsource.GitAuth) error {
		sub := filepath.Join(dir, "manifests")
		if err := os.MkdirAll(sub, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(sub, "cm.yaml"),
			[]byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: from-git\n  namespace: default\n"), 0o644)
	}

	res, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dc", Namespace: nsDefault},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v, want nil", err)
	}
	if res.RequeueAfter != 5*time.Minute {
		t.Errorf("RequeueAfter = %v, want 5m (default)", res.RequeueAfter)
	}

	var got myv1.DriftCheck
	if err := r.Get(context.Background(), types.NamespacedName{Name: "dc", Namespace: nsDefault}, &got); err != nil {
		t.Fatal(err)
	}
	if got.Status.Summary.New != 1 {
		t.Errorf("summary = %+v, want new=1", got.Status.Summary)
	}
	if len(got.Status.Conditions) != 1 || got.Status.Conditions[0].Status != metav1.ConditionTrue {
		t.Errorf("expected a True Ready condition, got %+v", got.Status.Conditions)
	}
}

func writeChartAt(t *testing.T, base string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(base, "templates"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "Chart.yaml"),
		[]byte("apiVersion: v2\nname: demo\nversion: 0.1.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "templates", "cm.yaml"),
		[]byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: {{ .Release.Name }}-cm\n  namespace: default\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReconcile_HelmSourceDetectsDrift(t *testing.T) {
	scheme := newScheme(t)
	dc := &myv1.DriftCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "dc", Namespace: nsDefault},
		Spec: myv1.DriftCheckSpec{
			Source: myv1.Source{
				Type: myv1.SourceTypeHelm,
				Helm: &myv1.HelmSource{
					Git:         myv1.GitSource{URL: testRepoURL, Path: "chart"},
					ReleaseName: "rel",
				},
			},
		},
	}
	r := reconcilerFor(scheme, &fakeFetcher{}, dc) // live empty → rendered CM is "new"
	r.GitCloner = func(_ context.Context, dir, _, _ string, _ *driftsource.GitAuth) error {
		writeChartAt(t, filepath.Join(dir, "chart"))
		return nil
	}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dc", Namespace: nsDefault},
	}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	var got myv1.DriftCheck
	_ = r.Get(context.Background(), types.NamespacedName{Name: "dc", Namespace: nsDefault}, &got)
	if got.Status.Summary.New != 1 {
		t.Errorf("summary = %+v, want new=1 (rel-cm)", got.Status.Summary)
	}
}

func TestReconcile_KustomizeSourceDetectsDrift(t *testing.T) {
	scheme := newScheme(t)
	dc := &myv1.DriftCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "dc", Namespace: nsDefault},
		Spec: myv1.DriftCheckSpec{
			Source: myv1.Source{
				Type:      myv1.SourceTypeKustomize,
				Kustomize: &myv1.KustomizeSource{Git: myv1.GitSource{URL: testRepoURL, Path: "overlay"}},
			},
		},
	}
	r := reconcilerFor(scheme, &fakeFetcher{}, dc)
	r.GitCloner = func(_ context.Context, dir, _, _ string, _ *driftsource.GitAuth) error {
		base := filepath.Join(dir, "overlay")
		if err := os.MkdirAll(base, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(base, "kustomization.yaml"),
			[]byte("resources:\n- cm.yaml\nnamePrefix: prod-\n"), 0o644); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(base, "cm.yaml"),
			[]byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: config\n  namespace: default\n"), 0o644)
	}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dc", Namespace: nsDefault},
	}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	var got myv1.DriftCheck
	_ = r.Get(context.Background(), types.NamespacedName{Name: "dc", Namespace: nsDefault}, &got)
	if got.Status.Summary.New != 1 {
		t.Errorf("summary = %+v, want new=1 (prod-config)", got.Status.Summary)
	}
}

func TestReconcile_HelmMissingBlock(t *testing.T) {
	scheme := newScheme(t)
	dc := &myv1.DriftCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "dc", Namespace: nsDefault},
		Spec:       myv1.DriftCheckSpec{Source: myv1.Source{Type: myv1.SourceTypeHelm}},
	}
	r := reconcilerFor(scheme, &fakeFetcher{}, dc)
	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dc", Namespace: nsDefault},
	}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	var got myv1.DriftCheck
	_ = r.Get(context.Background(), types.NamespacedName{Name: "dc", Namespace: nsDefault}, &got)
	if len(got.Status.Conditions) != 1 || got.Status.Conditions[0].Reason != reasonSourceError {
		t.Errorf("expected SourceError condition, got %+v", got.Status.Conditions)
	}
}

func TestReconcile_KustomizeMissingBlock(t *testing.T) {
	scheme := newScheme(t)
	dc := &myv1.DriftCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "dc", Namespace: nsDefault},
		Spec:       myv1.DriftCheckSpec{Source: myv1.Source{Type: myv1.SourceTypeKustomize}},
	}
	r := reconcilerFor(scheme, &fakeFetcher{}, dc)
	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dc", Namespace: nsDefault},
	}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	var got myv1.DriftCheck
	_ = r.Get(context.Background(), types.NamespacedName{Name: "dc", Namespace: nsDefault}, &got)
	if len(got.Status.Conditions) != 1 || got.Status.Conditions[0].Reason != reasonSourceError {
		t.Errorf("expected SourceError condition, got %+v", got.Status.Conditions)
	}
}

func TestReconcile_GitSourceMissingGitBlock(t *testing.T) {
	scheme := newScheme(t)
	// Type is Git but the git block is absent → source resolution fails.
	dc := &myv1.DriftCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "dc", Namespace: nsDefault},
		Spec:       myv1.DriftCheckSpec{Source: myv1.Source{Type: myv1.SourceTypeGit}},
	}
	r := reconcilerFor(scheme, &fakeFetcher{}, dc)

	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dc", Namespace: nsDefault},
	}); err != nil {
		t.Fatalf("Reconcile() error = %v, want nil", err)
	}
	var got myv1.DriftCheck
	_ = r.Get(context.Background(), types.NamespacedName{Name: "dc", Namespace: nsDefault}, &got)
	if len(got.Status.Conditions) != 1 || got.Status.Conditions[0].Reason != reasonSourceError {
		t.Errorf("expected SourceError condition, got %+v", got.Status.Conditions)
	}
}

func TestReconcile_GitSourceMissingURL(t *testing.T) {
	scheme := newScheme(t)
	// Type is Git with a git block but no URL → source resolution fails
	// permanently rather than looping on clone.
	dc := &myv1.DriftCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "dc", Namespace: nsDefault},
		Spec:       myv1.DriftCheckSpec{Source: myv1.Source{Type: myv1.SourceTypeGit, Git: &myv1.GitSource{}}},
	}
	r := reconcilerFor(scheme, &fakeFetcher{}, dc)

	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dc", Namespace: nsDefault},
	}); err != nil {
		t.Fatalf("Reconcile() error = %v, want nil", err)
	}
	var got myv1.DriftCheck
	_ = r.Get(context.Background(), types.NamespacedName{Name: "dc", Namespace: nsDefault}, &got)
	if len(got.Status.Conditions) != 1 || got.Status.Conditions[0].Reason != reasonSourceError {
		t.Errorf("expected SourceError condition, got %+v", got.Status.Conditions)
	}
}

func TestReconcile_NoFetcher(t *testing.T) {
	scheme := newScheme(t)
	dc := newDriftCheck()
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: nameDesired, Namespace: nsDefault},
		Data:       map[string]string{"m.yaml": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: x\n"},
	}
	// Reconciler built WITHOUT a fetcher.
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dc, cm).
		WithStatusSubresource(&myv1.DriftCheck{}).Build()
	r := &DriftCheckReconciler{Client: cl, Scheme: scheme}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dc", Namespace: nsDefault},
	}); err != nil {
		t.Fatalf("Reconcile() error = %v, want nil", err)
	}
	var got myv1.DriftCheck
	_ = r.Get(context.Background(), types.NamespacedName{Name: "dc", Namespace: nsDefault}, &got)
	if len(got.Status.Conditions) != 1 || got.Status.Conditions[0].Reason != "NoFetcher" {
		t.Errorf("expected NoFetcher condition, got %+v", got.Status.Conditions)
	}
}

func TestReconcile_UnknownSourceType(t *testing.T) {
	scheme := newScheme(t)
	dc := &myv1.DriftCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "dc", Namespace: nsDefault},
		Spec:       myv1.DriftCheckSpec{Source: myv1.Source{Type: "Bogus"}},
	}
	r := reconcilerFor(scheme, &fakeFetcher{}, dc)

	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dc", Namespace: nsDefault},
	}); err != nil {
		t.Fatalf("Reconcile() error = %v, want nil", err)
	}
	var got myv1.DriftCheck
	_ = r.Get(context.Background(), types.NamespacedName{Name: "dc", Namespace: nsDefault}, &got)
	if len(got.Status.Conditions) != 1 || got.Status.Conditions[0].Reason != reasonSourceError {
		t.Errorf("expected SourceError condition, got %+v", got.Status.Conditions)
	}
}

func TestConfigMapManifests(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cm", Namespace: nsDefault},
		Data:       map[string]string{"b.yaml": "kind: B", "a.yaml": "kind: A"},
	}

	// No key → concatenated in sorted key order (a before b).
	all, err := configMapManifests(cm, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := string(all); got != "kind: A\n---\nkind: B" {
		t.Errorf("concat = %q", got)
	}

	// Specific key.
	one, err := configMapManifests(cm, "a.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(one) != "kind: A" {
		t.Errorf("key select = %q", string(one))
	}

	// Missing key → error.
	if _, err := configMapManifests(cm, "missing"); err == nil {
		t.Error("expected error for missing key, got nil")
	}
}

func TestConfigMapManifests_EmptyKeyedEntry(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cm", Namespace: nsDefault},
		Data:       map[string]string{"blank.yaml": " \n"},
		BinaryData: map[string][]byte{"empty.bin": {}},
	}
	for _, key := range []string{"blank.yaml", "empty.bin"} {
		if _, err := configMapManifests(cm, key); err == nil {
			t.Errorf("configMapManifests(%q): expected error for empty entry, got nil", key)
		}
	}
}

func TestReconcile_TargetNarrowsComparison(t *testing.T) {
	desired := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: nameDesired, Namespace: nsDefault},
		Data: map[string]string{"m.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: in-default-web
  namespace: default
  labels: {tier: web}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: in-default-db
  namespace: default
  labels: {tier: db}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: in-other-web
  namespace: other
  labels: {tier: web}
`},
	}

	tests := []struct {
		name   string
		target myv1.Target
		want   int
	}{
		{"empty target compares everything", myv1.Target{}, 3},
		{"namespaces", myv1.Target{Namespaces: []string{nsDefault}}, 2},
		{"labelSelector", myv1.Target{LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"tier": "web"}}}, 2},
		{"both", myv1.Target{
			Namespaces:    []string{nsDefault},
			LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"tier": "web"}},
		}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dc := newDriftCheck()
			dc.Spec.Target = tt.target
			r := reconcilerFor(newScheme(t), &fakeFetcher{}, dc, desired.DeepCopy())
			if _, err := r.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: types.NamespacedName{Name: "dc", Namespace: nsDefault},
			}); err != nil {
				t.Fatalf("Reconcile() error = %v", err)
			}
			var got myv1.DriftCheck
			if err := r.Get(context.Background(), types.NamespacedName{Name: "dc", Namespace: nsDefault}, &got); err != nil {
				t.Fatal(err)
			}
			if got.Status.Summary.New != tt.want {
				t.Errorf("summary.new = %d, want %d (%+v)", got.Status.Summary.New, tt.want, got.Status.DriftedResources)
			}
		})
	}
}

func TestReconcile_FetchErrors(t *testing.T) {
	gr := schema.GroupResource{Resource: "configmaps"}
	tests := []struct {
		name      string
		err       error
		reason    string
		permanent bool
	}{
		{"forbidden waits for the interval", apierrors.NewForbidden(gr, "x", errors.New("no rbac")), "FetchError", true},
		{"unauthorized waits for the interval", apierrors.NewUnauthorized("token expired"), "FetchError", true},
		{"timeout backs off", apierrors.NewServerTimeout(gr, "get", 1), "CompareError", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desired := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: nameDesired, Namespace: nsDefault},
				Data:       map[string]string{"m.yaml": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: x\n  namespace: default\n"},
			}
			r := reconcilerFor(newScheme(t), &fakeFetcher{err: tt.err}, newDriftCheck(), desired)

			res, err := r.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: types.NamespacedName{Name: "dc", Namespace: nsDefault},
			})
			if tt.permanent {
				if err != nil || res.RequeueAfter != 5*time.Minute {
					t.Errorf("Reconcile() = %+v, %v; want RequeueAfter=5m, nil", res, err)
				}
			} else if err == nil {
				t.Error("Reconcile() error = nil, want the fetch error returned for backoff")
			}
			var got myv1.DriftCheck
			if err := r.Get(context.Background(), types.NamespacedName{Name: "dc", Namespace: nsDefault}, &got); err != nil {
				t.Fatal(err)
			}
			if c := got.Status.Conditions; len(c) != 1 || c[0].Reason != tt.reason {
				t.Errorf("conditions = %+v, want a single %s condition", c, tt.reason)
			}
		})
	}
}

func TestReconcile_InvalidTargetSelector(t *testing.T) {
	dc := newDriftCheck()
	dc.Spec.Target.LabelSelector = &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
		{Key: "tier", Operator: "Bogus", Values: []string{"web"}},
	}}
	desired := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: nameDesired, Namespace: nsDefault},
		Data:       map[string]string{"m.yaml": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: x\n  namespace: default\n"},
	}
	r := reconcilerFor(newScheme(t), &fakeFetcher{}, dc, desired)

	res, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dc", Namespace: nsDefault},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v, want nil (config errors retry on the interval)", err)
	}
	if res.RequeueAfter != 5*time.Minute {
		t.Errorf("RequeueAfter = %v, want 5m", res.RequeueAfter)
	}
	var got myv1.DriftCheck
	if err := r.Get(context.Background(), types.NamespacedName{Name: "dc", Namespace: nsDefault}, &got); err != nil {
		t.Fatal(err)
	}
	if c := got.Status.Conditions; len(c) != 1 || c[0].Reason != "InvalidTarget" {
		t.Errorf("conditions = %+v, want a single InvalidTarget condition", c)
	}
}
