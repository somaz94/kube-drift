package source

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"helm.sh/helm/v3/pkg/chart"
)

// helmCloneWritingChart returns a fake cloner that writes a minimal chart into
// the "chart" sub-path of the checkout, so DependencyBuild tests can render.
func helmCloneWritingChart(t *testing.T) CloneFunc {
	return func(_ context.Context, dir, _, _ string, _ *GitAuth) error {
		writeChart(t, filepath.Join(dir, "chart"))
		return nil
	}
}

func TestHelmSource_DependencyBuildInvoked(t *testing.T) {
	called := false
	h := NewHelmSource(context.Background(), "u", "", "chart", "rel", "ns", nil, nil, true, nil, helmCloneWritingChart(t))
	h.buildDeps = func(string) error { called = true; return nil }
	if _, err := h.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !called {
		t.Error("buildDeps was not invoked when DependencyBuild=true")
	}
}

func TestHelmSource_DependencyBuildSkippedWhenDisabled(t *testing.T) {
	called := false
	h := NewHelmSource(context.Background(), "u", "", "chart", "rel", "ns", nil, nil, false, nil, helmCloneWritingChart(t))
	h.buildDeps = func(string) error { called = true; return nil }
	if _, err := h.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if called {
		t.Error("buildDeps was invoked despite DependencyBuild=false")
	}
}

func TestHelmSource_DependencyBuildError(t *testing.T) {
	h := NewHelmSource(context.Background(), "u", "", "chart", "rel", "ns", nil, nil, true, nil, helmCloneWritingChart(t))
	h.buildDeps = func(string) error { return errors.New("dependency fetch failed") }
	if _, err := h.Load(); err == nil {
		t.Fatal("expected Load to surface the buildDeps error, got nil")
	}
}

// TestBuildDependencies_NoDeps exercises the real buildDependencies plumbing
// (temp repo dir, repo-file write, Manager.Update) on a dependency-free chart,
// which resolves to a no-op without any network access.
func TestHelmSource_DependencyBuildCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already canceled
	h := NewHelmSource(ctx, "u", "", "chart", "rel", "ns", nil, nil, true, nil, helmCloneWritingChart(t))
	// A builder that blocks until cleanup; the canceled ctx must win the select.
	block := make(chan struct{})
	t.Cleanup(func() { close(block) })
	h.buildDeps = func(string) error { <-block; return nil }
	if _, err := h.Load(); err == nil {
		t.Fatal("expected a canceled-context error, got nil")
	}
}

func TestBuildDependencies_NoDeps(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "chart")
	writeChart(t, dir) // Chart.yaml declares no dependencies
	if err := buildDependencies(dir); err != nil {
		t.Fatalf("buildDependencies on a dependency-free chart: %v", err)
	}
}

func TestRenderHelmChart_WithCRDs(t *testing.T) {
	dir := t.TempDir()
	writeChart(t, dir)
	// A chart's crds/ directory is not processed by engine.Render, so
	// renderHelmChart appends it explicitly — assert it lands in the output.
	mustWrite(t, filepath.Join(dir, "crds", "crd.yaml"),
		"apiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\nmetadata:\n  name: widgets.example.com\n")
	out, err := renderHelmChart(dir, "rel", "ns", nil, nil)
	if err != nil {
		t.Fatalf("renderHelmChart() error = %v", err)
	}
	if !strings.Contains(string(out), "CustomResourceDefinition") || !strings.Contains(string(out), "widgets.example.com") {
		t.Errorf("chart crds/ not included in render output:\n%s", out)
	}
}

func TestRenderHelmChart_BadInlineValues(t *testing.T) {
	dir := t.TempDir()
	writeChart(t, dir)
	// A YAML sequence where a mapping is required fails chartutil.ReadValues.
	if _, err := renderHelmChart(dir, "rel", "ns", []byte("- not\n- a\n- map\n"), nil); err == nil {
		t.Fatal("expected error for invalid inline values, got nil")
	}
}

func TestRenderHelmChart_SkipsNotes(t *testing.T) {
	dir := t.TempDir()
	writeChart(t, dir)
	mustWrite(t, filepath.Join(dir, "templates", "NOTES.txt"), "Thank you for installing {{ .Chart.Name }}\n")
	out, err := renderHelmChart(dir, "rel", "ns", nil, nil)
	if err != nil {
		t.Fatalf("renderHelmChart() error = %v", err)
	}
	if strings.Contains(string(out), "Thank you") {
		t.Errorf("NOTES.txt must be excluded from the rendered manifests:\n%s", out)
	}
}

func TestRenderHelmChart_RenderError(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "Chart.yaml"), "apiVersion: v2\nname: demo\nversion: 0.1.0\n")
	mustWrite(t, filepath.Join(dir, "values.yaml"), "x: y\n")
	// A template that always errors during rendering.
	mustWrite(t, filepath.Join(dir, "templates", "bad.yaml"), `{{ fail "intentional render failure" }}`)
	if _, err := renderHelmChart(dir, "rel", "ns", nil, nil); err == nil {
		t.Fatal("expected a render error, got nil")
	}
}

func TestRenderHelmChart_BadValuesFile(t *testing.T) {
	dir := t.TempDir()
	writeChart(t, dir)
	mustWrite(t, filepath.Join(dir, "bad.yaml"), "{ not: valid: yaml:")
	if _, err := renderHelmChart(dir, "rel", "ns", nil, []string{"bad.yaml"}); err == nil {
		t.Fatal("expected error for an invalid values file, got nil")
	}
}

func TestMergeValues_NilDst(t *testing.T) {
	got := mergeValues(nil, map[string]any{"a": 1})
	if got == nil || got["a"] != 1 {
		t.Errorf("mergeValues(nil, ...) = %+v, want {a:1}", got)
	}
}

func TestBuildDependencies_BadChart(t *testing.T) {
	// A directory with no Chart.yaml fails the initial loader.Load.
	if err := buildDependencies(t.TempDir()); err == nil {
		t.Fatal("expected error loading a chart-less directory, got nil")
	}
}

func TestDependencyRepoFile(t *testing.T) {
	deps := []*chart.Dependency{
		{Name: "a", Repository: "https://charts.example.com"},
		{Name: "b", Repository: "https://charts.example.com"}, // duplicate URL → deduped
		{Name: "c", Repository: "@localrepo"},                 // named alias → skipped
		{Name: "d", Repository: ""},                           // empty → skipped
		{Name: "e", Repository: "https://other.example.com"},
	}
	rf := dependencyRepoFile(deps)
	if len(rf.Repositories) != 2 {
		t.Fatalf("got %d registered repos, want 2 (dedup + skip named/empty): %+v", len(rf.Repositories), rf.Repositories)
	}
	urls := map[string]bool{}
	for _, r := range rf.Repositories {
		urls[r.URL] = true
	}
	if !urls["https://charts.example.com"] || !urls["https://other.example.com"] {
		t.Errorf("registered URLs = %v, want both example URLs", urls)
	}
}

// writeChart writes a minimal renderable chart into dir and returns dir.
func writeChart(t *testing.T, dir string) {
	t.Helper()
	mustWrite(t, filepath.Join(dir, "Chart.yaml"), "apiVersion: v2\nname: demo\nversion: 0.1.0\n")
	mustWrite(t, filepath.Join(dir, "values.yaml"), "greeting: default\n")
	tmpl := `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-cm
  namespace: {{ .Release.Namespace }}
data:
  greeting: {{ .Values.greeting }}
`
	mustWrite(t, filepath.Join(dir, "templates", "cm.yaml"), tmpl)
	// A partial and NOTES.txt must be dropped from the rendered output.
	mustWrite(t, filepath.Join(dir, "templates", "_helpers.tpl"), `{{- define "x" -}}y{{- end -}}`)
	mustWrite(t, filepath.Join(dir, "templates", "NOTES.txt"), "thanks for installing")
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRenderHelmChart_ValuesPrecedence(t *testing.T) {
	dir := t.TempDir()
	writeChart(t, dir)

	// Inline values override the chart default.
	out, err := renderHelmChart(dir, "rel", "ns", []byte(`{"greeting":"inline"}`), nil)
	if err != nil {
		t.Fatalf("renderHelmChart error = %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "name: rel-cm") || !strings.Contains(s, "namespace: ns") {
		t.Errorf("release name/namespace not rendered: %s", s)
	}
	if !strings.Contains(s, "greeting: inline") {
		t.Errorf("inline value did not win: %s", s)
	}
	// NOTES.txt and partials are excluded.
	if strings.Contains(s, "thanks for installing") {
		t.Errorf("NOTES.txt leaked into output: %s", s)
	}
}

func TestRenderHelmChart_ValuesFileThenInline(t *testing.T) {
	dir := t.TempDir()
	writeChart(t, dir)
	mustWrite(t, filepath.Join(dir, "override.yaml"), "greeting: fromfile\n")

	// Values file applies when no inline override is given.
	out, err := renderHelmChart(dir, "rel", "ns", nil, []string{"override.yaml"})
	if err != nil {
		t.Fatalf("renderHelmChart error = %v", err)
	}
	if !strings.Contains(string(out), "greeting: fromfile") {
		t.Errorf("values file not applied: %s", out)
	}

	// Inline still wins over the file.
	out, err = renderHelmChart(dir, "rel", "ns", []byte(`{"greeting":"inline"}`), []string{"override.yaml"})
	if err != nil {
		t.Fatalf("renderHelmChart error = %v", err)
	}
	if !strings.Contains(string(out), "greeting: inline") {
		t.Errorf("inline did not override values file: %s", out)
	}
}

func TestRenderHelmChart_BadChart(t *testing.T) {
	if _, err := renderHelmChart(t.TempDir(), "rel", "ns", nil, nil); err == nil {
		t.Error("expected error loading a directory with no Chart.yaml")
	}
}

func TestRenderHelmChart_MissingValuesFile(t *testing.T) {
	dir := t.TempDir()
	writeChart(t, dir)
	if _, err := renderHelmChart(dir, "rel", "ns", nil, []string{"nope.yaml"}); err == nil {
		t.Error("expected error for a missing values file")
	}
}

func TestHelmSource_Load(t *testing.T) {
	// The fake cloner writes the chart into a "chart" sub-directory of the
	// clone target, exercising the withCheckout + sub-path path.
	clone := func(_ context.Context, dir, _, _ string, _ *GitAuth) error {
		writeChart(t, filepath.Join(dir, "chart"))
		return nil
	}
	h := NewHelmSource(context.Background(), "https://example.com/repo.git", "main", "chart",
		"rel", "ns", []byte(`{"greeting":"loaded"}`), nil, false, nil, clone)

	resources, err := h.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("got %d resources, want 1", len(resources))
	}
	if resources[0].Kind != "ConfigMap" || resources[0].Name != "rel-cm" {
		t.Errorf("unexpected resource: %+v", resources[0])
	}
}

func TestMergeValues(t *testing.T) {
	dst := map[string]interface{}{
		"a":      "1",
		"nested": map[string]interface{}{"x": "keep", "y": "old"},
	}
	src := map[string]interface{}{
		"b":      "2",
		"nested": map[string]interface{}{"y": "new", "z": "add"},
	}
	got := mergeValues(dst, src)
	nested := got["nested"].(map[string]interface{})
	if nested["x"] != "keep" || nested["y"] != "new" || nested["z"] != "add" {
		t.Errorf("nested merge = %+v", nested)
	}
	if got["a"] != "1" || got["b"] != "2" {
		t.Errorf("top-level merge = %+v", got)
	}
}
