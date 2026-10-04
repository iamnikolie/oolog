package config

import (
	"strings"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Setenv("OOLOG_HOME", t.TempDir())

	in := Config{
		URL:      "https://logs.example.com",
		Org:      "default",
		Email:    "agent@example.com",
		Password: "secret",
	}
	if err := Save(in, ""); err != nil {
		t.Fatal(err)
	}
	got, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if got != in {
		t.Fatalf("round trip mismatch: %+v != %+v", got, in)
	}
}

func TestLoadMissing(t *testing.T) {
	t.Setenv("OOLOG_HOME", t.TempDir())
	if _, err := Load(""); err == nil {
		t.Fatal("expected error loading missing config")
	}
}

func TestSSHValidate(t *testing.T) {
	base := SSH{Host: "203.0.113.10", User: "u", Key: "~/.ssh/id"}
	cases := []struct {
		name    string
		mutate  func(*SSH)
		wantErr string
	}{
		{"service ok", func(s *SSH) { s.Service = "openobserve/openobserve"; s.ServicePort = 5080 }, ""},
		{"target ok", func(s *SSH) { s.Target = "127.0.0.1:5080" }, ""},
		{"target ipv6 ok", func(s *SSH) { s.Target = "[::1]:5080" }, ""},
		{"neither", func(s *SSH) {}, "exactly one"},
		{"both", func(s *SSH) { s.Service = "a/b"; s.ServicePort = 1; s.Target = "h:1" }, "not both"},
		{"missing host", func(s *SSH) { s.Host = ""; s.Target = "h:1" }, "required"},
		{"missing user", func(s *SSH) { s.User = ""; s.Target = "h:1" }, "required"},
		{"bad service", func(s *SSH) { s.Service = "openobserve"; s.ServicePort = 1 }, "<namespace>/<name>"},
		{"shell in service", func(s *SSH) { s.Service = "a/b;rm -rf /"; s.ServicePort = 1 }, "<namespace>/<name>"},
		{"service no port", func(s *SSH) { s.Service = "a/b" }, "service_port"},
		{"target no port", func(s *SSH) { s.Target = "127.0.0.1" }, "host:port"},
		{"target bad port", func(s *SSH) { s.Target = "h:abc" }, "port invalid"},
		{"target with service_port", func(s *SSH) { s.Target = "h:1"; s.ServicePort = 5 }, "only apply with service"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := base
			tc.mutate(&s)
			err := s.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if s.Port != "22" {
					t.Fatalf("port default not applied")
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestServiceDefaultsKubeconfig(t *testing.T) {
	s := SSH{Host: "h", User: "u", Key: "k", Service: "a/b", ServicePort: 1}
	if err := s.Validate(); err != nil || s.Kubeconfig != DefaultKubeconfig {
		t.Fatalf("kubeconfig default: %v %q", err, s.Kubeconfig)
	}
}

func TestFieldsResolved(t *testing.T) {
	f := Fields{Pod: "host"}.Resolved()
	if f.Namespace != DefaultNamespaceField || f.Pod != "host" || f.Container != DefaultContainerField || f.Level != "" {
		t.Fatalf("bad resolve: %+v", f)
	}
}

func TestResolvePassword(t *testing.T) {
	t.Setenv("OOLOG_PASSWORD", "")
	c := Config{Password: "plain"}
	if err := c.ResolvePassword(); err != nil || c.Password != "plain" {
		t.Fatalf("plain: %v %q", err, c.Password)
	}
	c = Config{Password: "plain", PasswordCommand: "printf 'from-cmd\\n'"}
	if err := c.ResolvePassword(); err != nil || c.Password != "from-cmd" {
		t.Fatalf("command: %v %q", err, c.Password)
	}
	t.Setenv("OOLOG_PASSWORD", "from-env")
	if err := c.ResolvePassword(); err != nil || c.Password != "from-env" {
		t.Fatalf("env: %v %q", err, c.Password)
	}
	t.Setenv("OOLOG_PASSWORD", "")
	c = Config{PasswordCommand: "exit 3"}
	if err := c.ResolvePassword(); err == nil {
		t.Fatal("want error for failing command")
	}
	c = Config{}
	if err := c.ResolvePassword(); err == nil {
		t.Fatal("want error for no password")
	}
}

func TestLoadParsesStreamAndFields(t *testing.T) {
	t.Setenv("OOLOG_HOME", t.TempDir())
	in := Config{URL: "http://x", Stream: "app", Fields: Fields{Namespace: "service", Level: "lvl", Message: "msg"}}
	if err := Save(in, ""); err != nil {
		t.Fatal(err)
	}
	got, err := Load("")
	if err != nil || got != in {
		t.Fatalf("got %+v err %v", got, err)
	}
}
