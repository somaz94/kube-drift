// Package source adapts external desired-state backends (Git repositories,
// plus in-process Helm-chart and Kustomize-overlay rendering on top of a Git
// checkout) into the kube-diff source.Source interface consumed by the drift
// engine.
package source

import (
	"context"
	"fmt"
	"os"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	kdsource "github.com/somaz94/kube-diff/pkg/source"
)

// CloneFunc clones the repository at url into dir and checks out ref (a branch,
// tag, or commit; empty means the repository's default branch). It is separated
// from GitSource so tests can inject a hermetic implementation that populates a
// local directory instead of reaching the network.
type CloneFunc func(ctx context.Context, dir, url, ref string) error

// GitSource loads plain-YAML manifests from a Git repository. It implements
// kube-diff's source.Source: Load clones the repository into a temporary
// directory, checks out the requested ref, parses the manifests under the
// configured sub-path via kube-diff's FileSource, and removes the checkout
// before returning (Load yields parsed in-memory resources, not file paths).
//
// Only plain YAML is supported — no Helm/Kustomize rendering. Authentication is
// not yet wired, so only anonymously cloneable repositories work in v0.1.
type GitSource struct {
	URL  string
	Ref  string
	Path string

	// ctx carries the reconcile request's cancellation into the clone. A
	// GitSource is created and consumed within a single Reconcile call, so
	// storing the request context here (rather than threading it through the
	// context-free source.Source.Load signature) is bounded to that lifetime.
	ctx context.Context

	// clone performs the actual clone+checkout; defaults to gitClone.
	clone CloneFunc
}

// NewGitSource builds a GitSource. When clone is nil the real go-git-backed
// gitClone is used; tests pass a fake to stay offline.
func NewGitSource(ctx context.Context, url, ref, path string, clone CloneFunc) *GitSource {
	if clone == nil {
		clone = gitClone
	}
	return &GitSource{URL: url, Ref: ref, Path: path, ctx: ctx, clone: clone}
}

// Load clones the repository, parses the manifests under Path, and cleans up.
func (g *GitSource) Load() ([]kdsource.Resource, error) {
	return withCheckout(g.ctx, g.URL, g.Ref, g.Path, g.clone, func(loadPath string) ([]kdsource.Resource, error) {
		return kdsource.NewFileSource(loadPath).Load()
	})
}

// withCheckout clones url@ref into a temporary directory, resolves path against
// the checkout (rejecting escapes), invokes fn with the resolved load path, and
// removes the checkout before returning. It is the shared clone+cleanup harness
// behind the Git, Helm, and Kustomize sources — each supplies its own fn to
// parse or render the checked-out files.
//
// The checkout is removed via a deferred cleanup whose failure is surfaced on
// the (named) error return when fn otherwise succeeded — this controller clones
// on every reconcile, so a silently-leaked temp directory would accumulate.
func withCheckout(ctx context.Context, url, ref, path string, clone CloneFunc, fn func(loadPath string) ([]kdsource.Resource, error)) (resources []kdsource.Resource, err error) {
	if clone == nil {
		clone = gitClone
	}
	dir, err := os.MkdirTemp("", "kube-drift-git-")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer func() {
		if rerr := os.RemoveAll(dir); rerr != nil && err == nil {
			err = fmt.Errorf("cleanup checkout %s: %w", dir, rerr)
		}
	}()

	if ctx == nil {
		ctx = context.Background()
	}
	if err := clone(ctx, dir, url, ref); err != nil {
		return nil, fmt.Errorf("clone %s: %w", url, err)
	}

	loadPath, err := resolveSubPath(dir, path)
	if err != nil {
		return nil, err
	}
	return fn(loadPath)
}

// resolveSubPath joins root with the user-supplied sub-path using SecureJoin,
// which resolves symlink components against root so that neither "../" strings
// nor a symlink committed inside the cloned repository (e.g. "manifests" ->
// "/etc") can escape the checkout. An empty sub-path returns the repository root.
func resolveSubPath(root, sub string) (string, error) {
	if sub == "" {
		return root, nil
	}
	joined, err := securejoin.SecureJoin(root, sub)
	if err != nil {
		return "", fmt.Errorf("resolve sub-path %q: %w", sub, err)
	}
	return joined, nil
}

// gitClone clones url into dir using go-git (pure Go, no git binary or shell
// out) and checks out ref. An empty ref leaves the default branch checked out.
func gitClone(ctx context.Context, dir, url, ref string) error {
	repo, err := git.PlainCloneContext(ctx, dir, false, &git.CloneOptions{URL: url})
	if err != nil {
		return err
	}
	if ref == "" {
		return nil
	}

	hash, err := resolveRevision(repo, ref)
	if err != nil {
		return fmt.Errorf("resolve ref %q: %w", ref, err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("open worktree: %w", err)
	}
	if err := wt.Checkout(&git.CheckoutOptions{Hash: *hash}); err != nil {
		return fmt.Errorf("checkout %q: %w", ref, err)
	}
	return nil
}

// resolveRevision resolves ref to a commit hash. It first tries ref verbatim
// (matching a commit SHA or tag), then the "origin/<ref>" remote-tracking form
// so a non-default branch name still resolves after a full clone.
func resolveRevision(repo *git.Repository, ref string) (*plumbing.Hash, error) {
	var lastErr error
	for _, rev := range []plumbing.Revision{
		plumbing.Revision(ref),
		plumbing.Revision("origin/" + ref),
	} {
		if hash, err := repo.ResolveRevision(rev); err == nil {
			return hash, nil
		} else {
			lastErr = err
		}
	}
	return nil, lastErr
}
