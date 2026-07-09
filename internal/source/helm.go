package source

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	"helm.sh/helm/v3/pkg/releaseutil"

	kdsource "github.com/somaz94/kube-diff/pkg/source"
)

// HelmSource renders a Helm chart from a Git checkout in-process (no `helm`
// binary or shell-out) and compares the rendered manifests against the cluster.
// It implements kube-diff's source.Source: Load clones the repository, renders
// the chart under the configured sub-path, and parses the output — cleaning up
// the checkout before returning.
type HelmSource struct {
	URL, Ref, Path         string
	ReleaseName, Namespace string

	// Values holds inline chart values (JSON or YAML bytes), applied last. May
	// be nil.
	Values []byte
	// ValuesFiles are values files relative to the chart directory, merged in
	// order before Values.
	ValuesFiles []string

	// auth holds clone credentials; nil clones anonymously.
	auth *GitAuth

	// ctx carries the reconcile request's cancellation into the clone, bounded
	// to a single Reconcile call (same rationale as GitSource.ctx). clone
	// defaults to the real go-git clone when nil.
	ctx   context.Context
	clone CloneFunc
}

// NewHelmSource builds a HelmSource. When clone is nil the real go-git-backed
// clone is used; tests pass a fake to stay offline. auth may be nil for an
// anonymous clone.
func NewHelmSource(ctx context.Context, url, ref, path, releaseName, namespace string, values []byte, valuesFiles []string, auth *GitAuth, clone CloneFunc) *HelmSource {
	return &HelmSource{
		URL: url, Ref: ref, Path: path,
		ReleaseName: releaseName, Namespace: namespace,
		Values: values, ValuesFiles: valuesFiles,
		auth: auth, ctx: ctx, clone: clone,
	}
}

// Load clones the repository, renders the chart, parses the manifests, and
// cleans up.
func (h *HelmSource) Load() ([]kdsource.Resource, error) {
	return withCheckout(h.ctx, h.URL, h.Ref, h.Path, h.auth, h.clone, func(chartDir string) ([]kdsource.Resource, error) {
		manifests, err := renderHelmChart(chartDir, h.ReleaseName, h.Namespace, h.Values, h.ValuesFiles)
		if err != nil {
			return nil, err
		}
		return kdsource.NewBytesSource(manifests).Load()
	})
}

// renderHelmChart loads the chart at chartDir, merges the values (files first,
// in order, then inline values on top), renders the templates with the release
// name/namespace, and returns the concatenated manifest stream compared against
// the cluster.
//
// The output is the chart's steady-state resources:
//   - CRDs shipped under the chart's crds/ directory are included (engine.Render
//     only processes templates/, so they are added explicitly).
//   - Hook resources (pre/post-install, test Pods/Jobs, …) are excluded — they
//     are Helm-lifecycle objects that do not persist in the cluster, so
//     comparing them would report permanent false drift.
//   - Partials (_*.tpl), NOTES.txt, and empty documents are dropped.
//
// Chart dependencies must be vendored under charts/; declared-but-unfetched
// dependencies (no `helm dependency build`) are silently absent from the render.
// valuesFiles are confined to the chart directory (a "../" escape is rejected).
func renderHelmChart(chartDir, releaseName, namespace string, inline []byte, valuesFiles []string) ([]byte, error) {
	ch, err := loader.Load(chartDir)
	if err != nil {
		return nil, fmt.Errorf("load chart %s: %w", chartDir, err)
	}

	vals := map[string]any{}
	for _, vf := range valuesFiles {
		p, err := securejoin.SecureJoin(chartDir, vf)
		if err != nil {
			return nil, fmt.Errorf("resolve values file %q: %w", vf, err)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read values file %q: %w", vf, err)
		}
		m, err := chartutil.ReadValues(b)
		if err != nil {
			return nil, fmt.Errorf("parse values file %q: %w", vf, err)
		}
		vals = mergeValues(vals, m)
	}
	if len(inline) > 0 {
		m, err := chartutil.ReadValues(inline)
		if err != nil {
			return nil, fmt.Errorf("parse inline values: %w", err)
		}
		vals = mergeValues(vals, m)
	}

	relOpts := chartutil.ReleaseOptions{Name: releaseName, Namespace: namespace}
	renderVals, err := chartutil.ToRenderValues(ch, vals, relOpts, chartutil.DefaultCapabilities)
	if err != nil {
		return nil, fmt.Errorf("build render values: %w", err)
	}
	files, err := engine.Render(ch, renderVals)
	if err != nil {
		return nil, fmt.Errorf("render chart: %w", err)
	}

	// Drop NOTES.txt before splitting; SortManifests skips partials and empty
	// files itself, separates hook resources out, and orders the rest by kind.
	rendered := make(map[string]string, len(files))
	for k, v := range files {
		if strings.HasSuffix(k, "NOTES.txt") {
			continue
		}
		rendered[k] = v
	}
	_, manifests, err := releaseutil.SortManifests(rendered, chartutil.DefaultCapabilities.APIVersions, releaseutil.InstallOrder)
	if err != nil {
		return nil, fmt.Errorf("sort rendered manifests: %w", err)
	}

	docs := make([][]byte, 0, len(manifests)+len(ch.CRDObjects()))
	for _, crd := range ch.CRDObjects() {
		if len(bytes.TrimSpace(crd.File.Data)) > 0 {
			docs = append(docs, crd.File.Data)
		}
	}
	for _, m := range manifests {
		if strings.TrimSpace(m.Content) == "" {
			continue
		}
		docs = append(docs, []byte(m.Content))
	}
	return bytes.Join(docs, []byte("\n---\n")), nil
}

// mergeValues deep-merges src into dst, with src taking precedence, and returns
// dst. Nested maps are merged recursively; any other value in src overwrites the
// corresponding key in dst. Note: src's nested maps are aliased into dst, not
// deep-copied — callers must pass transient maps (freshly parsed per file), not
// shared ones.
func mergeValues(dst, src map[string]any) map[string]any {
	if dst == nil {
		dst = map[string]any{}
	}
	for k, v := range src {
		if srcMap, ok := v.(map[string]any); ok {
			if dstMap, ok := dst[k].(map[string]any); ok {
				dst[k] = mergeValues(dstMap, srcMap)
				continue
			}
		}
		dst[k] = v
	}
	return dst
}
