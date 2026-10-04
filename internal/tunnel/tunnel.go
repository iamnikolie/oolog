// Package tunnel dials OpenObserve through an SSH connection, either to a
// Kubernetes Service (ClusterIP resolved with kubectl on the node) or to a fixed
// host:port reachable from the SSH host, so an instance with no public ingress
// is still reachable from the CLI. The SSH
// connection is opened lazily on the first request and reused for the rest of
// the process.
package tunnel

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/iamnikolie/oolog/internal/config"
)

// Dialer opens the SSH connection on first use and dials the Service through it.
type Dialer struct {
	cfg    config.SSH
	once   sync.Once
	client *ssh.Client
	target string
	err    error
}

// New validates cfg; nothing is dialed until DialContext is called.
func New(cfg config.SSH) (*Dialer, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Dialer{cfg: cfg}, nil
}

func (d *Dialer) what() string {
	if d.cfg.Target != "" {
		return d.cfg.Target
	}
	return d.cfg.Service
}

// DialContext ignores addr: every connection goes to the resolved Service.
func (d *Dialer) DialContext(ctx context.Context, _, _ string) (net.Conn, error) {
	d.once.Do(d.connect)
	if d.err != nil {
		return nil, d.err
	}
	type result struct {
		c   net.Conn
		err error
	}
	ch := make(chan result, 1)
	go func() {
		c, err := d.client.Dial("tcp", d.target)
		ch <- result{c, err}
	}()
	select {
	case r := <-ch:
		if r.err != nil {
			return nil, fmt.Errorf("ssh tunnel: dial %s (%s) via %s: %w", d.what(), d.target, d.cfg.Host, r.err)
		}
		return r.c, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Close releases the SSH connection, if one was opened.
func (d *Dialer) Close() {
	if d.client != nil {
		d.client.Close()
	}
}

func (d *Dialer) connect() {
	client, err := dial(d.cfg)
	if err != nil {
		d.err = err
		return
	}
	if d.cfg.Target != "" {
		d.client = client
		d.target = d.cfg.Target
		return
	}
	ip, err := resolveClusterIP(client, d.cfg)
	if err != nil {
		client.Close()
		d.err = err
		return
	}
	d.client = client
	d.target = net.JoinHostPort(ip, fmt.Sprintf("%d", d.cfg.ServicePort))
}

func dial(c config.SSH) (*ssh.Client, error) {
	auth, closeAgent, err := authMethods(c)
	if err != nil {
		return nil, err
	}
	defer closeAgent()
	khPath := expandHome("~/.ssh/known_hosts")
	if c.KnownHosts != "" {
		khPath = expandHome(c.KnownHosts)
	}
	hostKey, err := knownhosts.New(khPath)
	if err != nil {
		return nil, fmt.Errorf("ssh: load %s (connect once with plain ssh to add %s): %w", khPath, c.Host, err)
	}
	addr := net.JoinHostPort(c.Host, c.Port)
	client, err := ssh.Dial("tcp", addr, &ssh.ClientConfig{
		User:            c.User,
		Auth:            auth,
		HostKeyCallback: hostKey,
		Timeout:         10 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("ssh: dial %s: %w", addr, err)
	}
	return client, nil
}

func resolveClusterIP(client *ssh.Client, c config.SSH) (string, error) {
	ns, name, _ := strings.Cut(c.Service, "/")
	sess, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("ssh: session: %w", err)
	}
	defer sess.Close()
	// ns and name are validated against serviceRe, so they are safe to inline.
	cmd := fmt.Sprintf("KUBECONFIG=%s kubectl -n %s get svc %s -o jsonpath='{.spec.clusterIP}'", shellQuote(c.Kubeconfig), ns, name)
	out, err := sess.CombinedOutput(cmd)
	if err != nil {
		return "", fmt.Errorf("ssh: resolve service %s: %w: %s", c.Service, err, strings.TrimSpace(string(out)))
	}
	ip := strings.TrimSpace(string(out))
	if net.ParseIP(ip) == nil {
		return "", fmt.Errorf("ssh: service %s has no ClusterIP (got %q)", c.Service, ip)
	}
	return ip, nil
}

// authMethods mirrors plain ssh: the configured key first (decrypted with
// $OOLOG_SSH_PASSPHRASE when it is passphrase-protected), then any keys held by
// ssh-agent ($SSH_AUTH_SOCK). A protected key with no passphrase is skipped
// when the agent can stand in for it (after `ssh-add`).
func authMethods(c config.SSH) ([]ssh.AuthMethod, func(), error) {
	var methods []ssh.AuthMethod
	closeAgent := func() {}
	var agentSigners func() ([]ssh.Signer, error)
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		if conn, err := net.Dial("unix", sock); err == nil {
			ag := agent.NewClient(conn)
			agentSigners = ag.Signers
			closeAgent = func() { _ = conn.Close() }
		}
	}
	if c.Key != "" {
		keyPath := expandHome(c.Key)
		keyBytes, err := os.ReadFile(keyPath)
		if err != nil {
			closeAgent()
			return nil, nil, fmt.Errorf("ssh: read key %s: %w", keyPath, err)
		}
		signer, err := ssh.ParsePrivateKey(keyBytes)
		var missing *ssh.PassphraseMissingError
		switch {
		case err == nil:
			methods = append(methods, ssh.PublicKeys(signer))
		case errors.As(err, &missing):
			if pass := os.Getenv("OOLOG_SSH_PASSPHRASE"); pass != "" {
				signer, err = ssh.ParsePrivateKeyWithPassphrase(keyBytes, []byte(pass))
				if err != nil {
					closeAgent()
					return nil, nil, fmt.Errorf("ssh: decrypt key %s with $OOLOG_SSH_PASSPHRASE: %w", keyPath, err)
				}
				methods = append(methods, ssh.PublicKeys(signer))
			} else if agentSigners == nil {
				return nil, nil, fmt.Errorf("ssh: key %s is passphrase-protected: run ssh-agent and `ssh-add %s`, or set $OOLOG_SSH_PASSPHRASE", keyPath, c.Key)
			}
		default:
			closeAgent()
			return nil, nil, fmt.Errorf("ssh: parse key %s: %w", keyPath, err)
		}
	}
	if agentSigners != nil {
		methods = append(methods, ssh.PublicKeysCallback(agentSigners))
	}
	if len(methods) == 0 {
		return nil, nil, errors.New("ssh: no credentials: set ssh.key in the config or run ssh-agent with your key added")
	}
	return methods, closeAgent, nil
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}
