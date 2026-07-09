package controller

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	driftv1alpha1 "github.com/somaz94/kube-drift/api/v1alpha1"
	"github.com/somaz94/kube-drift/internal/metrics"
	"github.com/somaz94/kube-drift/internal/notify"
	driftsource "github.com/somaz94/kube-drift/internal/source"

	"github.com/somaz94/kube-diff/pkg/cluster"
	"github.com/somaz94/kube-diff/pkg/diff"
	"github.com/somaz94/kube-diff/pkg/engine"
	"github.com/somaz94/kube-diff/pkg/source"
)

const defaultInterval = 5 * time.Minute

// DriftCheckReconciler reconciles a DriftCheck object.
type DriftCheckReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// Fetcher retrieves live cluster state for comparison. It is injected so
	// tests can supply a fake; in-cluster it is built from the manager's
	// rest.Config via cluster.NewFetcherFromConfig (see cmd/main.go).
	Fetcher cluster.ResourceFetcher

	// Metrics records the per-DriftCheck drift gauges. It may be nil (methods
	// are no-ops), so the controller runs fine without metrics wiring.
	Metrics *metrics.Recorder

	// GitCloner clones the repository for Git-source DriftChecks. It is
	// injected so tests can stay offline; nil falls back to the real
	// go-git-backed clone in internal/source.
	GitCloner driftsource.CloneFunc

	// Notifier delivers drift notifications to the configured webhooks. It is
	// injected so tests can supply a fake; nil falls back to an HTTP sender.
	Notifier notify.Notifier

	// Recorder emits Kubernetes events (e.g. a notification delivery failure).
	// It may be nil (event emission is skipped).
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=drift.somaz.io,resources=driftchecks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=drift.somaz.io,resources=driftchecks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=drift.somaz.io,resources=driftchecks/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile evaluates a DriftCheck: it loads the desired manifests from the
// configured source, compares them against the live cluster via the kube-diff
// engine, records the per-resource drift into status, and requeues after the
// configured interval.
//
// NOTE: comparing arbitrary resources also requires the operator's
// ServiceAccount to hold read access to those kinds (e.g. bound to the built-in
// "view" ClusterRole). Only ConfigMap read is declared above; broader read
// access is granted at install time.
func (r *DriftCheckReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var dc driftv1alpha1.DriftCheck
	if err := r.Get(ctx, req.NamespacedName, &dc); err != nil {
		if apierrors.IsNotFound(err) {
			// The DriftCheck is gone: drop its metric series so stale gauges
			// do not linger in the registry.
			r.Metrics.Delete(req.Name, req.Namespace)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	interval := dc.Spec.Interval.Duration
	if interval <= 0 {
		interval = defaultInterval
	}

	if r.Fetcher == nil {
		return r.permanentFail(ctx, &dc, interval, "NoFetcher",
			fmt.Errorf("cluster fetcher is not configured"))
	}

	src, err := r.buildSource(ctx, &dc)
	if err != nil {
		// Source resolution failures (missing ConfigMap, bad key, a Git source
		// with no git block) are treated as non-fatal: record the condition and
		// retry on the interval rather than hot-looping.
		return r.permanentFail(ctx, &dc, interval, "SourceError", err)
	}

	results, err := engine.Run(ctx, src, r.Fetcher, diff.DefaultCompareOptions())
	if err != nil {
		// Comparison failures are typically transient (cluster API blips, a Git
		// clone that timed out): record the condition (best effort) and return
		// the error so controller-runtime retries with backoff instead of
		// waiting a full interval, and so it surfaces in reconcile-error
		// metrics.
		_ = r.markNotReady(ctx, &dc, "CompareError", err)
		return ctrl.Result{}, err
	}

	dc.Status.DriftedResources, dc.Status.Summary = summarize(results)
	now := metav1.Now()
	dc.Status.LastCheckedAt = &now
	dc.Status.ObservedGeneration = dc.Generation
	meta.SetStatusCondition(&dc.Status.Conditions, metav1.Condition{
		Type:   "Ready",
		Status: metav1.ConditionTrue,
		Reason: "DriftEvaluated",
		Message: fmt.Sprintf("%d changed, %d new, %d deleted",
			dc.Status.Summary.Changed, dc.Status.Summary.New, dc.Status.Summary.Deleted),
		ObservedGeneration: dc.Generation,
	})

	if err := r.Status().Update(ctx, &dc); err != nil {
		return ctrl.Result{}, err
	}

	r.Metrics.RecordDrift(dc.Name, dc.Namespace,
		dc.Status.Summary.Changed, dc.Status.Summary.New,
		dc.Status.Summary.Deleted, dc.Status.Summary.Unchanged)

	if err := r.notify(ctx, &dc); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("drift evaluated", "name", dc.Name,
		"changed", dc.Status.Summary.Changed,
		"new", dc.Status.Summary.New,
		"deleted", dc.Status.Summary.Deleted)
	return ctrl.Result{RequeueAfter: interval}, nil
}

// buildSource resolves the DriftCheck's source into a kube-diff source.Source.
func (r *DriftCheckReconciler) buildSource(ctx context.Context, dc *driftv1alpha1.DriftCheck) (source.Source, error) {
	switch dc.Spec.Source.Type {
	case driftv1alpha1.SourceTypeConfigMap:
		cm := dc.Spec.Source.ConfigMap
		if cm == nil {
			return nil, fmt.Errorf("source.configMap is required when type is ConfigMap")
		}
		ns := cm.Namespace
		if ns == "" {
			ns = dc.Namespace
		}
		var obj corev1.ConfigMap
		if err := r.Get(ctx, types.NamespacedName{Name: cm.Name, Namespace: ns}, &obj); err != nil {
			return nil, fmt.Errorf("get ConfigMap %s/%s: %w", ns, cm.Name, err)
		}
		data, err := configMapManifests(&obj, cm.Key)
		if err != nil {
			return nil, err
		}
		return source.NewBytesSource(data), nil
	case driftv1alpha1.SourceTypeGit:
		g := dc.Spec.Source.Git
		if g == nil {
			return nil, fmt.Errorf("source.git is required when type is Git")
		}
		if g.URL == "" {
			// The CRD marks url as required, but guard here too so a bad config
			// surfaces as a permanent SourceError rather than looping on clone.
			return nil, fmt.Errorf("source.git.url is required when type is Git")
		}
		auth, err := r.resolveGitAuth(ctx, dc.Namespace, g.Auth)
		if err != nil {
			return nil, err
		}
		return driftsource.NewGitSource(ctx, g.URL, g.Ref, g.Path, auth, r.GitCloner), nil
	case driftv1alpha1.SourceTypeHelm:
		h := dc.Spec.Source.Helm
		if h == nil {
			return nil, fmt.Errorf("source.helm is required when type is Helm")
		}
		if h.Git.URL == "" {
			return nil, fmt.Errorf("source.helm.git.url is required when type is Helm")
		}
		releaseName := h.ReleaseName
		if releaseName == "" {
			releaseName = dc.Name
		}
		namespace := h.Namespace
		if namespace == "" {
			namespace = dc.Namespace
		}
		var values []byte
		if h.Values != nil {
			values = h.Values.Raw
		}
		auth, err := r.resolveGitAuth(ctx, dc.Namespace, h.Git.Auth)
		if err != nil {
			return nil, err
		}
		return driftsource.NewHelmSource(ctx, h.Git.URL, h.Git.Ref, h.Git.Path,
			releaseName, namespace, values, h.ValuesFiles, h.DependencyBuild, auth, r.GitCloner), nil
	case driftv1alpha1.SourceTypeKustomize:
		k := dc.Spec.Source.Kustomize
		if k == nil {
			return nil, fmt.Errorf("source.kustomize is required when type is Kustomize")
		}
		if k.Git.URL == "" {
			return nil, fmt.Errorf("source.kustomize.git.url is required when type is Kustomize")
		}
		auth, err := r.resolveGitAuth(ctx, dc.Namespace, k.Git.Auth)
		if err != nil {
			return nil, err
		}
		return driftsource.NewKustomizeSource(ctx, k.Git.URL, k.Git.Ref, k.Git.Path, auth, r.GitCloner), nil
	default:
		return nil, fmt.Errorf("unknown source type %q", dc.Spec.Source.Type)
	}
}

// resolveGitAuth dereferences the DriftCheck's Git credentials from a Secret in
// its namespace, mirroring resolveWebhookURL. A nil spec yields nil (anonymous
// clone). The keys read depend on the scheme; a missing or empty required key is
// an error so a misconfigured Secret surfaces as a SourceError rather than a
// silent anonymous clone. Error messages never echo credential material.
func (r *DriftCheckReconciler) resolveGitAuth(ctx context.Context, ns string, spec *driftv1alpha1.GitAuth) (*driftsource.GitAuth, error) {
	if spec == nil {
		return nil, nil
	}
	var sec corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{Name: spec.SecretRef.Name, Namespace: ns}, &sec); err != nil {
		return nil, fmt.Errorf("get git auth Secret %s/%s: %w", ns, spec.SecretRef.Name, err)
	}
	secretName := ns + "/" + spec.SecretRef.Name

	// requireStr reads a required non-secret string key (e.g. a username),
	// trimming surrounding whitespace.
	requireStr := func(key string) (string, error) {
		v := strings.TrimSpace(string(sec.Data[key]))
		if v == "" {
			return "", fmt.Errorf("git auth Secret %s is missing key %q", secretName, key)
		}
		return v, nil
	}
	// requireSecret reads a required credential key (a password or token),
	// stripping only a trailing newline — a common Secret artifact — so any
	// other character in the credential is preserved verbatim.
	requireSecret := func(key string) (string, error) {
		v := strings.TrimRight(string(sec.Data[key]), "\r\n")
		if v == "" {
			return "", fmt.Errorf("git auth Secret %s is missing key %q", secretName, key)
		}
		return v, nil
	}

	switch spec.Type {
	case driftv1alpha1.GitAuthBasic:
		username, err := requireStr("username")
		if err != nil {
			return nil, err
		}
		password, err := requireSecret("password")
		if err != nil {
			return nil, err
		}
		return &driftsource.GitAuth{Basic: &driftsource.BasicAuth{Username: username, Password: password}}, nil
	case driftv1alpha1.GitAuthBearer:
		token, err := requireSecret("bearerToken")
		if err != nil {
			return nil, err
		}
		return &driftsource.GitAuth{Bearer: token}, nil
	case driftv1alpha1.GitAuthSSH:
		identity := sec.Data["identity"]
		if len(bytes.TrimSpace(identity)) == 0 {
			return nil, fmt.Errorf("git auth Secret %s is missing key %q", secretName, "identity")
		}
		knownHosts := sec.Data["known_hosts"]
		if len(bytes.TrimSpace(knownHosts)) == 0 {
			// Fail-closed: SSH host-key verification requires known_hosts.
			return nil, fmt.Errorf("git auth Secret %s is missing key %q (SSH host-key verification is fail-closed)", secretName, "known_hosts")
		}
		return &driftsource.GitAuth{SSH: &driftsource.SSHAuth{
			PrivateKey: identity,
			Passphrase: sec.Data["password"],
			KnownHosts: knownHosts,
		}}, nil
	default:
		return nil, fmt.Errorf("unknown git auth type %q", spec.Type)
	}
}

// configMapManifests extracts the YAML manifest bytes from a ConfigMap, reading
// from both Data and BinaryData. When key is set, that single entry is used;
// otherwise every entry is concatenated (in sorted key order for determinism)
// as a multi-document YAML stream. It errors when the selected content is empty
// so an empty ConfigMap surfaces as a condition rather than a silent no-op.
func configMapManifests(cm *corev1.ConfigMap, key string) ([]byte, error) {
	get := func(k string) ([]byte, bool) {
		if v, ok := cm.Data[k]; ok {
			return []byte(v), true
		}
		if v, ok := cm.BinaryData[k]; ok {
			return v, true
		}
		return nil, false
	}

	if key != "" {
		if v, ok := get(key); ok {
			return v, nil
		}
		return nil, fmt.Errorf("key %q not found in ConfigMap %s/%s", key, cm.Namespace, cm.Name)
	}

	keys := make([]string, 0, len(cm.Data)+len(cm.BinaryData))
	for k := range cm.Data {
		keys = append(keys, k)
	}
	for k := range cm.BinaryData {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	docs := make([][]byte, 0, len(keys))
	for _, k := range keys {
		if v, ok := get(k); ok {
			docs = append(docs, v)
		}
	}
	joined := bytes.Join(docs, []byte("\n---\n"))
	if len(bytes.TrimSpace(joined)) == 0 {
		return nil, fmt.Errorf("no manifests found in ConfigMap %s/%s", cm.Namespace, cm.Name)
	}
	return joined, nil
}

// summarize maps engine results into the DriftCheck status shape: a summary
// tally over every compared resource plus the list of resources that drifted
// (unchanged resources are omitted from the list).
func summarize(results []*diff.Result) ([]driftv1alpha1.DriftedResource, driftv1alpha1.DriftSummary) {
	var drifted []driftv1alpha1.DriftedResource
	var summary driftv1alpha1.DriftSummary
	for _, res := range results {
		switch res.Status {
		case diff.StatusChanged:
			summary.Changed++
		case diff.StatusNew:
			summary.New++
		case diff.StatusDeleted:
			summary.Deleted++
		default:
			summary.Unchanged++
			continue
		}
		drifted = append(drifted, driftv1alpha1.DriftedResource{
			APIVersion: res.APIVersion,
			Kind:       res.Kind,
			Name:       res.Name,
			Namespace:  res.Namespace,
			Status:     driftv1alpha1.DriftStatus(res.Status),
		})
	}
	return drifted, summary
}

// markNotReady sets a False "Ready" condition with the given reason and writes
// status. The status-update error (if any) is returned so callers can decide
// whether to retry (e.g. on an optimistic-concurrency conflict).
func (r *DriftCheckReconciler) markNotReady(ctx context.Context, dc *driftv1alpha1.DriftCheck, reason string, cause error) error {
	meta.SetStatusCondition(&dc.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            cause.Error(),
		ObservedGeneration: dc.Generation,
	})
	return r.Status().Update(ctx, dc)
}

// permanentFail records a not-ready condition for a non-transient failure and
// requeues after the interval. If the status write itself fails (e.g. a
// conflict), the error is returned so controller-runtime retries promptly
// instead of losing the condition until the next interval.
//
// It also clears the drift gauges: a persistent failure (missing source,
// unsupported/unknown type, no fetcher) means there is no valid drift reading,
// so a stale last-known-good count must not linger in the metrics. Transient
// compare failures take the markNotReady path instead and keep the last gauge
// while controller-runtime retries with backoff.
func (r *DriftCheckReconciler) permanentFail(ctx context.Context, dc *driftv1alpha1.DriftCheck, interval time.Duration, reason string, cause error) (ctrl.Result, error) {
	r.Metrics.Delete(dc.Name, dc.Namespace)
	if err := r.markNotReady(ctx, dc, reason, cause); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: interval}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DriftCheckReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&driftv1alpha1.DriftCheck{}).
		Complete(r)
}
