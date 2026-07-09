package source

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	xssh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// testSSHKeyPEM returns a freshly generated, unencrypted PEM-encoded ed25519
// private key suitable for gitssh.NewPublicKeys.
func testSSHKeyPEM(t *testing.T) []byte {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ssh key: %v", err)
	}
	blk, err := xssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatalf("marshal ssh key: %v", err)
	}
	return pem.EncodeToMemory(blk)
}

// testKnownHosts returns a single valid known_hosts line for github.com backed
// by a freshly generated host key.
func testKnownHosts(t *testing.T) []byte {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate host key: %v", err)
	}
	sshPub, err := xssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("wrap host key: %v", err)
	}
	return []byte(knownhosts.Line([]string{"github.com"}, sshPub) + "\n")
}

func TestAuthMethod(t *testing.T) {
	// nil auth clones anonymously → nil AuthMethod, no error.
	if m, err := authMethod(nil); err != nil || m != nil {
		t.Fatalf("authMethod(nil) = %v, %v; want nil, nil", m, err)
	}

	// An auth with no scheme populated is also anonymous.
	if m, err := authMethod(&GitAuth{}); err != nil || m != nil {
		t.Fatalf("authMethod(empty) = %v, %v; want nil, nil", m, err)
	}

	// Basic → HTTPS BasicAuth carrying the username/password verbatim.
	m, err := authMethod(&GitAuth{Basic: &BasicAuth{Username: "u", Password: "tok"}})
	if err != nil {
		t.Fatalf("basic: %v", err)
	}
	ba, ok := m.(*githttp.BasicAuth)
	if !ok {
		t.Fatalf("basic type = %T, want *githttp.BasicAuth", m)
	}
	if ba.Username != "u" || ba.Password != "tok" {
		t.Errorf("basic creds = %+v, want {u tok}", ba)
	}

	// Bearer → HTTPS TokenAuth.
	m, err = authMethod(&GitAuth{Bearer: "tok"})
	if err != nil {
		t.Fatalf("bearer: %v", err)
	}
	ta, ok := m.(*githttp.TokenAuth)
	if !ok {
		t.Fatalf("bearer type = %T, want *githttp.TokenAuth", m)
	}
	if ta.Token != "tok" {
		t.Errorf("bearer token = %q, want tok", ta.Token)
	}

	// SSH with a valid key + known_hosts → PublicKeys with the default "git"
	// user and a host-key callback wired.
	m, err = authMethod(&GitAuth{SSH: &SSHAuth{PrivateKey: testSSHKeyPEM(t), KnownHosts: testKnownHosts(t)}})
	if err != nil {
		t.Fatalf("ssh: %v", err)
	}
	pk, ok := m.(*gitssh.PublicKeys)
	if !ok {
		t.Fatalf("ssh type = %T, want *gitssh.PublicKeys", m)
	}
	if pk.User != "git" {
		t.Errorf("ssh user = %q, want git", pk.User)
	}
	if pk.HostKeyCallback == nil {
		t.Error("ssh HostKeyCallback not set — host-key verification would be skipped")
	}
}

func TestAuthMethod_SSHUserOverride(t *testing.T) {
	m, err := authMethod(&GitAuth{SSH: &SSHAuth{User: "deploy", PrivateKey: testSSHKeyPEM(t), KnownHosts: testKnownHosts(t)}})
	if err != nil {
		t.Fatalf("ssh: %v", err)
	}
	if pk := m.(*gitssh.PublicKeys); pk.User != "deploy" {
		t.Errorf("ssh user = %q, want deploy", pk.User)
	}
}

func TestAuthMethod_SSHMissingKnownHosts(t *testing.T) {
	// Fail-closed: SSH without known_hosts is an error, not an insecure clone.
	if _, err := authMethod(&GitAuth{SSH: &SSHAuth{PrivateKey: testSSHKeyPEM(t)}}); err == nil {
		t.Fatal("expected error for SSH without known_hosts, got nil")
	}
}

func TestKnownHostsCallback_BadParse(t *testing.T) {
	// Non-empty but malformed known_hosts content fails to parse (it is not an
	// empty-input case, which is handled separately by the fail-closed guard).
	if _, err := knownHostsCallback([]byte("this is not a valid known_hosts entry\n")); err == nil {
		t.Fatal("expected parse error for malformed known_hosts, got nil")
	}
}

func TestAuthMethod_SSHBadKey(t *testing.T) {
	if _, err := authMethod(&GitAuth{SSH: &SSHAuth{PrivateKey: []byte("not a private key"), KnownHosts: testKnownHosts(t)}}); err == nil {
		t.Fatal("expected error for an unparseable SSH key, got nil")
	}
}

// TestGitClone_AuthMethodError drives the real gitClone (nil cloner) with SSH
// auth that fails authMethod (fail-closed: no known_hosts). authMethod errors
// before any network access, exercising gitClone's auth-error path and
// withCheckout's clone-error wrapping offline.
func TestGitClone_AuthMethodError(t *testing.T) {
	auth := &GitAuth{SSH: &SSHAuth{PrivateKey: testSSHKeyPEM(t)}} // no KnownHosts → fail-closed
	src := NewGitSource(context.Background(), "https://example.com/repo.git", "", "", auth, nil)
	if _, err := src.Load(); err == nil {
		t.Fatal("expected an auth error before clone, got nil")
	}
}

// TestGitSource_PassesAuthToCloner verifies the resolved auth threads through
// Load → withCheckout → the injected cloner unchanged.
func TestGitSource_PassesAuthToCloner(t *testing.T) {
	want := &GitAuth{Basic: &BasicAuth{Username: "u", Password: "p"}}
	var got *GitAuth
	clone := func(_ context.Context, dir, _, _ string, auth *GitAuth) error {
		got = auth
		return os.WriteFile(filepath.Join(dir, "x.yaml"),
			[]byte("apiVersion: v1\nkind: X\nmetadata:\n  name: x\n"), 0o644)
	}
	src := NewGitSource(context.Background(), "u", "", "", want, clone)
	if _, err := src.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got != want {
		t.Errorf("cloner received auth %p, want %p", got, want)
	}
}
