// Package config loads and saves the per-account oolog config.yaml.
package config

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/iamnikolie/oolog/internal/paths"
	"gopkg.in/yaml.v3"
)

// Default field names: the OpenObserve Kubernetes (fluent-bit/otel collector) layout.
const (
	DefaultNamespaceField = "kubernetes_namespace_name"
	DefaultPodField       = "kubernetes_pod_name"
	DefaultContainerField = "kubernetes_container_name"
)

// Config is the credentials + target for one OpenObserve account.
type Config struct {
	URL   string `yaml:"url"`
	Org   string `yaml:"org"`
	Email string `yaml:"email"`
	// Password is the plain-text password/passcode. Prefer the OOLOG_PASSWORD
	// env var or PasswordCommand so the secret need not live in this file.
	Password string `yaml:"password,omitempty"`
	// PasswordCommand is run via `sh -c`; its stdout (trimmed) is the password.
	PasswordCommand string `yaml:"password_command,omitempty"`
	// Stream is the default stream for read commands (--stream overrides).
	Stream string `yaml:"stream,omitempty"`
	// Fields maps the shared filter flags onto this instance's field names.
	Fields Fields `yaml:"fields,omitempty"`
	// SSH, when set, routes every request through an in-process SSH tunnel
	// instead of dialing URL's host directly. URL's host is then only used for
	// the Host header.
	SSH *SSH `yaml:"ssh,omitempty"`
}

// Fields maps logical log attributes to field names in the stream. Empty
// values take the Kubernetes-collector defaults (see Resolved) except Level.
type Fields struct {
	Namespace string `yaml:"namespace,omitempty"`
	Pod       string `yaml:"pod,omitempty"`
	Container string `yaml:"container,omitempty"`
	// Level is the severity field. Unset: --level does a full-text match_all
	// and rendering looks for a "level" field.
	Level string `yaml:"level,omitempty"`
	// Message is tried first when picking the log line to display.
	Message string `yaml:"message,omitempty"`
}

// Resolved returns Fields with defaults filled in for namespace/pod/container.
func (f Fields) Resolved() Fields {
	if f.Namespace == "" {
		f.Namespace = DefaultNamespaceField
	}
	if f.Pod == "" {
		f.Pod = DefaultPodField
	}
	if f.Container == "" {
		f.Container = DefaultContainerField
	}
	return f
}

// SSH describes the SSH host to tunnel through and what to reach from there.
// Exactly one of Service (Kubernetes mode) or Target (plain mode) is set.
type SSH struct {
	Host string `yaml:"host"`
	Port string `yaml:"port,omitempty"` // default 22
	User string `yaml:"user"`
	Key  string `yaml:"key,omitempty"` // private key path; ~ is expanded. Optional with ssh-agent; protected keys need ssh-add or $OOLOG_SSH_PASSPHRASE
	// KnownHosts is the known_hosts file to verify the host key against
	// (default ~/.ssh/known_hosts).
	KnownHosts string `yaml:"known_hosts,omitempty"`

	// Target is "host:port", reachable from the SSH host (e.g. "127.0.0.1:5080"
	// for OpenObserve in docker on that machine).
	Target string `yaml:"target,omitempty"`

	// Service is "<namespace>/<name>"; its ClusterIP is resolved on the node
	// with kubectl on each run, so a recreated Service needs no config change.
	Service     string `yaml:"service,omitempty"`
	ServicePort int    `yaml:"service_port,omitempty"`
	// Kubeconfig is the node-side kubeconfig path for kubectl
	// (default /etc/rancher/k3s/k3s.yaml, which is k3s's default location).
	Kubeconfig string `yaml:"kubeconfig,omitempty"`
}

// DefaultKubeconfig is where k3s writes its kubeconfig.
const DefaultKubeconfig = "/etc/rancher/k3s/k3s.yaml"

var serviceRe = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?/[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

// Validate checks the SSH block and fills defaults (port, kubeconfig).
func (s *SSH) Validate() error {
	if s.Host == "" || s.User == "" {
		return errors.New("ssh: host and user are required (key is optional when ssh-agent holds it)")
	}
	hasSvc, hasTarget := s.Service != "", s.Target != ""
	switch {
	case hasSvc && hasTarget:
		return errors.New("ssh: set exactly one of service or target, not both")
	case !hasSvc && !hasTarget:
		return errors.New("ssh: set exactly one of service (Kubernetes Service) or target (host:port)")
	}
	if hasSvc {
		if !serviceRe.MatchString(s.Service) {
			return fmt.Errorf("ssh: service must be <namespace>/<name>, got %q", s.Service)
		}
		if s.ServicePort <= 0 || s.ServicePort > 65535 {
			return errors.New("ssh: service_port is required with service")
		}
		if s.Kubeconfig == "" {
			s.Kubeconfig = DefaultKubeconfig
		}
	} else {
		host, port, err := net.SplitHostPort(s.Target)
		if err != nil || host == "" {
			return fmt.Errorf("ssh: target must be host:port, got %q", s.Target)
		}
		if n, err := strconv.Atoi(port); err != nil || n <= 0 || n > 65535 {
			return fmt.Errorf("ssh: target port invalid in %q", s.Target)
		}
		if s.ServicePort != 0 || s.Kubeconfig != "" {
			return errors.New("ssh: service_port/kubeconfig only apply with service, not target")
		}
	}
	if s.Port == "" {
		s.Port = "22"
	}
	return nil
}

// Validate checks the config for obvious mistakes.
func (c *Config) Validate() error {
	if c.URL == "" {
		return errors.New("config: url is required")
	}
	if c.SSH != nil {
		return c.SSH.Validate()
	}
	return nil
}

// ResolvePassword fills c.Password from, in order: the OOLOG_PASSWORD env var,
// password_command stdout, the password field. The secret is never printed.
func (c *Config) ResolvePassword() error {
	if v := os.Getenv("OOLOG_PASSWORD"); v != "" {
		c.Password = v
		return nil
	}
	if c.PasswordCommand != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "sh", "-c", c.PasswordCommand)
		cmd.Stderr = os.Stderr
		out, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("password_command failed: %w", err)
		}
		pw := strings.TrimRight(string(out), "\r\n")
		if pw == "" {
			return errors.New("password_command produced empty output")
		}
		c.Password = pw
		return nil
	}
	if c.Password == "" {
		return errors.New("no password: set OOLOG_PASSWORD, password_command, or password in config")
	}
	return nil
}

func file(account string) (string, error) {
	dir, err := paths.Home(account)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// Load reads config.yaml for the account.
func Load(account string) (Config, error) {
	p, err := file(account)
	if err != nil {
		return Config{}, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return Config{}, fmt.Errorf("no config (%s); run 'oolog config init': %w", p, err)
	}
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return Config{}, err
	}
	return c, nil
}

// Save writes config.yaml (mode 0600).
func Save(c Config, account string) error {
	p, err := file(account)
	if err != nil {
		return err
	}
	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o600)
}
