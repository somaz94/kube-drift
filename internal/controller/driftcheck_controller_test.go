package controller

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	myv1 "github.com/somaz94/kube-drift/api/v1alpha1"
	"github.com/somaz94/kube-drift/internal/metrics"
)

// fakeFetcher implements kube-diff's cluster.ResourceFetcher. A resource keyed
// by name is returned as-is; anything else returns notFound (→ "new").
type fakeFetcher struct {
	objs map[string]*unstructured.Unstructured
}

func (f *fakeFetcher) Get(_ context.Context, _, _, _, name string) (*unstructured.Unstructured, error) {
	if obj, ok := f.objs[name]; ok {
		return obj, nil
	}
	return nil, errors.New("not found")
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
		NamespacedName: types.NamespacedName{Name: "nonexistent", Namespace: "default"},
	})
	if err != nil {
		t.Errorf("Reconcile() error = %v, want nil for not found", err)
	}
}

func newDriftCheck() *myv1.DriftCheck {
	return &myv1.DriftCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "dc", Namespace: "default"},
		Spec: myv1.DriftCheckSpec{
			Source: myv1.Source{
				Type:      myv1.SourceTypeConfigMap,
				ConfigMap: &myv1.ConfigMapSource{Name: "desired"},
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
		NamespacedName: types.NamespacedName{Name: "dc", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v, want nil", err)
	}
	if res.RequeueAfter != 5*time.Minute {
		t.Errorf("RequeueAfter = %v, want 5m", res.RequeueAfter)
	}

	var got myv1.DriftCheck
	if err := r.Get(context.Background(), types.NamespacedName{Name: "dc", Namespace: "default"}, &got); err != nil {
		t.Fatal(err)
	}
	cond := got.Status.Conditions
	if len(cond) != 1 || cond[0].Status != metav1.ConditionFalse || cond[0].Reason != "SourceError" {
		t.Errorf("expected a False/SourceError condition, got %+v", cond)
	}
}

func TestReconcile_DetectsDrift(t *testing.T) {
	scheme := newScheme(t)
	dc := newDriftCheck()

	// Desired manifests in the ConfigMap: one existing (will differ) + one absent (new).
	desired := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "desired", Namespace: "default"},
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
		"apiVersion": "v1", "kind": "ConfigMap",
		"metadata": map[string]interface{}{"name": "app-config", "namespace": "default"},
		"data":     map[string]interface{}{"key": "live"},
	}}
	fetcher := &fakeFetcher{objs: map[string]*unstructured.Unstructured{"app-config": live}}

	r := reconcilerFor(scheme, fetcher, dc, desired)

	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dc", Namespace: "default"},
	}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var got myv1.DriftCheck
	if err := r.Get(context.Background(), types.NamespacedName{Name: "dc", Namespace: "default"}, &got); err != nil {
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
		ObjectMeta: metav1.ObjectMeta{Name: "dc", Namespace: "default"},
		Spec: myv1.DriftCheckSpec{
			Source: myv1.Source{
				Type: myv1.SourceTypeGit,
				Git:  &myv1.GitSource{URL: "https://example.com/repo.git", Ref: "main", Path: "manifests"},
			},
		},
	}

	// Live cluster is empty, so the desired ConfigMap surfaces as "new".
	r := reconcilerFor(scheme, &fakeFetcher{}, dc)
	r.GitCloner = func(_ context.Context, dir, _, _ string) error {
		sub := filepath.Join(dir, "manifests")
		if err := os.MkdirAll(sub, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(sub, "cm.yaml"),
			[]byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: from-git\n  namespace: default\n"), 0o644)
	}

	res, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dc", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v, want nil", err)
	}
	if res.RequeueAfter != 5*time.Minute {
		t.Errorf("RequeueAfter = %v, want 5m (default)", res.RequeueAfter)
	}

	var got myv1.DriftCheck
	if err := r.Get(context.Background(), types.NamespacedName{Name: "dc", Namespace: "default"}, &got); err != nil {
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
		ObjectMeta: metav1.ObjectMeta{Name: "dc", Namespace: "default"},
		Spec: myv1.DriftCheckSpec{
			Source: myv1.Source{
				Type: myv1.SourceTypeHelm,
				Helm: &myv1.HelmSource{
					Git:         myv1.GitSource{URL: "https://example.com/repo.git", Path: "chart"},
					ReleaseName: "rel",
				},
			},
		},
	}
	r := reconcilerFor(scheme, &fakeFetcher{}, dc) // live empty → rendered CM is "new"
	r.GitCloner = func(_ context.Context, dir, _, _ string) error {
		writeChartAt(t, filepath.Join(dir, "chart"))
		return nil
	}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dc", Namespace: "default"},
	}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	var got myv1.DriftCheck
	_ = r.Get(context.Background(), types.NamespacedName{Name: "dc", Namespace: "default"}, &got)
	if got.Status.Summary.New != 1 {
		t.Errorf("summary = %+v, want new=1 (rel-cm)", got.Status.Summary)
	}
}

func TestReconcile_KustomizeSourceDetectsDrift(t *testing.T) {
	scheme := newScheme(t)
	dc := &myv1.DriftCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "dc", Namespace: "default"},
		Spec: myv1.DriftCheckSpec{
			Source: myv1.Source{
				Type:      myv1.SourceTypeKustomize,
				Kustomize: &myv1.KustomizeSource{Git: myv1.GitSource{URL: "https://example.com/repo.git", Path: "overlay"}},
			},
		},
	}
	r := reconcilerFor(scheme, &fakeFetcher{}, dc)
	r.GitCloner = func(_ context.Context, dir, _, _ string) error {
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
		NamespacedName: types.NamespacedName{Name: "dc", Namespace: "default"},
	}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	var got myv1.DriftCheck
	_ = r.Get(context.Background(), types.NamespacedName{Name: "dc", Namespace: "default"}, &got)
	if got.Status.Summary.New != 1 {
		t.Errorf("summary = %+v, want new=1 (prod-config)", got.Status.Summary)
	}
}

func TestReconcile_HelmMissingBlock(t *testing.T) {
	scheme := newScheme(t)
	dc := &myv1.DriftCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "dc", Namespace: "default"},
		Spec:       myv1.DriftCheckSpec{Source: myv1.Source{Type: myv1.SourceTypeHelm}},
	}
	r := reconcilerFor(scheme, &fakeFetcher{}, dc)
	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dc", Namespace: "default"},
	}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	var got myv1.DriftCheck
	_ = r.Get(context.Background(), types.NamespacedName{Name: "dc", Namespace: "default"}, &got)
	if len(got.Status.Conditions) != 1 || got.Status.Conditions[0].Reason != "SourceError" {
		t.Errorf("expected SourceError condition, got %+v", got.Status.Conditions)
	}
}

func TestReconcile_KustomizeMissingBlock(t *testing.T) {
	scheme := newScheme(t)
	dc := &myv1.DriftCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "dc", Namespace: "default"},
		Spec:       myv1.DriftCheckSpec{Source: myv1.Source{Type: myv1.SourceTypeKustomize}},
	}
	r := reconcilerFor(scheme, &fakeFetcher{}, dc)
	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dc", Namespace: "default"},
	}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	var got myv1.DriftCheck
	_ = r.Get(context.Background(), types.NamespacedName{Name: "dc", Namespace: "default"}, &got)
	if len(got.Status.Conditions) != 1 || got.Status.Conditions[0].Reason != "SourceError" {
		t.Errorf("expected SourceError condition, got %+v", got.Status.Conditions)
	}
}

func TestReconcile_GitSourceMissingGitBlock(t *testing.T) {
	scheme := newScheme(t)
	// Type is Git but the git block is absent → source resolution fails.
	dc := &myv1.DriftCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "dc", Namespace: "default"},
		Spec:       myv1.DriftCheckSpec{Source: myv1.Source{Type: myv1.SourceTypeGit}},
	}
	r := reconcilerFor(scheme, &fakeFetcher{}, dc)

	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dc", Namespace: "default"},
	}); err != nil {
		t.Fatalf("Reconcile() error = %v, want nil", err)
	}
	var got myv1.DriftCheck
	_ = r.Get(context.Background(), types.NamespacedName{Name: "dc", Namespace: "default"}, &got)
	if len(got.Status.Conditions) != 1 || got.Status.Conditions[0].Reason != "SourceError" {
		t.Errorf("expected SourceError condition, got %+v", got.Status.Conditions)
	}
}

func TestReconcile_GitSourceMissingURL(t *testing.T) {
	scheme := newScheme(t)
	// Type is Git with a git block but no URL → source resolution fails
	// permanently rather than looping on clone.
	dc := &myv1.DriftCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "dc", Namespace: "default"},
		Spec:       myv1.DriftCheckSpec{Source: myv1.Source{Type: myv1.SourceTypeGit, Git: &myv1.GitSource{}}},
	}
	r := reconcilerFor(scheme, &fakeFetcher{}, dc)

	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dc", Namespace: "default"},
	}); err != nil {
		t.Fatalf("Reconcile() error = %v, want nil", err)
	}
	var got myv1.DriftCheck
	_ = r.Get(context.Background(), types.NamespacedName{Name: "dc", Namespace: "default"}, &got)
	if len(got.Status.Conditions) != 1 || got.Status.Conditions[0].Reason != "SourceError" {
		t.Errorf("expected SourceError condition, got %+v", got.Status.Conditions)
	}
}

func TestReconcile_NoFetcher(t *testing.T) {
	scheme := newScheme(t)
	dc := newDriftCheck()
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "desired", Namespace: "default"},
		Data:       map[string]string{"m.yaml": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: x\n"},
	}
	// Reconciler built WITHOUT a fetcher.
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dc, cm).
		WithStatusSubresource(&myv1.DriftCheck{}).Build()
	r := &DriftCheckReconciler{Client: cl, Scheme: scheme}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dc", Namespace: "default"},
	}); err != nil {
		t.Fatalf("Reconcile() error = %v, want nil", err)
	}
	var got myv1.DriftCheck
	_ = r.Get(context.Background(), types.NamespacedName{Name: "dc", Namespace: "default"}, &got)
	if len(got.Status.Conditions) != 1 || got.Status.Conditions[0].Reason != "NoFetcher" {
		t.Errorf("expected NoFetcher condition, got %+v", got.Status.Conditions)
	}
}

func TestReconcile_UnknownSourceType(t *testing.T) {
	scheme := newScheme(t)
	dc := &myv1.DriftCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "dc", Namespace: "default"},
		Spec:       myv1.DriftCheckSpec{Source: myv1.Source{Type: "Bogus"}},
	}
	r := reconcilerFor(scheme, &fakeFetcher{}, dc)

	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dc", Namespace: "default"},
	}); err != nil {
		t.Fatalf("Reconcile() error = %v, want nil", err)
	}
	var got myv1.DriftCheck
	_ = r.Get(context.Background(), types.NamespacedName{Name: "dc", Namespace: "default"}, &got)
	if len(got.Status.Conditions) != 1 || got.Status.Conditions[0].Reason != "SourceError" {
		t.Errorf("expected SourceError condition, got %+v", got.Status.Conditions)
	}
}

func TestConfigMapManifests(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cm", Namespace: "default"},
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
