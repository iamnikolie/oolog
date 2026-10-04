package tunnel

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"golang.org/x/crypto/ssh"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iamnikolie/oolog/internal/config"
)

func TestNewAppliesDefaultsAndDialsNothing(t *testing.T) {
	d, err := New(config.SSH{Host: "h", User: "u", Key: "k", Service: "ns/svc", ServicePort: 5080})
	if err != nil {
		t.Fatal(err)
	}
	if d.cfg.Port != "22" || d.cfg.Kubeconfig != config.DefaultKubeconfig {
		t.Fatalf("defaults not applied: %+v", d.cfg)
	}
	if _, err := New(config.SSH{Host: "h", User: "u", Key: "k"}); err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("want exactly-one error, got %v", err)
	}
}

func TestExpandHome(t *testing.T) {
	if got := expandHome("/abs/key"); got != "/abs/key" {
		t.Fatalf("absolute path changed: %s", got)
	}
	if got := expandHome("~/.ssh/id_rsa"); strings.HasPrefix(got, "~") {
		t.Fatalf("~ not expanded: %s", got)
	}
}

func writeKey(t *testing.T, passphrase string) string {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var block *pem.Block
	if passphrase == "" {
		block, err = ssh.MarshalPrivateKey(priv, "")
	} else {
		block, err = ssh.MarshalPrivateKeyWithPassphrase(priv, "", []byte(passphrase))
	}
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "id")
	if err := os.WriteFile(p, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAuthMethods(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("OOLOG_SSH_PASSPHRASE", "")
	wantErr := func(err error, sub string) {
		t.Helper()
		if err == nil || !strings.Contains(err.Error(), sub) {
			t.Fatalf("want error containing %q, got %v", sub, err)
		}
	}
	ok := func(m []ssh.AuthMethod, done func(), err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		done()
		if len(m) != 1 {
			t.Fatalf("want 1 auth method, got %d", len(m))
		}
	}

	ok(authMethods(config.SSH{Key: writeKey(t, "")}))

	protected := writeKey(t, "s3cret")
	_, _, err := authMethods(config.SSH{Key: protected})
	wantErr(err, "passphrase-protected")

	t.Setenv("OOLOG_SSH_PASSPHRASE", "wrong")
	_, _, err = authMethods(config.SSH{Key: protected})
	wantErr(err, "decrypt key")

	t.Setenv("OOLOG_SSH_PASSPHRASE", "s3cret")
	ok(authMethods(config.SSH{Key: protected}))

	_, _, err = authMethods(config.SSH{})
	wantErr(err, "no credentials")
}
