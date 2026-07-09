package source

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeClone writes the given files (relative path → contents) into the clone
// directory instead of reaching the network, letting Load run offline.
func fakeClone(files map[string]string) CloneFunc {
	return func(_ context.Context, dir, _, _ string, _ *GitAuth) error {
		for rel, content := range files {
			full := filepath.Join(dir, rel)
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
				return err
			}
		}
		return nil
	}
}

func TestGitSource_Load(t *testing.T) {
	clone := fakeClone(map[string]string{
		"manifests/cm.yaml": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: app\n  namespace: default\n",
		"README.md":         "# not a manifest",
	})
	src := NewGitSource(context.Background(), "https://example.com/repo.git", "main", "manifests", nil, clone)

	resources, err := src.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("got %d resources, want 1: %+v", len(resources), resources)
	}
	if resources[0].Kind != "ConfigMap" || resources[0].Name != "app" {
		t.Errorf("resource = %+v, want ConfigMap/app", resources[0])
	}
}

func TestGitSource_LoadRepoRoot(t *testing.T) {
	clone := fakeClone(map[string]string{
		"svc.yaml": "apiVersion: v1\nkind: Service\nmetadata:\n  name: svc\n",
	})
	// Empty path → repository root is loaded.
	src := NewGitSource(context.Background(), "https://example.com/repo.git", "", "", nil, clone)

	resources, err := src.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(resources) != 1 || resources[0].Kind != "Service" {
		t.Fatalf("got %+v, want a single Service", resources)
	}
}

func TestGitSource_CleansUpTempDir(t *testing.T) {
	var seenDir string
	clone := func(_ context.Context, dir, _, _ string, _ *GitAuth) error {
		seenDir = dir
		return os.WriteFile(filepath.Join(dir, "x.yaml"), []byte("kind: X\napiVersion: v1\nmetadata:\n  name: x\n"), 0o644)
	}
	src := NewGitSource(context.Background(), "u", "", "", nil, clone)
	if _, err := src.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if seenDir == "" {
		t.Fatal("clone was not invoked")
	}
	if _, err := os.Stat(seenDir); !os.IsNotExist(err) {
		t.Errorf("temp dir %q was not removed after Load (stat err = %v)", seenDir, err)
	}
}

func TestGitSource_NilContextFallsBackToBackground(t *testing.T) {
	// A GitSource built without a context (nil) still loads: Load falls back to
	// context.Background rather than passing a nil context to the cloner.
	var gotCtx context.Context
	src := &GitSource{
		URL: "u",
		clone: func(ctx context.Context, dir, _, _ string, _ *GitAuth) error {
			gotCtx = ctx
			return os.WriteFile(filepath.Join(dir, "x.yaml"),
				[]byte("apiVersion: v1\nkind: X\nmetadata:\n  name: x\n"), 0o644)
		},
	}
	if _, err := src.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if gotCtx == nil {
		t.Error("cloner received a nil context, want context.Background()")
	}
}

func TestGitSource_CloneError(t *testing.T) {
	wantErr := os.ErrPermission
	clone := func(_ context.Context, _, _, _ string, _ *GitAuth) error { return wantErr }
	src := NewGitSource(context.Background(), "u", "", "", nil, clone)

	if _, err := src.Load(); err == nil {
		t.Fatal("expected clone error to propagate, got nil")
	}
}

func TestGitSource_PathTraversalClamped(t *testing.T) {
	// The clone succeeds, but the sub-path tries to escape the checkout root.
	// SecureJoin clamps it back inside root, so it resolves to a non-existent
	// directory under the checkout rather than reading /etc — Load errors on
	// the missing directory instead of leaking host files.
	clone := fakeClone(map[string]string{"a.yaml": "kind: A\napiVersion: v1\nmetadata:\n  name: a\n"})
	src := NewGitSource(context.Background(), "u", "", "../../etc", nil, clone)

	if _, err := src.Load(); err == nil {
		t.Fatal("expected error for clamped non-existent path, got nil")
	}
}

func TestGitSource_SecureJoinError(t *testing.T) {
	// A symlink loop inside the checkout makes SecureJoin fail (ELOOP) while
	// resolving a sub-path that passes through it, so Load surfaces the error
	// instead of hanging or escaping.
	clone := func(_ context.Context, dir, _, _ string, _ *GitAuth) error {
		// A relative self-referential symlink ("loop" -> "loop") drives
		// SecureJoin into ELOOP when a sub-path descends through it.
		return os.Symlink("loop", filepath.Join(dir, "loop"))
	}
	src := NewGitSource(context.Background(), "u", "", "loop/x", nil, clone)

	if _, err := src.Load(); err == nil {
		t.Fatal("expected SecureJoin error on symlink loop, got nil")
	}
}

func TestResolveSubPath(t *testing.T) {
	root := "/tmp/clone"
	// Legitimate sub-paths resolve verbatim under root.
	for _, tc := range []struct{ sub, want string }{
		{"", root},
		{"manifests", filepath.Join(root, "manifests")},
		{"a/b/c", filepath.Join(root, "a/b/c")},
	} {
		got, err := resolveSubPath(root, tc.sub)
		if err != nil {
			t.Errorf("resolveSubPath(%q) unexpected error: %v", tc.sub, err)
		}
		if got != tc.want {
			t.Errorf("resolveSubPath(%q) = %q, want %q", tc.sub, got, tc.want)
		}
	}

	// Escaping sub-paths are clamped back inside root, never above it.
	for _, sub := range []string{"..", "../outside", "../../etc/passwd"} {
		got, err := resolveSubPath(root, sub)
		if err != nil {
			t.Errorf("resolveSubPath(%q) unexpected error: %v", sub, err)
		}
		if got != root && !strings.HasPrefix(got, root+string(os.PathSeparator)) {
			t.Errorf("resolveSubPath(%q) = %q, escaped root %q", sub, got, root)
		}
	}
}
