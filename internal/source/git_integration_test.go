package source

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// newLocalRepo creates a real on-disk git repository with a single manifest
// committed under manifests/, plus a "v1" tag on that commit. go-git clones a
// local path the same way it clones a remote, so this exercises the real
// gitClone/resolveRevision path without any network access.
func newLocalRepo(t *testing.T) (dir, head string) {
	t.Helper()
	dir = t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("PlainInit: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "manifests"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	manifest := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: from-git\n  namespace: default\n"
	if err := os.WriteFile(filepath.Join(dir, "manifests", "cm.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}
	if _, err := wt.Add("manifests/cm.yaml"); err != nil {
		t.Fatalf("add: %v", err)
	}
	sig := &object.Signature{Name: "test", Email: "test@example.com", When: time.Unix(0, 0).UTC()}
	hash, err := wt.Commit("init", &git.CommitOptions{Author: sig, Committer: sig})
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if _, err := repo.CreateTag("v1", hash, nil); err != nil {
		t.Fatalf("tag: %v", err)
	}
	// A non-default branch so the "origin/<ref>" fallback in resolveRevision is
	// exercised (the default branch resolves via its verbatim name).
	if err := repo.Storer.SetReference(plumbing.NewHashReference("refs/heads/feature", hash)); err != nil {
		t.Fatalf("branch: %v", err)
	}
	return dir, hash.String()
}

func TestGitClone_RealLocalRepo(t *testing.T) {
	repoDir, head := newLocalRepo(t)

	tests := []struct {
		name string
		ref  string
	}{
		{"default branch", ""},
		{"commit sha", head},
		{"tag", "v1"},
		{"non-default branch", "feature"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use the real gitClone (nil cloner) against the local path.
			src := NewGitSource(context.Background(), repoDir, tt.ref, "manifests", nil, nil)
			resources, err := src.Load()
			if err != nil {
				t.Fatalf("Load(ref=%q) error = %v", tt.ref, err)
			}
			if len(resources) != 1 || resources[0].Name != "from-git" {
				t.Fatalf("ref=%q resources = %+v, want a single from-git ConfigMap", tt.ref, resources)
			}
		})
	}
}

func TestGitClone_BadRef(t *testing.T) {
	repoDir, _ := newLocalRepo(t)
	src := NewGitSource(context.Background(), repoDir, "does-not-exist", "", nil, nil)
	if _, err := src.Load(); err == nil {
		t.Fatal("expected error for unresolvable ref, got nil")
	}
}

func TestGitClone_BadURL(t *testing.T) {
	// A path with no repository fails the clone itself.
	src := NewGitSource(context.Background(), filepath.Join(t.TempDir(), "nope"), "", "", nil, nil)
	if _, err := src.Load(); err == nil {
		t.Fatal("expected clone error for missing repo, got nil")
	}
}
