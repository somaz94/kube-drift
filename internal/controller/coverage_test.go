package controller

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/somaz94/kube-diff/pkg/diff"
	myv1 "github.com/somaz94/kube-drift/api/v1alpha1"
)

func TestSummarize(t *testing.T) {
	drifted, summary := summarize([]*diff.Result{
		{APIVersion: "v1", Kind: "ConfigMap", Name: "a", Namespace: "default", Status: diff.StatusChanged},
		{APIVersion: "v1", Kind: "Service", Name: "b", Namespace: "default", Status: diff.StatusNew},
		{APIVersion: "apps/v1", Kind: "Deployment", Name: "c", Namespace: "default", Status: diff.StatusDeleted},
		{APIVersion: "v1", Kind: "Secret", Name: "d", Namespace: "default", Status: diff.StatusUnchanged},
	})
	if summary.Changed != 1 || summary.New != 1 || summary.Deleted != 1 || summary.Unchanged != 1 {
		t.Errorf("summary = %+v, want 1 of each", summary)
	}
	// Unchanged resources are omitted from the drifted list.
	if len(drifted) != 3 {
		t.Fatalf("drifted = %d, want 3 (unchanged omitted): %+v", len(drifted), drifted)
	}
	if drifted[0].Kind != "ConfigMap" || drifted[0].Status != myv1.DriftChanged {
		t.Errorf("drifted[0] = %+v, want ConfigMap/changed", drifted[0])
	}
}

// reconcileExpectingSourceError drives one Reconcile and asserts it recorded a
// permanent SourceError (interval requeue, no error returned).
func reconcileExpectingSourceError(t *testing.T, r *DriftCheckReconciler) {
	t.Helper()
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
	if len(got.Status.Conditions) == 0 || got.Status.Conditions[0].Reason != "SourceError" {
		t.Errorf("want a SourceError condition, got %+v", got.Status.Conditions)
	}
}

func TestResolveGitAuth_BasicMissingUsername(t *testing.T) {
	scheme := newScheme(t)
	sec := gitAuthSecret("creds", map[string][]byte{"password": []byte("tok")}) // no username
	r := reconcilerFor(scheme, &fakeFetcher{}, sec)
	if _, err := r.resolveGitAuth(context.Background(), "default",
		&myv1.GitAuth{Type: myv1.GitAuthBasic, SecretRef: myv1.LocalSecretRef{Name: "creds"}}); err == nil {
		t.Fatal("expected error for missing username key, got nil")
	}
}

func TestResolveGitAuth_BearerMissingToken(t *testing.T) {
	scheme := newScheme(t)
	sec := gitAuthSecret("creds", map[string][]byte{}) // no bearerToken
	r := reconcilerFor(scheme, &fakeFetcher{}, sec)
	if _, err := r.resolveGitAuth(context.Background(), "default",
		&myv1.GitAuth{Type: myv1.GitAuthBearer, SecretRef: myv1.LocalSecretRef{Name: "creds"}}); err == nil {
		t.Fatal("expected error for missing bearerToken key, got nil")
	}
}

func TestResolveGitAuth_SSHMissingIdentity(t *testing.T) {
	scheme := newScheme(t)
	sec := gitAuthSecret("creds", map[string][]byte{"known_hosts": []byte("github.com ssh-ed25519 AAAA")}) // no identity
	r := reconcilerFor(scheme, &fakeFetcher{}, sec)
	if _, err := r.resolveGitAuth(context.Background(), "default",
		&myv1.GitAuth{Type: myv1.GitAuthSSH, SecretRef: myv1.LocalSecretRef{Name: "creds"}}); err == nil {
		t.Fatal("expected error for missing identity key, got nil")
	}
}

func TestResolveWebhookURL_KeyNotFound(t *testing.T) {
	scheme := newScheme(t)
	sec := gitAuthSecret("wh", map[string][]byte{"other": []byte("x")}) // ref.Key absent
	r := reconcilerFor(scheme, &fakeFetcher{}, sec)
	if _, err := r.resolveWebhookURL(context.Background(), "default",
		&myv1.Webhook{URLSecretRef: &myv1.SecretKeyRef{Name: "wh", Key: "url"}}); err == nil {
		t.Fatal("expected error for missing Secret key, got nil")
	}
}

func TestResolveWebhookURL_EmptyValue(t *testing.T) {
	scheme := newScheme(t)
	sec := gitAuthSecret("wh", map[string][]byte{"url": []byte("   ")}) // whitespace-only
	r := reconcilerFor(scheme, &fakeFetcher{}, sec)
	if _, err := r.resolveWebhookURL(context.Background(), "default",
		&myv1.Webhook{URLSecretRef: &myv1.SecretKeyRef{Name: "wh", Key: "url"}}); err == nil {
		t.Fatal("expected error for empty Secret value, got nil")
	}
}

func TestResolveWebhookURL_NeitherURLNorRef(t *testing.T) {
	scheme := newScheme(t)
	r := reconcilerFor(scheme, &fakeFetcher{})
	if _, err := r.resolveWebhookURL(context.Background(), "default", &myv1.Webhook{}); err == nil {
		t.Fatal("expected error when neither url nor urlSecretRef is set, got nil")
	}
}

func TestReconcile_HelmAuthMissingSecret(t *testing.T) {
	scheme := newScheme(t)
	dc := &myv1.DriftCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "dc", Namespace: "default"},
		Spec: myv1.DriftCheckSpec{
			Source: myv1.Source{
				Type: myv1.SourceTypeHelm,
				Helm: &myv1.HelmSource{
					Git: myv1.GitSource{
						URL:  "https://example.com/repo.git",
						Auth: &myv1.GitAuth{Type: myv1.GitAuthBasic, SecretRef: myv1.LocalSecretRef{Name: "nope"}},
					},
				},
			},
			Interval: metav1.Duration{Duration: 5 * time.Minute},
		},
	}
	reconcileExpectingSourceError(t, reconcilerFor(scheme, &fakeFetcher{}, dc))
}

func TestReconcile_KustomizeAuthMissingSecret(t *testing.T) {
	scheme := newScheme(t)
	dc := &myv1.DriftCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "dc", Namespace: "default"},
		Spec: myv1.DriftCheckSpec{
			Source: myv1.Source{
				Type: myv1.SourceTypeKustomize,
				Kustomize: &myv1.KustomizeSource{
					Git: myv1.GitSource{
						URL:  "https://example.com/repo.git",
						Auth: &myv1.GitAuth{Type: myv1.GitAuthSSH, SecretRef: myv1.LocalSecretRef{Name: "nope"}},
					},
				},
			},
			Interval: metav1.Duration{Duration: 5 * time.Minute},
		},
	}
	reconcileExpectingSourceError(t, reconcilerFor(scheme, &fakeFetcher{}, dc))
}
