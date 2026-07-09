package source

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// writeKustomization writes a minimal overlay (one ConfigMap + a namePrefix)
// into dir.
func writeKustomization(t *testing.T, dir string) {
	t.Helper()
	mustWrite(t, filepath.Join(dir, "kustomization.yaml"), "resources:\n- cm.yaml\nnamePrefix: prod-\n")
	mustWrite(t, filepath.Join(dir, "cm.yaml"),
		"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: config\ndata:\n  k: v\n")
}

func TestRenderKustomize(t *testing.T) {
	dir := t.TempDir()
	writeKustomization(t, dir)

	out, err := renderKustomize(dir)
	if err != nil {
		t.Fatalf("renderKustomize error = %v", err)
	}
	if !strings.Contains(string(out), "name: prod-config") {
		t.Errorf("namePrefix not applied: %s", out)
	}
}

func TestRenderKustomize_NoKustomization(t *testing.T) {
	if _, err := renderKustomize(t.TempDir()); err == nil {
		t.Error("expected error building a directory with no kustomization.yaml")
	}
}

// A standard overlay → ../base layout must build: a directory base carrying its
// own kustomization.yaml is re-rooted, so the default root-only load restriction
// does not block it.
func TestRenderKustomize_OverlayReferencingBase(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "base", "kustomization.yaml"), "resources:\n- cm.yaml\n")
	mustWrite(t, filepath.Join(root, "base", "cm.yaml"),
		"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: config\ndata:\n  k: v\n")
	mustWrite(t, filepath.Join(root, "overlay", "kustomization.yaml"),
		"resources:\n- ../base\nnamePrefix: prod-\n")

	out, err := renderKustomize(filepath.Join(root, "overlay"))
	if err != nil {
		t.Fatalf("renderKustomize error = %v", err)
	}
	if !strings.Contains(string(out), "name: prod-config") {
		t.Errorf("overlay/base build did not apply namePrefix: %s", out)
	}
}

func TestKustomizeSource_Load(t *testing.T) {
	clone := func(_ context.Context, dir, _, _ string) error {
		writeKustomization(t, filepath.Join(dir, "overlay"))
		return nil
	}
	k := NewKustomizeSource(context.Background(), "https://example.com/repo.git", "main", "overlay", clone)

	resources, err := k.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("got %d resources, want 1", len(resources))
	}
	if resources[0].Kind != "ConfigMap" || resources[0].Name != "prod-config" {
		t.Errorf("unexpected resource: %+v", resources[0])
	}
}
