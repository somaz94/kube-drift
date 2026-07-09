package source

import (
	"context"
	"fmt"

	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/kyaml/filesys"

	kdsource "github.com/somaz94/kube-diff/pkg/source"
)

// KustomizeSource builds a Kustomize overlay from a Git checkout in-process (no
// `kustomize`/`kubectl` binary or shell-out) and compares the built manifests
// against the cluster. It implements kube-diff's source.Source: Load clones the
// repository, runs the kustomization under the configured sub-path, and parses
// the output — cleaning up the checkout before returning.
type KustomizeSource struct {
	URL, Ref, Path string

	// auth holds clone credentials; nil clones anonymously.
	auth *GitAuth

	// ctx carries the reconcile request's cancellation into the clone, bounded
	// to a single Reconcile call (same rationale as GitSource.ctx). clone
	// defaults to the real go-git clone when nil.
	ctx   context.Context
	clone CloneFunc
}

// NewKustomizeSource builds a KustomizeSource. When clone is nil the real
// go-git-backed clone is used; tests pass a fake to stay offline. auth may be
// nil for an anonymous clone.
func NewKustomizeSource(ctx context.Context, url, ref, path string, auth *GitAuth, clone CloneFunc) *KustomizeSource {
	return &KustomizeSource{URL: url, Ref: ref, Path: path, auth: auth, ctx: ctx, clone: clone}
}

// Load clones the repository, builds the overlay, parses the manifests, and
// cleans up.
func (k *KustomizeSource) Load() ([]kdsource.Resource, error) {
	return withCheckout(k.ctx, k.URL, k.Ref, k.Path, k.auth, k.clone, func(dir string) ([]kdsource.Resource, error) {
		manifests, err := renderKustomize(dir)
		if err != nil {
			return nil, err
		}
		return kdsource.NewBytesSource(manifests).Load()
	})
}

// renderKustomize runs the kustomization rooted at dir and returns the built
// YAML stream. It uses the default (root-only) load restrictions so a
// kustomization cannot read files outside its own directory tree.
func renderKustomize(dir string) ([]byte, error) {
	k := krusty.MakeKustomizer(krusty.MakeDefaultOptions())
	resMap, err := k.Run(filesys.MakeFsOnDisk(), dir)
	if err != nil {
		return nil, fmt.Errorf("kustomize build %s: %w", dir, err)
	}
	out, err := resMap.AsYaml()
	if err != nil {
		return nil, fmt.Errorf("render kustomize output: %w", err)
	}
	return out, nil
}
