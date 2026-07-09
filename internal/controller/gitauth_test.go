package controller

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	myv1 "github.com/somaz94/kube-drift/api/v1alpha1"
	driftsource "github.com/somaz94/kube-drift/internal/source"
)

func gitAuthSecret(name string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Data:       data,
	}
}

func TestResolveGitAuth_Nil(t *testing.T) {
	scheme := newScheme(t)
	r := reconcilerFor(scheme, &fakeFetcher{})
	got, err := r.resolveGitAuth(context.Background(), "default", nil)
	if err != nil || got != nil {
		t.Fatalf("resolveGitAuth(nil) = %v, %v; want nil, nil", got, err)
	}
}

func TestResolveGitAuth_Basic(t *testing.T) {
	scheme := newScheme(t)
	// Trailing newlines (common in Secrets) must be trimmed for string creds.
	sec := gitAuthSecret("creds", map[string][]byte{"username": []byte("u\n"), "password": []byte("tok\n")})
	r := reconcilerFor(scheme, &fakeFetcher{}, sec)

	got, err := r.resolveGitAuth(context.Background(), "default",
		&myv1.GitAuth{Type: myv1.GitAuthBasic, SecretRef: myv1.LocalSecretRef{Name: "creds"}})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got == nil || got.Basic == nil {
		t.Fatalf("got %+v, want a Basic auth", got)
	}
	if got.Basic.Username != "u" || got.Basic.Password != "tok" {
		t.Errorf("creds = %+v, want {u tok} (whitespace not trimmed?)", got.Basic)
	}
}

func TestResolveGitAuth_BasicMissingKey(t *testing.T) {
	scheme := newScheme(t)
	sec := gitAuthSecret("creds", map[string][]byte{"username": []byte("u")}) // no password
	r := reconcilerFor(scheme, &fakeFetcher{}, sec)

	if _, err := r.resolveGitAuth(context.Background(), "default",
		&myv1.GitAuth{Type: myv1.GitAuthBasic, SecretRef: myv1.LocalSecretRef{Name: "creds"}}); err == nil {
		t.Fatal("expected error for missing password key, got nil")
	}
}

func TestResolveGitAuth_Bearer(t *testing.T) {
	scheme := newScheme(t)
	sec := gitAuthSecret("creds", map[string][]byte{"bearerToken": []byte("tok\n")})
	r := reconcilerFor(scheme, &fakeFetcher{}, sec)

	got, err := r.resolveGitAuth(context.Background(), "default",
		&myv1.GitAuth{Type: myv1.GitAuthBearer, SecretRef: myv1.LocalSecretRef{Name: "creds"}})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got == nil || got.Bearer != "tok" {
		t.Errorf("got %+v, want Bearer tok", got)
	}
}

func TestResolveGitAuth_SSH(t *testing.T) {
	scheme := newScheme(t)
	sec := gitAuthSecret("creds", map[string][]byte{
		"identity":    []byte("PEM-KEY"),
		"known_hosts": []byte("github.com ssh-ed25519 AAAA"),
		"password":    []byte("passphrase"),
	})
	r := reconcilerFor(scheme, &fakeFetcher{}, sec)

	got, err := r.resolveGitAuth(context.Background(), "default",
		&myv1.GitAuth{Type: myv1.GitAuthSSH, SecretRef: myv1.LocalSecretRef{Name: "creds"}})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got == nil || got.SSH == nil {
		t.Fatalf("got %+v, want an SSH auth", got)
	}
	if string(got.SSH.PrivateKey) != "PEM-KEY" || string(got.SSH.KnownHosts) != "github.com ssh-ed25519 AAAA" ||
		string(got.SSH.Passphrase) != "passphrase" {
		t.Errorf("ssh auth = %+v", got.SSH)
	}
}

func TestResolveGitAuth_SSHMissingKnownHosts(t *testing.T) {
	scheme := newScheme(t)
	// identity present but known_hosts absent → fail-closed error.
	sec := gitAuthSecret("creds", map[string][]byte{"identity": []byte("PEM-KEY")})
	r := reconcilerFor(scheme, &fakeFetcher{}, sec)

	if _, err := r.resolveGitAuth(context.Background(), "default",
		&myv1.GitAuth{Type: myv1.GitAuthSSH, SecretRef: myv1.LocalSecretRef{Name: "creds"}}); err == nil {
		t.Fatal("expected fail-closed error for SSH without known_hosts, got nil")
	}
}

func TestResolveGitAuth_MissingSecret(t *testing.T) {
	scheme := newScheme(t)
	r := reconcilerFor(scheme, &fakeFetcher{}) // no Secret in the client

	if _, err := r.resolveGitAuth(context.Background(), "default",
		&myv1.GitAuth{Type: myv1.GitAuthBasic, SecretRef: myv1.LocalSecretRef{Name: "nope"}}); err == nil {
		t.Fatal("expected error for missing Secret, got nil")
	}
}

func TestResolveGitAuth_UnknownType(t *testing.T) {
	scheme := newScheme(t)
	sec := gitAuthSecret("creds", map[string][]byte{"username": []byte("u"), "password": []byte("p")})
	r := reconcilerFor(scheme, &fakeFetcher{}, sec)

	if _, err := r.resolveGitAuth(context.Background(), "default",
		&myv1.GitAuth{Type: "Bogus", SecretRef: myv1.LocalSecretRef{Name: "creds"}}); err == nil {
		t.Fatal("expected error for unknown auth type, got nil")
	}
}

// TestReconcile_GitSourceThreadsAuth drives a full Reconcile of a Git-source
// DriftCheck with auth and asserts buildSource resolved the Secret and threaded
// the credentials into the cloner.
func TestReconcile_GitSourceThreadsAuth(t *testing.T) {
	scheme := newScheme(t)
	dc := &myv1.DriftCheck{
		ObjectMeta: metav1.ObjectMeta{Name: "dc", Namespace: "default"},
		Spec: myv1.DriftCheckSpec{
			Source: myv1.Source{
				Type: myv1.SourceTypeGit,
				Git: &myv1.GitSource{
					URL:  "https://example.com/repo.git",
					Path: "manifests",
					Auth: &myv1.GitAuth{Type: myv1.GitAuthBasic, SecretRef: myv1.LocalSecretRef{Name: "creds"}},
				},
			},
			Interval: metav1.Duration{Duration: 5 * time.Minute},
		},
	}
	sec := gitAuthSecret("creds", map[string][]byte{"username": []byte("u"), "password": []byte("tok")})
	r := reconcilerFor(scheme, &fakeFetcher{}, dc, sec)

	var gotAuth *driftsource.GitAuth
	r.GitCloner = func(_ context.Context, dir, _, _ string, auth *driftsource.GitAuth) error {
		gotAuth = auth
		sub := filepath.Join(dir, "manifests")
		if err := os.MkdirAll(sub, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(sub, "cm.yaml"),
			[]byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: from-git\n  namespace: default\n"), 0o644)
	}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dc", Namespace: "default"},
	}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if gotAuth == nil || gotAuth.Basic == nil || gotAuth.Basic.Username != "u" || gotAuth.Basic.Password != "tok" {
		t.Errorf("cloner received auth %+v, want Basic{u tok}", gotAuth)
	}
}
