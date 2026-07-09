package source

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	clone := func(_ context.Context, dir, _, _ string) error {
		writeChart(t, filepath.Join(dir, "chart"))
		return nil
	}
	h := NewHelmSource(context.Background(), "https://example.com/repo.git", "main", "chart",
		"rel", "ns", []byte(`{"greeting":"loaded"}`), nil, clone)

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
